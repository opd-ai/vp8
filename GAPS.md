# Implementation Gaps — 2026-03-23

This document identifies gaps between the project's stated goals and its current implementation, based on the README, ROADMAP.md, and code analysis.

---

## Gap 1: Residual Coding Pipeline Not Integrated

- **Stated Goal**: ROADMAP.md Milestone 1 claims completion of:
  - §1.4 Forward DCT and quantization ✅
  - §1.5 Residual token entropy coding ✅  
  - §1.6 Entropy probability update ✅
  - §1.7 Multiple DCT partitions ✅

- **Current State**: These components exist as standalone, tested modules (`dct.go`, `token.go`, `partition.go`) but are not wired into `Encoder.Encode()`. The encoder path in `encoder.go:91-100` creates macroblocks via `processMacroblock()` which always returns:
  ```go
  return macroblock{
      lumaMode:   DC_PRED,
      chromaMode: DC_PRED_CHROMA,
      skip:       true,  // Always skipped
      dcValue:    0,     // Always zero
  }
  ```

- **Impact**: 
  - Encoded frames contain no actual image data—just prediction mode signals
  - All encoded frames are visually identical regardless of input
  - WebRTC receivers see a solid color (typically gray/purple) rather than actual video
  - Quality is at minimum possible (PSNR approaches 0 dB for most content)

- **Closing the Gap**: Implement a proper encoding loop in `Encode()`:
  1. Parse input YUV into per-macroblock blocks
  2. For each macroblock:
     - Select best prediction mode via `SelectBest16x16Mode()`
     - Compute residual: `ComputeResidual(src, prediction)`
     - Transform: `ForwardDCT4x4()` for each 4x4 block
     - Quantize: `QuantizeBlock()` with factors from `GetQuantFactors()`
     - Determine skip: `BlockHasNonZeroCoeffs()` — if all zero, skip=true
     - If not skipped, encode tokens via `TokenEncoder.EncodeBlock()`
  3. Assemble frame using existing `BuildKeyFrame()` with populated residual partition
  
  Estimated effort: 100-200 lines of integration code.

---

## Gap 2: Prediction Mode Selection Not Used

- **Stated Goal**: ROADMAP.md §1.2 claims:
  > "Implement V_PRED, H_PRED, TM_PRED... Pick the mode that minimises sum-of-absolute-differences (SAD)"
  
  And marks this ✅ implemented.

- **Current State**: `SelectBest16x16Mode()`, `SelectBest8x8ChromaMode()`, and `SelectBest4x4Mode()` are fully implemented and tested in `prediction.go` and `bpred.go`. However, `processMacroblock()` in `macroblock.go` hardcodes `DC_PRED` without calling these functions.

- **Impact**:
  - Suboptimal compression even with residuals (wrong mode selection wastes bits)
  - The extensive prediction infrastructure (400+ lines) provides no value to users

- **Closing the Gap**: 
  1. Modify `processMacroblock()` to accept the source block and neighbor pixels
  2. Call `SelectBest16x16Mode()` for luma and `SelectBest8x8ChromaMode()` for chroma
  3. Store selected modes in the macroblock struct
  4. Pass modes to `encodeFrameHeader()` which already supports encoding different modes

---

## Gap 3: B_PRED (4×4 Subblock) Mode Never Used

- **Stated Goal**: ROADMAP.md §1.3 claims:
  > "Implement all ten 4×4 luma sub-modes: B_DC_PRED, B_TM_PRED, B_VE_PRED..."
  
  Marked ✅ implemented.

- **Current State**: All 10 B_PRED sub-modes are implemented in `bpred.go` with `Predict4x4()` and `SelectBest4x4Mode()`. However:
  - `encodeFrameHeader()` in `bitstream.go:73` always encodes `y_mode != B_PRED`
  - No code path exists to select or signal B_PRED mode

- **Impact**:
  - Compression efficiency reduced for content with local detail variations
  - ~200 lines of B_PRED code provides no user value

- **Closing the Gap**:
  1. Add B_PRED mode decision: Compare SAD of best 16x16 mode vs sum of best 4x4 modes
  2. When B_PRED wins, encode all 16 sub-block modes via `bmode` probability tree
  3. Update `encodeFrameHeader()` to conditionally emit B_PRED signal

---

