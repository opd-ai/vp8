# Implementation Plan: Complete Feature Enablement & Code Quality

## Project Context
- **What it does**: A pure-Go VP8 encoder with no CGo dependencies, producing valid VP8 bitstreams compatible with WebRTC stacks.
- **Current goal**: Enable fully functional loop filter (API exists but disabled) and resolve code duplication to improve maintainability.
- **Estimated Scope**: Medium (14 functions above complexity threshold, 6.1% duplication ratio)

## Goal-Achievement Status
| Stated Goal | Current Status | This Plan Addresses |
|-------------|---------------|---------------------|
| Pure Go — no CGo | ✅ Achieved | No |
| Valid VP8 key-frame bitstreams | ✅ Achieved | No |
| Valid VP8 inter-frame bitstreams | ⚠️ Partial (untestable locally) | Yes (external validation) |
| Loop filter for reference frame quality | ⚠️ API exists, disabled | Yes |
| B_PRED mode for compression efficiency | ⚠️ Implemented, disabled | No (blocked by bitstream bug) |
| Token probability updates | ⚠️ Implemented, unused | No (optimization, lower priority) |
| Multi-partition support | ✅ Achieved | No |
| Per-plane quantizer deltas | ✅ Achieved | No |

## Metrics Summary
- Complexity hotspots on goal-critical paths: **14 functions** above cyclomatic complexity 9
- Duplication ratio: **6.1%** (357 duplicated lines, largest clone 61 lines)
- Doc coverage: **98.4%** (excellent)
- Test status: All 23 tests pass with race detector
- Dependency: `golang.org/x/image v0.37.0` has CVE-2026-33809 in TIFF package (not used by this project)

### High-Complexity Functions Requiring Attention
| Function | File | Complexity | Impact |
|----------|------|------------|--------|
| encodeResidualPartition | bitstream.go:651 | 24 | Core encoder loop |
| encodeResidualMultiPartition | bitstream.go:496 | 24 | Duplicates above |
| build4x4Context | macroblock.go:238 | 23 | Neighbor extraction |
| findNearestMV | motion.go:311 | 21 | MV prediction |
| buildMBContext | encoder.go:356 | 15 | Context building |

### Critical Duplication Violations (>20 lines)
| Clone Size | Location 1 | Location 2 | Severity |
|------------|-----------|------------|----------|
| 61 lines | bitstream.go:585-645 | bitstream.go:757-819 | Violation |
| 54 lines | token.go:369-422 | token.go:512-563 | Violation |
| 32 lines | bitstream.go:62-93 | interbitstream.go:176-202 | Violation |
| 30 lines | bitstream.go:552-581 | bitstream.go:721-752 | Violation |
| 21 lines | loopfilter.go:93-113 | loopfilter.go:131-147 | Violation |

---

## Implementation Steps

### Step 1: Enable Loop Filter in Bitstream Header
- **Deliverable**: Modify `bitstream.go` and `interbitstream.go` to encode configured loop filter level instead of hardcoded 0
- **Dependencies**: None
- **Goal Impact**: Enables the documented loop filter feature, improving inter-frame reference quality
- **Acceptance**: 
  - `SetLoopFilterLevel(30)` results in `loop_filter_level=30` in encoded frame header
  - All existing tests continue to pass
  - New test verifies loop filter level appears in bitstream
- **Validation**: 
  ```bash
  go test -v -run TestLoopFilter ./...
  ```

### Step 2: Uncomment Loop Filter Application in Encoder
- **Deliverable**: Enable `applyLoopFilter(&recon, e.loopFilter)` call in `encoder.go:309` for inter-frame encoding
- **Dependencies**: Step 1 (header must match applied filter)
- **Goal Impact**: Completes loop filter feature; reduces blocking artifacts in reference frames
- **Acceptance**:
  - Inter-frame sequences with loop filter enabled decode correctly
  - Visual quality improvement measurable via PSNR on test sequences
- **Validation**: 
  ```bash
  go test -race ./... && go test -v -run TestInterFrame ./...
  ```

### Step 3: Extract Shared Frame Header Encoding
- **Deliverable**: Create shared helper function for common frame header encoding logic duplicated between `bitstream.go:62-93` and `interbitstream.go:176-202`
- **Dependencies**: None
- **Goal Impact**: Reduces duplication ratio, improves maintainability
- **Acceptance**: 
  - Duplication ratio decreases by ~1%
  - No behavioral change in encoded bitstreams
- **Validation**: 
  ```bash
  go test -race ./... && go-stats-generator analyze . --skip-tests --format json --sections duplication | python3 -c "import sys,json; d=json.loads(sys.stdin.read().split('100.0%)')[-1]); print(f'Duplication: {d[\"duplication\"][\"duplication_ratio\"]*100:.1f}%')"
  ```

