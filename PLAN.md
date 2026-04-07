# Implementation Plan: RFC 6386 Compliance & Bitstream Correctness

## Project Context
- **What it does**: A pure-Go VP8 encoder with no CGo dependencies that produces valid VP8 bitstreams for WebRTC applications
- **Current goal**: Fix critical RFC 6386 compliance issues (MV encoding, loop filter, chroma prediction) that prevent correct decode
- **Estimated Scope**: Medium (8 high-complexity functions on critical paths, 1.93% duplication)

## Goal-Achievement Status
| Stated Goal | Current Status | This Plan Addresses |
|-------------|---------------|---------------------|
| Pure Go — no C libraries, no CGo | ✅ Achieved | No |
| Produces valid VP8 bitstreams (RFC 6386) | ⚠️ Partial | **Yes** |
| Compatible with WebRTC stacks | ⚠️ Partial | **Yes** |
| Inter-frame (P-frame) encoding | ⚠️ Partial | **Yes** |
| Loop filter for reference frame quality | ⚠️ Partial | **Yes** |
| Reference frame management (golden/altref) | ⚠️ Partial | No (future) |
| Diamond search motion estimation | ✅ Achieved | No |
| Configurable key frame interval | ✅ Achieved | No |
| Multi-partition support | ✅ Achieved | No |

## Industry Context
This is the **only pure-Go VP8 encoder** in the ecosystem. All alternatives (pion/mediadevices, libvpx bindings) require CGo. Fixing RFC compliance issues significantly increases the value proposition for Go developers needing CGo-free WebRTC video.

## Metrics Summary
- High-complexity functions on goal-critical paths: **8** functions above threshold 9.0
- Duplication ratio: **1.93%** (excellent, below 3% target)
- Doc coverage: **99.3%** (excellent)
- Package coupling: Single package, well-factored (211 functions across 16 files)

### High-Complexity Functions on Critical Paths
| Function | File | Complexity | Goal Impact |
|----------|------|------------|-------------|
| encodeMVComponent | interbitstream.go | 10.1 | MV encoding (Critical) |
| encodeFrameHeaderWithProbs | bitstream.go | 10.6 | Loop filter header |
| encodeLumaBlocks | bitstream.go | 10.6 | Residual encoding |
| reconstructLumaBPred | refframe.go | 10.3 | B_PRED reconstruction |
| build4x4AboveFromMBContext | macroblock.go | 10.1 | Prediction context |
| EncodeCoeffProbUpdates | token.go | 10.3 | Probability updates |

---

## Implementation Steps

### Step 1: Fix Motion Vector Short Encoding (Critical)
- **Deliverable**: Rewrite `encodeMVComponent` in `interbitstream.go:55-70` to use RFC 6386 §17.1 short MV tree structure
- **Dependencies**: None
- **Goal Impact**: Fixes critical inter-frame decode failures — enables correct P-frame encoding
- **Files**: `interbitstream.go`
- **Acceptance**: Inter frames with non-zero MVs decode correctly in reference decoder (ffprobe/ffmpeg)
- **Validation**: 
  ```bash
  go test -v -run TestInterFrame ./... && ffprobe -v error -show_frames test_inter.ivf 2>&1 | grep -c "pict_type=P"
  ```

### Step 2: Fix Motion Vector Long Encoding (Critical)
- **Deliverable**: Rewrite long MV encoding in `interbitstream.go:71-82` to match RFC 6386 §17.1 (high bits via probs[9-15], low 3 bits via short tree)
- **Dependencies**: Step 1 (shares encoding logic)
- **Goal Impact**: Enables correct encoding of large motion vectors (>7 quarter-pixels)
- **Files**: `interbitstream.go`
- **Acceptance**: Inter frames with large MVs (e.g., fast motion) decode correctly
- **Validation**:
  ```bash
  go test -v -run TestLargeMV ./...
  ```

### Step 3: Implement Per-Quadrant Chroma DC Prediction
- **Deliverable**: Modify `predict8x8DC` in `prediction.go:217-239` to compute four separate DC values per RFC 6386 §12.2
- **Dependencies**: None
- **Goal Impact**: Fixes chroma prediction mismatch between encoder and decoder
- **Files**: `prediction.go`
- **Acceptance**: Chroma DC prediction matches `golang.org/x/image/vp8` decoder behavior
- **Validation**:
  ```bash
  go test -v -run TestChromaDC ./...
  ```

### Step 4: Fix Simple Loop Filter Formula
- **Deliverable**: Rewrite `computeSimpleFilter` in `loopfilter.go:86-103` to match RFC 6386 §15.2 formula
- **Dependencies**: None
- **Goal Impact**: Enables loop filter feature advertised in API (`SetLoopFilterLevel`)
- **Files**: `loopfilter.go`
- **Acceptance**: Loop-filtered frames decode with expected PSNR improvement
- **Validation**:
  ```bash
  go test -v -run TestLoopFilter ./...
  ```

