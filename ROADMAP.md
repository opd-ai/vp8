# Roadmap — Path to VP8 Spec Compliance

This document tracks the work required to evolve `github.com/opd-ai/vp8` from its current
minimal I-frame skeleton into a fully RFC 6386–conformant VP8 encoder.  Items are grouped
into milestones that can be shipped independently.  Within each milestone, items are listed
roughly in dependency order.

---

## Current state (baseline)

| Capability | Status |
|---|---|
| Boolean arithmetic coder (RFC 6386 §7) | ✅ implemented |
| Key-frame bitstream framing (tag, start code, partitions) | ✅ implemented |
| Frame-header fields (color space, filter level, quantizer, entropy probs) | ✅ implemented |
| Intra prediction — DC_PRED (16×16) | ✅ implemented (predictor only, not used for residuals) |
| Macroblock mode signalling — DC_PRED + coeff_skip=1 | ✅ implemented |
| Residual DCT coefficients | ❌ all macroblocks marked skipped; no tokens written |
| Any intra mode other than DC_PRED | ❌ not implemented |
| Loop filter | ❌ disabled (level=0) |
| Inter (P-frame) coding | ❌ not implemented |
| Segmentation | ❌ disabled |
| Multiple DCT partitions | ❌ single partition only |
| Rate control | ⚠️ bitrate→QI linear mapping only |

---

## Milestone 1 — Real I-frame residual coding

> Goal: produce fully spec-conformant I-frames with actual pixel fidelity.

### 1.1 Accurate dequantization / quantizer tables

- Replace the linear `quantIndexToQp` approximation with the exact lookup tables from
  RFC 6386 §14 (separate DC and AC step sizes for Y, Y2, UV planes).
- Map `qi` → `{y_dc_q, y_ac_q, y2_dc_q, y2_ac_q, uv_dc_q, uv_ac_q}`.
- Expose per-plane quantizer deltas (`y_dc_delta`, `y2_dc_delta`, `y2_ac_delta`,
  `uv_dc_delta`, `uv_ac_delta`) in the frame header.

### 1.2 All 16×16 intra prediction modes

- Implement V_PRED, H_PRED, TM_PRED (in addition to the existing DC_PRED) for
  16×16 luma blocks (RFC 6386 §12.1).
- Implement the corresponding chroma predictors (V_PRED_CHROMA, H_PRED_CHROMA,
  TM_PRED_CHROMA) for 8×8 UV blocks (RFC 6386 §12.2).
- Pick the mode that minimises sum-of-absolute-differences (SAD) between the
  predicted and source block.

### 1.3 4×4 intra prediction sub-modes (B_PRED)

- Implement all ten 4×4 luma sub-modes: B_DC_PRED, B_TM_PRED, B_VE_PRED,
  B_HE_PRED, B_LD_PRED, B_RD_PRED, B_VR_PRED, B_VL_PRED, B_HD_PRED, B_HU_PRED
  (RFC 6386 §12.3).
- Signal B_PRED at the macroblock level and encode the chosen sub-mode for each
  of the 16 luma 4×4 blocks via the `bmode` probability tree (RFC 6386 §11.2).

### 1.4 Forward DCT and quantization

- Implement the VP8 integer 4×4 forward DCT (RFC 6386 §14.1) for residuals of
  luma 4×4 blocks.
- Implement the 4×4 WHT (Walsh-Hadamard Transform) used for the Y2 (DC-of-DC)
  plane (RFC 6386 §14.3).
- Apply per-plane quantization using the step sizes from §1.1 above.

### 1.5 Residual token entropy coding

- Implement the coefficient probability tables (RFC 6386 §13.4, default tables
  in Annexe B).
- Implement the token tree and value coding path for DCT coefficients
  (RFC 6386 §13.2–13.3): ZERO_TOKEN, ONE_TOKEN, TWO_TOKEN … DCT_VAL_CAT6.
- Emit coefficient tokens into the second (residual) partition for non-skip
  macroblocks.
- Update `macroblock.skip` based on whether all quantized coefficients are zero,
  and set `coeff_skip` accordingly in the first-partition MB header.

### 1.6 Entropy probability update

- Implement serialisation of coefficient probability table updates
  (RFC 6386 §13.4, `coeff_prob_update_flag`) so the encoder can communicate
  adapted probabilities to the decoder.

### 1.7 Multiple DCT partitions

- Support encoding residuals into 1, 2, 4, or 8 independent second-partition
  segments as signalled by `token_partition` (RFC 6386 §9.5).
- Write the per-partition size bytes between the first partition and the
  residual partitions (RFC 6386 §9.7).

---

## Milestone 2 — Loop filter

> Goal: reduce blocking artefacts; required for competitive visual quality.

### 2.1 Simple loop filter

- Implement the simple (2-tap) horizontal and vertical edge filters applied at
  macroblock and 4×4 sub-block boundaries (RFC 6386 §15.2).
- Signal `filter_type=0`, `loop_filter_level`, and `sharpness_level` in the
  frame header.