### Step 4: Extract Shared Residual Encoding Logic
- **Deliverable**: Refactor `encodeResidualPartition` (bitstream.go:651) and `encodeResidualMultiPartition` (bitstream.go:496) to share the 61-line duplicated coefficient encoding block (lines 585-645 and 757-819)
- **Dependencies**: None
- **Goal Impact**: Eliminates largest duplication clone, reduces complexity
- **Acceptance**:
  - Single shared function replaces duplicate blocks
  - Cyclomatic complexity of both functions decreases
  - All tests pass
- **Validation**: 
  ```bash
  go test -race ./... && go-stats-generator analyze . --skip-tests --format json --sections functions | python3 -c "import sys,json; d=json.loads(sys.stdin.read().split('100.0%)')[-1]); f=[x for x in d['functions'] if 'Residual' in x['name']]; print('\\n'.join([f\"{x['name']}: {x['complexity']['cyclomatic']}\" for x in f]))"
  ```

### Step 5: Extract Shared Token Coefficient Loop
- **Deliverable**: Refactor the 54-line duplicate in `token.go` (lines 369-422 and 512-563) into a shared helper
- **Dependencies**: None
- **Goal Impact**: Reduces duplication in critical entropy coding path
- **Acceptance**:
  - Single function handles coefficient token encoding
  - All coefficient-related tests pass
- **Validation**: 
  ```bash
  go test -race ./... -run Token
  ```

### Step 6: Extract Loop Filter Edge Processing
- **Deliverable**: Refactor 21-line duplicate in `loopfilter.go` (lines 93-113 and 131-147) into shared edge processing function
- **Dependencies**: Steps 1-2 (ensures loop filter is actually exercised)
- **Goal Impact**: Improves loop filter code maintainability
- **Acceptance**:
  - Single function handles both horizontal and vertical edge filtering
  - Loop filter behavior unchanged
- **Validation**: 
  ```bash
  go test -race ./...
  ```

### Step 7: Add Inter-Frame Integration Test with External Decoder
- **Deliverable**: Create integration test that writes IVF file with key+inter frame sequence and validates with `ffprobe`
- **Dependencies**: Steps 1-2 (complete loop filter enables proper inter-frame testing)
- **Goal Impact**: Verifies inter-frame bitstream validity that `golang.org/x/image/vp8` cannot test
- **Acceptance**:
  - Test generates valid IVF file
  - `ffprobe -show_frames` reports correct frame types and count
  - Test is skipped gracefully if ffprobe not available
- **Validation**: 
  ```bash
  go test -v -run TestInterFrameFFprobe ./... || echo "Skipped: ffprobe not available"
  ```

### Step 8: Document Loop Filter Activation in README
- **Deliverable**: Update README.md to note that loop filter is now functional (remove implicit "disabled" status from GAPS.md findings)
- **Dependencies**: Steps 1-2
- **Goal Impact**: Documentation accuracy
- **Acceptance**: README accurately describes loop filter behavior
- **Validation**: Manual review

---

## Out of Scope (Acknowledged Blocked/Deferred Items)

| Item | Reason | Reference |
|------|--------|-----------|
| B_PRED mode enablement | Bitstream encoding bug causes decode failures | macroblock.go:70 TODO |
| Token probability updates | Lower priority optimization; infrastructure exists | token.go:843 |
| Golden/AltRef frame API | Feature scope expansion; not currently advertised | refframe.go |
| Sub-pixel motion estimation | Explicit design limitation | README.md |

---

## Validation Commands Summary

```bash
# Run all tests with race detector
go test -race ./...

# Check duplication ratio
go-stats-generator analyze . --skip-tests --format json --sections duplication | \
  python3 -c "import sys,json; d=json.loads(sys.stdin.read().split('100.0%)')[-1]); print(f'Duplication: {d[\"duplication\"][\"duplication_ratio\"]*100:.1f}%')"

# Check complexity of refactored functions
go-stats-generator analyze . --skip-tests --format json --sections functions | \
  python3 -c "import sys,json; d=json.loads(sys.stdin.read().split('100.0%)')[-1]); high=[f for f in d['functions'] if f['complexity']['cyclomatic']>15]; print(f'Functions >15 complexity: {len(high)}')"

# Verify static analysis passes
go vet ./...
```

---

## Scope Assessment Rationale

| Metric | Value | Threshold | Assessment |
|--------|-------|-----------|------------|
| Functions above complexity 9 | 14 | 5-15 = Medium | Medium |
| Duplication ratio | 6.1% | 3-10% = Medium | Medium |
| Doc coverage gap | 1.6% | <10% = Small | Small |

**Overall: Medium** — Significant refactoring required for duplication reduction and feature enablement, but no architectural changes needed.
