# Implementation Gaps — 2026-03-23

This document identifies gaps between the project's stated goals and its current implementation, based on the README, ROADMAP.md, and code analysis.

**UPDATE 2026-03-23**: Gaps 1, 2, and 7 have been closed through PLAN.md implementation.

---

## Gap 1: Residual Coding Pipeline Not Integrated ✅ CLOSED

- **Status**: CLOSED as of 2026-03-23
- **Resolution**: Full residual coding pipeline implemented in PLAN.md Steps 1-6:
  - `processMacroblock()` now computes DCT, WHT, quantization, and skip detection
  - `buildMBContext()` provides neighbor pixel context for prediction
  - `encodeResidualPartition()` properly encodes coefficients with context tracking
  - WebRTC decode verification test passes with PSNR 48+ dB for gradients

---

## Gap 2: Prediction Mode Selection Not Used ✅ CLOSED

- **Status**: CLOSED as of 2026-03-23
- **Resolution**: PLAN.md Step 2 wired mode selection into `processMacroblock()`:
  - `SelectBest16x16Mode()` called for luma
  - `SelectBest8x8ChromaMode()` called for chroma
  - Modes stored in macroblock struct and encoded in first partition

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

## Gap 7: WebRTC Interoperability Unverified ✅ CLOSED

- **Status**: CLOSED as of 2026-03-23
- **Resolution**: PLAN.md Step 7 added WebRTC decode verification test:
  - `TestDecodeVerification` in encoder_test.go tests round-trip encode/decode
  - Uses golang.org/x/image/vp8 decoder (same decoder used by pion/webrtc)
  - Verifies PSNR ≥ 15 dB for gradient images, exact match for solid colors
  - Test passes with PSNR 48+ dB for gradient content

---

## Priority Matrix (Updated)

| Gap | Severity | Effort | Status |
|-----|----------|--------|--------|
| Gap 1: Residual coding not integrated | HIGH | MEDIUM | ✅ CLOSED |
| Gap 2: Prediction mode selection | MEDIUM | LOW | ✅ CLOSED |
| Gap 3: B_PRED mode | LOW | MEDIUM | Open |
| Gap 4: Multiple partitions | LOW | LOW | Open |
| Gap 5: Quantizer deltas | LOW | LOW | Open |
| Gap 6: Probability updates | LOW | MEDIUM | Open |
| Gap 7: WebRTC interop tests | MEDIUM | LOW | ✅ CLOSED |

**Status**: Core I-frame encoder is now functional. Remaining gaps are optimizations.