### 2.2 Normal (bicubic) loop filter

- Implement the full normal loop filter (RFC 6386 §15.3): interior edge filter
  and high-edge-variance (HEV) logic.
- Signal `filter_type=1` when selected.

### 2.3 Per-macroblock loop-filter level adjustments

- Implement the `mb_lf_adjustments` syntax (RFC 6386 §9.3): per-reference-frame
  and per-mode loop-filter deltas.
- Encode `mode_ref_lf_delta_update` in the frame header when deltas change.

---

## Milestone 3 — Inter (P-frame) coding

> Goal: enable temporal redundancy reduction for real video compression ratios.

### 3.1 Reference frame management

- Add `lastFrame`, `goldenFrame`, `altRefFrame` buffers to `Encoder`.
- Implement `refresh_last`, `refresh_golden_frame`, `refresh_alt_ref_frame`
  flags and copy-from-gold/copy-from-altref semantics (RFC 6386 §9.8–9.9).

### 3.2 Motion estimation

- Implement full-pixel block-matching search (e.g. three-step search or
  diamond search) for 16×16 macroblocks.
- Add half-pixel and quarter-pixel sub-pel interpolation (RFC 6386 §16):
  six-tap and bilinear filter modes.

### 3.3 Motion vector coding

- Implement the motion vector probability update and coding (RFC 6386 §17):
  MV component sign, magnitude classes, and fractional bits.
- Encode `mv_update_prob` in the frame header when MV probabilities change.

### 3.4 Inter prediction modes

- Implement NEARESTMV, NEARMV, ZEROMV, and NEWMV for 16×16 macroblocks
  (RFC 6386 §11.3).
- Add the `split_mv` mode to allow 4 independent 8×8, or 16 independent 4×4,
  motion vectors per macroblock (RFC 6386 §11.4).

### 3.5 Mode decision (rate-distortion optimisation)

- Compute R-D cost `J = D + λ·R` for each candidate intra/inter mode.
- Select the mode that minimises `J` for each macroblock.
- Implement Lagrangian multiplier derivation from the quantizer index.

### 3.6 P-frame frame header

- Wire inter-frame-specific syntax: `refresh_entropy_probs`, inter-mode
  probability tree update, `prob_intra`, `prob_last`, `prob_gf`
  (RFC 6386 §9.9–9.10).

---

## Milestone 4 — Segmentation and advanced rate control

> Goal: fine-grained quality and bitrate management.

### 4.1 Segmentation

- Implement the segmentation map (up to 4 segments) and per-segment quantizer
  and loop-filter overrides (RFC 6386 §9.3, §10).
- Add `update_mb_segmentation_map` and `update_segment_feature_data` to the
  frame header writer.

### 4.2 Proper rate control

- Implement a frame-level rate controller (e.g. buffer-fullness feedback or
  two-pass CBR/VBR) that selects `qi` to hit the configured bitrate target.
- Replace the current linear bitrate→QI mapping with a model calibrated on
  actual encoded frame sizes.

### 4.3 Temporal scalability / reference-frame selection policy

- Expose a `SetTemporalLayer` API that controls which reference frame is
  updated and which is used for prediction, enabling SVC-style temporal
  layering compatible with VP8 SVC extensions used by WebRTC.

---

## Milestone 5 — Conformance test suite

> Goal: verify bitstream correctness against reference decoder output.

- Add a conformance test that decodes each encoded frame with a reference VP8
  decoder (e.g. `golang.org/x/image/vp8` or a CGo wrapper around libvpx) and
  compares PSNR.
- Add fuzzing harness (`go test -fuzz`) targeting `BuildKeyFrame` and `Encode`.
- Add a golden-file test with known-good bitstream hashes for regression
  detection.

---

## RFC 6386 section cross-reference

| RFC 6386 section | Topic | Milestone |
|---|---|---|
| §7 | Boolean entropy coder | ✅ done |
| §9.1 | Frame tag | ✅ done |
| §9.2 | Key-frame header | ✅ done |
| §9.3 | Segmentation, loop-filter delta | M2.3 / M4.1 |
| §9.4 | Filter header | M2.1 |
| §9.5 | DCT partition count | M1.7 |
| §9.6 | Quantizer indices | M1.1 |
| §9.7 | Partition size bytes | M1.7 |
| §9.8–9.10 | Reference/inter frame header | M3.6 |
| §11.2 | Key-frame MB mode coding | M1.2, M1.3 |
| §11.3–11.4 | Inter MB mode coding | M3.4 |
| §12 | Intra prediction | M1.2, M1.3 |
| §13 | Residual decoding / token tree | M1.5, M1.6 |
| §14 | DCT/WHT and dequantization | M1.1, M1.4 |
| §15 | Loop filter | M2.1, M2.2 |
| §16 | Sub-pixel interpolation | M3.2 |
| §17 | Motion vectors | M3.3 |
| Annexe B | Default probability tables | M1.5 |