### Step 5: Differentiate Macroblock vs Sub-Block Edge Filtering
- **Deliverable**: Modify `filterPlane` in `loopfilter.go:67-81` to apply stronger filtering at macroblock edges (every 16 luma / 8 chroma pixels)
- **Dependencies**: Step 4
- **Goal Impact**: Correct loop filter behavior per RFC 6386 §15
- **Files**: `loopfilter.go`
- **Acceptance**: Visual inspection shows stronger filtering at MB boundaries
- **Validation**:
  ```bash
  go test -v -run TestLoopFilterEdges ./...
  ```

### Step 6: Fix leftBModes Pass-By-Value Bug
- **Deliverable**: Change `leftBModes` parameter in `updateBPredContext` (`interbitstream.go:252`) from `[4]intraBMode` to `*[4]intraBMode`
- **Dependencies**: None
- **Goal Impact**: Enables correct B_PRED encoding across adjacent macroblocks in inter frames
- **Files**: `interbitstream.go`
- **Acceptance**: Adjacent B_PRED macroblocks encode/decode correctly
- **Validation**:
  ```bash
  go test -v -run TestBPredContext ./...
  ```

### Step 7: Reset coeffProbs on Key Frame
- **Deliverable**: Add `e.coeffProbs = DefaultCoeffProbs` at `encoder.go:328-333` when encoding key frames with probability updates enabled
- **Dependencies**: None
- **Goal Impact**: Fixes probability state corruption across GOPs
- **Files**: `encoder.go`
- **Acceptance**: Multi-GOP sequences with probability updates decode correctly
- **Validation**:
  ```bash
  go test -v -run TestProbReset ./...
  ```

### Step 8: Add First Partition Size Overflow Check
- **Deliverable**: Add validation in `buildFrameTag` (`bitstream.go:754-760`) to return error if first partition exceeds 19-bit limit (524,287 bytes)
- **Dependencies**: None
- **Goal Impact**: Prevents silent bitstream corruption for very large frames
- **Files**: `bitstream.go`
- **Acceptance**: Encoding returns error for oversized first partitions
- **Validation**:
  ```bash
  go test -v -run TestPartitionOverflow ./...
  ```

### Step 9: Remove Dead Code
- **Deliverable**: Delete unused functions `encodeInterMBMode` (`interbitstream.go:113-167`), `encodeYMode` (`bitstream.go:237-280`), and `macroblock.go.bak`
- **Dependencies**: Steps 1-8 (verify functions truly unused after fixes)
- **Goal Impact**: Reduces code surface and maintenance burden
- **Files**: `interbitstream.go`, `bitstream.go`, `macroblock.go.bak`
- **Acceptance**: Build succeeds, tests pass, `go vet` clean
- **Validation**:
  ```bash
  go build ./... && go vet ./... && go test ./...
  ```

### Step 10: Fix predict4x4HE Top-Left Pixel
- **Deliverable**: Modify `predict4x4HE` in `bpred.go:284-316` to use actual top-left pixel P from above array instead of hardcoded 129
- **Dependencies**: None
- **Goal Impact**: Improves B_PRED accuracy for HE sub-mode
- **Files**: `bpred.go`
- **Acceptance**: B_PRED HE mode produces correct predictions
- **Validation**:
  ```bash
  go test -v -run TestBPredHE ./...
  ```

### Step 11: Add B_PRED Evaluation to Inter Frame Intra Fallback
- **Deliverable**: Modify `processInterMacroblock` in `inter.go:41-87` to evaluate B_PRED when intra mode wins over inter
- **Dependencies**: Steps 6, 10
- **Goal Impact**: Enables B_PRED mode selection in inter frames for improved compression
- **Files**: `inter.go`
- **Acceptance**: B_PRED macroblocks appear in inter frames when beneficial
- **Validation**:
  ```bash
  go test -v -run TestInterBPred ./...
  ```

### Step 12: Improve Chroma Mode Selection
- **Deliverable**: Modify `processMacroblock` in `macroblock.go:95` to evaluate both U and V planes for chroma mode selection
- **Dependencies**: None
- **Goal Impact**: Optimizes chroma mode decision for better quality
- **Files**: `macroblock.go`
- **Acceptance**: PSNR improvement on UV planes for test sequences
- **Validation**:
  ```bash
  go test -v -run TestChromaMode ./... -bench=.
  ```

---

## Validation Commands

```bash
# Full test suite with race detector
go test -race ./...

# Run all RFC compliance tests
go test -v -run "Test(MV|Chroma|Loop|BPred)" ./...

# Generate updated metrics
go-stats-generator analyze . --skip-tests | grep -E "(Complexity|Duplication|Doc)"

# Verify high complexity reduction
go-stats-generator analyze . --skip-tests --format json --sections functions | \
  jq '[.functions[] | select(.complexity.overall > 9.0)] | length'
# Target: ≤6 (down from 8)

# Static analysis
go vet ./...
```

## Out of Scope (Explicit Design Decisions)
| Item | Reason |
|------|--------|
| Sub-pixel motion estimation | README §Limitations |
| Segmentation | README §Limitations |
| Temporal scalability | README §Limitations |
| Normal loop filter | README §Limitations |
| VP8 decoder | External dependency (`golang.org/x/image/vp8`) |
| Rate control | Future enhancement |

---

*Generated: 2026-04-07 from go-stats-generator metrics + AUDIT.md + ROADMAP.md*