## Gap 4: Multiple Partitions Never Used

- **Stated Goal**: ROADMAP.md §1.7 claims:
  > "Support encoding residuals into 1, 2, 4, or 8 independent second-partition segments"
  
  Marked ✅ implemented.

- **Current State**: `partition.go` provides complete partition management infrastructure (`PartitionWriter`, `AssembleMultiPartitionFrame()`), but `bitstream.go:39` always encodes `nb_dct_partitions: 0 → 1 partition`.

- **Impact**:
  - Multi-core encoding parallelism unavailable
  - No error resilience benefit from partition separation

- **Closing the Gap**:
  1. Add `PartitionCount` to `Encoder` struct
  2. Use `PartitionWriter` in encoding loop
  3. Modify frame assembly to use `AssembleMultiPartitionFrame()`
  4. Encode correct partition count in header

---

## Gap 5: Quantizer Deltas Not Configurable

- **Stated Goal**: ROADMAP.md §1.1 claims:
  > "Expose per-plane quantizer deltas (y_dc_delta, y2_dc_delta, ...)"
  
  Marked ✅ implemented.

- **Current State**: `quant.go` provides `GetQuantFactors()` accepting all delta parameters. However:
  - `Encoder` struct has no delta fields
  - `bitstream.go:44-54` always writes deltas as 0 (delta_present=false)
  - No API exposes delta configuration

- **Impact**:
  - Cannot tune quality trade-offs between luma DC, luma AC, and chroma
  - Professional video encoding scenarios limited

- **Closing the Gap**:
  1. Add delta fields to `Encoder`: `y1DCDelta`, `y2DCDelta`, etc.
  2. Add setter method: `SetQuantizerDeltas(y1dc, y2dc, y2ac, uvdc, uvac int)`
  3. Update `encodeFrameHeader()` to conditionally encode deltas when non-zero

---

## Gap 6: Token Probability Updates Not Used

- **Stated Goal**: ROADMAP.md §1.6 claims:
  > "Implement serialisation of coefficient probability table updates"
  
  Marked ✅ implemented.

- **Current State**: `token.go` provides `EncodeCoeffProbUpdates()` and `EncodeNoCoeffProbUpdates()`. However:
  - No code path calls these functions
  - Encoded frames don't include probability update section
  - `refresh_entropy_probs` is set but coeff prob updates are never written

- **Impact**:
  - Cannot adapt to content characteristics for better compression
  - Always uses default probability tables

- **Closing the Gap**:
  1. Track coefficient statistics during encoding
  2. After encoding, compute updated probabilities
  3. Call `EncodeCoeffProbUpdates()` in frame header assembly

---

## Gap 7: WebRTC Interoperability Unverified

- **Stated Goal**: README claims:
  > "Compatible with WebRTC stacks (pion/rtp VP8Payloader, ivfwriter)"

- **Current State**: The encoder produces valid VP8 keyframe bitstreams per tests. However:
  - No integration test with pion/rtp VP8Payloader
  - No test decoding with golang.org/x/image/vp8 or libvpx
  - No IVF container writing test

- **Impact**:
  - Compatibility is asserted but not verified
  - Users may encounter interop issues in production

- **Closing the Gap**:
  1. Add integration test: encode frame → VP8Payloader.Payload() → verify RTP packets
  2. Add decode test: encode frame → golang.org/x/image/vp8.Decode() → verify image
  3. Add IVF test: write IVF file → verify with ffprobe

---

## Priority Matrix

| Gap | Severity | Effort | Priority |
|-----|----------|--------|----------|
| Gap 1: Residual coding not integrated | HIGH | MEDIUM | **P1** |
| Gap 2: Prediction mode selection | MEDIUM | LOW | **P2** |
| Gap 3: B_PRED mode | LOW | MEDIUM | P3 |
| Gap 4: Multiple partitions | LOW | LOW | P4 |
| Gap 5: Quantizer deltas | LOW | LOW | P4 |
| Gap 6: Probability updates | LOW | MEDIUM | P5 |
| Gap 7: WebRTC interop tests | MEDIUM | LOW | **P2** |

**Recommended order**: Gap 1 → Gap 7 → Gap 2 → Gaps 3-6

Closing Gap 1 alone would transform this from a "minimal skeleton" to a "functional I-frame encoder" that produces real, visible output.
