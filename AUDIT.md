# VP8 Codec Audit Report

## Executive Summary

**Lines of code analyzed:** 6,326 (16 non-test source files)
**Total files audited:** 16 source files (plus 1 stale backup file)
**Major findings:** 23 issues identified
**Severity breakdown:** 3 Critical, 6 High, 9 Medium, 5 Low

This audit covers the pure-Go VP8 encoder implementation against RFC 6386. The encoder supports I-frame and P-frame encoding with motion estimation, loop filtering, multi-partition encoding, and adaptive probability updates. The most significant issues relate to motion vector encoding deviating from the spec, a pass-by-value bug in inter-frame B_PRED context propagation, and incomplete VP8 features (no decoder, no sub-pixel ME, no B_PRED in inter frames).

---

## Correctness Issues

### MV Component Short Encoding Uses Wrong Tree Structure
- **Issue**: `encodeMVComponent` encodes the short MV magnitude (0–7) as a flat 3-bit binary value using only `probs[2]`, `probs[3]`, `probs[4]`. RFC 6386 §17.1 specifies a 7-node binary tree that uses `probs[2]` through `probs[8]`, where the tree splits `{0,1}` vs `{2,3,4,5,6,7}`, then further splits recursively.
- **Location**: `interbitstream.go:55-70`
- **Severity**: Critical
- **Expected**: Short MV values (0–7) should be encoded via the spec's short_mv_tree using probs[2]–probs[8] with the structure: `{0,1}` vs `{2..7}`, `0` vs `1`, `{2,3}` vs `{4..7}`, etc.
- **Resolution**: [ ] Rewrite the short encoding branch to follow the RFC 6386 §17.1 short MV tree structure.

### MV Component Long Encoding Does Not Match VP8 Spec
- **Issue**: For magnitudes ≥ 8, the code encodes 10 raw bits MSB-to-LSB using `probs[9]`–`probs[18]`. Per RFC 6386 §17.1, the long format encodes bit 9 (the 512 flag) through bit 3 (the 8 flag) using probs[9]–probs[15], and then the low 3 bits use the same short MV tree (probs[2]–probs[8]).
- **Location**: `interbitstream.go:71-82`
- **Severity**: Critical
- **Expected**: Long encoding should split into high bits (probs[9]–probs[15] for bits 9 downto 3) and low 3 bits (the short tree), not a flat 10-bit encoding.
- **Resolution**: [ ] Rewrite long MV encoding to match RFC 6386 §17.1: encode high bits with probs[9..15], then low 3 bits via the short tree.

### Loop Filter Simple Filter Formula Deviates from RFC 6386
- **Issue**: `computeSimpleFilter` computes `filter_value = (p0 - q0 + 4) >> 3`, but RFC 6386 §15.2 specifies the simple loop filter formula as `filter_value = clamp(3*(q0 - p0) + hev_adjust, -128, 127)` with additional complexity for the normal filter. The simple filter specifically adjusts with a 2-tap filter kernel across the edge, not a simple difference with fixed rounding.
- **Location**: `loopfilter.go:86-103`
- **Severity**: High
- **Expected**: The simple loop filter should follow the exact formula from RFC 6386 §15.2, including the proper filter kernel and sub-block vs macroblock edge distinction.
- **Resolution**: [ ] Implement the simple filter per RFC 6386 §15.2 with proper filter formula, distinguishing between sub-block and macroblock edges.

### predict4x4HE Missing Top-Left Pixel (P) Context
- **Issue**: `predict4x4HE` uses a hardcoded `P = 129` for the top-left pixel. Per RFC 6386 §12.3, B_HE_PRED row 0 should use `avg3(above[-1], left[0], left[1])` where `above[-1]` is the true top-left pixel (P). The function does not receive the top-left pixel from its caller.
- **Location**: `bpred.go:284-316`
- **Severity**: Medium
- **Expected**: The top-left pixel P should be extracted from the `above` array (above[0]) if available, similar to how `predict4x4TM` handles it.
- **Resolution**: [ ] Pass the top-left pixel or extract it from the above array, and use the true P value for row 0 of HE prediction.

### Chroma Mode Selection Only Uses U Plane
- **Issue**: `processMacroblock` selects the best chroma mode using only the U plane (`SelectBest8x8ChromaMode(srcU, ...)`). The V plane is not considered in mode selection, so the chosen mode may be suboptimal for V.
- **Location**: `macroblock.go:95`
- **Severity**: Medium
- **Expected**: Best practice is to evaluate chroma mode SAD using both U and V planes combined, or at minimum consider both when making the mode decision.
- **Resolution**: [ ] Modify chroma mode selection to consider both U and V planes (sum of both SADs) for optimal mode decision.

### 8x8 Chroma DC Prediction Uses Full-Block DC Instead of Quadrant DC
- **Issue**: `predict8x8DC` computes a single DC value from all 8 above and all 8 left pixels. Per RFC 6386 §12.2, VP8 chroma DC prediction uses a per-quadrant DC computation: four separate 4x4 quadrants each get their own DC value based on available neighbors.
- **Location**: `prediction.go:217-239`
- **Severity**: High
- **Expected**: Chroma DC prediction should compute four separate DC values for the four 4×4 quadrants, where each quadrant averages its available above (4 pixels) and left (4 pixels) neighbors independently.
- **Resolution**: [ ] Implement per-quadrant chroma DC prediction as specified in RFC 6386 §12.2.

### Key Frame Dimension Encoding Missing Scale/Version Bits
- **Issue**: The key frame dimensions are written as raw `uint16` values. Per RFC 6386 §9.1, the dimension words encode `(scale << 14) | size` for both width and height. The current implementation works for scale=0 but doesn't explicitly handle the horizontal/vertical scale fields.
- **Location**: `bitstream.go:775-777`
- **Severity**: Low
- **Expected**: Explicitly mask the dimension values to 14 bits and set scale to 0 to be fully spec-compliant, ensuring dimensions > 16383 are rejected.
- **Resolution**: [ ] Add explicit 14-bit masking for dimensions and validate they fit within the VP8 limit (16383).

### Inter Frame B_PRED Not Supported in Mode Decision
- **Issue**: `processInterMacroblock` only compares inter mode against 16x16 intra modes. It never evaluates B_PRED as an intra fallback option. This means B_PRED mode is never selected for intra macroblocks within inter frames.
- **Location**: `inter.go:41-87`
- **Severity**: Medium
- **Expected**: When intra mode wins, the encoder should also evaluate B_PRED (same as key frame `selectLumaMode`) before committing to a 16x16 intra mode.
- **Resolution**: [ ] Add B_PRED evaluation to the intra fallback path in `processInterMacroblock`.

### Chroma UV Mode Not Encoded for Inter Macroblocks in Inter Frames
- **Issue**: In `encodeInterMBModes`, `encodeUVMode` is only called when `!mb.isInter`. Per RFC 6386 §16.3, inter macroblocks do not encode a chroma mode (chroma prediction is motion-compensated), so this is actually correct behavior. However, the `encodeInterMBMode` (legacy/unused function at line 113) unconditionally skips UV mode for inter blocks without the guard check, which is consistent but the function is dead code.
- **Location**: `interbitstream.go:113-167`
- **Severity**: Low
- **Expected**: Confirm that `encodeInterMBMode` (line 113) is dead code and can be removed.
- **Resolution**: [ ] Remove the unused `encodeInterMBMode` function to reduce confusion.

### Loop Filter Missing Macroblock vs Sub-Block Edge Distinction
- **Issue**: `filterPlane` applies the same filter at every 4-pixel boundary with no distinction between macroblock edges (every 16 pixels) and sub-block edges (every 4 pixels). VP8 spec (RFC 6386 §15) specifies stronger filtering at macroblock boundaries and lighter filtering at sub-block boundaries.
- **Location**: `loopfilter.go:67-81`
- **Severity**: High
- **Expected**: Macroblock boundaries (every 16 pixels for luma, every 8 pixels for chroma) should use a stronger filter than sub-block boundaries.
- **Resolution**: [ ] Implement differentiated filter strength for macroblock vs sub-block edges.

---

## Completeness Gaps

### No VP8 Decoder Implementation
- **Description**: The package only provides encoding functionality. There is no VP8 bitstream decoder.
- **VP8 Spec Reference**: RFC 6386 (entire document covers decode)
- **Impact**: Users must rely on external decoders (e.g., `golang.org/x/image/vp8`) for verification. Round-trip testing is limited.
- **Resolution**: [ ] Consider adding a decoder or documenting the dependency on external decoders.

### No Sub-Pixel Motion Estimation
- **Description**: Motion estimation operates at 2-full-pixel granularity only (MVs are snapped to multiples of 8 quarter-pixels, i.e., a 2-pixel grid). VP8 supports quarter-pixel precision with 6-tap filtering for much finer motion accuracy.
- **VP8 Spec Reference**: RFC 6386 §18.4 (sub-pixel interpolation)
- **Impact**: Significantly reduced compression efficiency for sequences with sub-pixel motion. This is explicitly documented as a limitation.
- **Resolution**: [ ] Implement sub-pixel motion estimation with 6-tap interpolation filters per RFC 6386 §18.4.

### No Segmentation Support
- **Description**: Segmentation is always disabled (`enc.putBit(128, false)` in frame header). VP8 supports up to 4 segments with per-segment quantizer and loop filter adjustments.
- **VP8 Spec Reference**: RFC 6386 §9.3 (segmentation)
- **Impact**: Cannot vary quality within a frame. Limits applicability for ROI (region of interest) encoding.
- **Resolution**: [ ] Implement segmentation for per-macroblock quality adjustment.

### No AltRef Frame Usage
- **Description**: The `refFrameAltRef` buffer is allocated and managed but never used for prediction. Only `refFrameLast` and `refFrameGolden` are active.
- **VP8 Spec Reference**: RFC 6386 §9.8 (reference frame management)
- **Impact**: Missing temporal prediction option that could improve compression for certain content patterns.
- **Resolution**: [ ] Implement AltRef frame prediction and management.

### No Rate Control
- **Description**: The bitrate-to-QI mapping is a rough linear approximation. There is no frame-level or macroblock-level rate control to maintain a target bitrate.
- **VP8 Spec Reference**: Not specified in RFC 6386 (encoder-side feature)
- **Impact**: Output bitrate is unpredictable; may cause buffer overflow/underflow in streaming scenarios.
- **Resolution**: [ ] Implement a rate control algorithm (e.g., single-pass CBR or two-pass VBR).

### No Skip MB Optimization in Residual Encoding
- **Description**: The encoder does not optimize the `prob_skip_false` value—it is hardcoded to 255. This means the decoder always expects a skip flag, but the probability is not tuned to the actual skip rate.
- **VP8 Spec Reference**: RFC 6386 §11.1
- **Impact**: Slightly suboptimal compression when the actual skip rate differs significantly from 255/256.
- **Resolution**: [ ] Compute optimal `prob_skip_false` from actual skip statistics before encoding.

### No Temporal Scalability / SVC
- **Description**: No support for temporal layers or scalable video coding.
- **VP8 Spec Reference**: VP8 supports temporal scalability through reference frame management
- **Impact**: Cannot produce layered streams for adaptive bitrate delivery.
- **Resolution**: [ ] Implement temporal layer support using VP8 reference frame management.

---

## Bugs Identified

### Pass-By-Value Bug in leftBModes Context Update (Inter Frame)
- **Issue**: In `updateBPredContext`, the `leftBModes` parameter is `[4]intraBMode` (a Go array, passed by value). Modifications to it inside the function are lost when the function returns. The caller in `encodeInterMBModes` expects `leftBModes` to be updated for the next macroblock's context.
- **Location**: `interbitstream.go:252`
- **Reproduction**: Encode an inter frame with B_PRED intra macroblocks side-by-side. The second macroblock's B_PRED context will use stale `leftBModes` values instead of the first macroblock's right column.
- **Resolution**: [ ] Change `leftBModes` parameter to a pointer (`*[4]intraBMode`) or return the updated value.

### Internal Functions Lack Dimension Validation Guards
- **Issue**: Internal functions like `refFrameManager.allocBuffer()`, `reconstructFrame`, and `extractLumaBlock` assume dimensions are positive and even, but do not validate this themselves. While `NewEncoder` validates dimensions at the public API boundary, internal callers (e.g., from tests or future refactoring) could pass invalid values and trigger index-out-of-range panics. This is a code hardening concern, not a current bug.
- **Location**: `refframe.go:60-71`
- **Reproduction**: Only possible if internal functions are called directly with invalid dimensions (bypassing `NewEncoder`).
- **Resolution**: [ ] Add defensive validation or document that internal functions assume pre-validated dimensions.

### Stale Coefficient Probabilities After Key Frame Reset
- **Issue**: When `useProbUpdates` is enabled, the encoder resets `coeffHistogram` after building the key frame bitstream (`e.coeffHistogram.Reset()` at line 332), but `e.coeffProbs` (the current probability state) is never reset to `DefaultCoeffProbs` on key frames. Per VP8 spec, all probabilities reset to defaults on key frames. If probabilities were previously updated for inter frames, they carry over incorrectly to the next GOP.
- **Location**: `encoder.go:328-333`
- **Reproduction**: Enable probability updates, encode several frames (building up adapted probs), then encode a key frame. The `coeffProbs` state will still contain the adapted values from the previous GOP.
- **Resolution**: [ ] Reset `e.coeffProbs` to `DefaultCoeffProbs` when encoding a key frame.

### Dead Code: Unused encodeInterMBMode Function
- **Issue**: `encodeInterMBMode` (line 113) is never called. The actual inter MB mode encoding uses `encodeInterMBModeWithContext` (line 270). The dead function is a maintenance hazard.
- **Location**: `interbitstream.go:113-167`
- **Reproduction**: N/A (dead code)
- **Resolution**: [ ] Remove `encodeInterMBMode` to reduce code surface.

### Dead Code: Unused encodeYMode Function
- **Issue**: `encodeYMode` (line 237) is never called in production code paths. The actual Y mode encoding uses `encodeYModeWithContext`. It may be called from the dead `encodeInterMBMode` function.
- **Location**: `bitstream.go:237-280`
- **Reproduction**: N/A (dead code)
- **Resolution**: [ ] Remove `encodeYMode` if confirmed unused, or mark as deprecated.

### Backup File Committed to Repository
- **Issue**: `macroblock.go.bak` is committed to the repository. This is a development artifact that should not be in version control.
- **Location**: `macroblock.go.bak`
- **Reproduction**: N/A
- **Resolution**: [ ] Remove `macroblock.go.bak` from the repository and add `*.bak` to `.gitignore`.

### Debug Variable Left in Production Code
- **Issue**: `var debugMB = false` is a package-level mutable variable used for debug logging via `fmt.Printf`. While currently set to `false`, its presence means any code that sets it to `true` will produce unstructured debug output to stdout, which could leak into production logs.
- **Location**: `macroblock.go:5`
- **Reproduction**: Set `debugMB = true` before encoding.
- **Resolution**: [ ] Remove the debug variable and associated conditional print statements, or gate behind build tags.

### Loop Filter Chroma Filtering Uses Luma Step Size
- **Issue**: `applyLoopFilter` calls `filterPlane` with step=4 for both luma and chroma. For chroma, sub-block boundaries are at 4-pixel intervals (since each chroma MB is 8×8 with 4 sub-blocks of 4×4), so step=4 is correct for sub-block filtering. However, the macroblock boundary for chroma should be at every 8 pixels, not every 4 pixels, and should use stronger filtering.
- **Location**: `loopfilter.go:44-45`
- **Reproduction**: Encode frames with loop filter enabled; chroma macroblock edges get the same filter strength as sub-block edges.
- **Resolution**: [ ] Differentiate macroblock (8-pixel) and sub-block (4-pixel) edges for chroma filtering.

### buildFrameTag First Partition Size Overflow
- **Issue**: `buildFrameTag` stores `firstPartSize` in 19 bits (bits 5–23 of the 3-byte tag). The maximum representable size is (2^19 - 1) = 524,287 bytes. If the first partition exceeds this, the frame tag silently overflows and the decoder will read the wrong partition boundary.
- **Location**: `bitstream.go:754-760`
- **Reproduction**: Encode a frame with an extremely large number of macroblocks or many B_PRED modes, producing a first partition > 512 KB.
- **Resolution**: [ ] Add a size check and return an error if `firstPartSize` exceeds the 19-bit limit.

---

## Resolution Checklist

- [ ] **Critical**: Rewrite short MV encoding to use RFC 6386 §17.1 tree structure — `interbitstream.go:55-70`
- [ ] **Critical**: Rewrite long MV encoding to match RFC 6386 §17.1 (high bits + short tree for low 3 bits) — `interbitstream.go:71-82`
- [ ] **High**: Implement correct simple loop filter formula per RFC 6386 §15.2 — `loopfilter.go:86-103`
- [ ] **High**: Implement per-quadrant chroma DC prediction per RFC 6386 §12.2 — `prediction.go:217-239`
- [ ] **High**: Differentiate macroblock vs sub-block edge filter strength — `loopfilter.go:67-81`
- [ ] **High**: Fix pass-by-value bug in `updateBPredContext` for leftBModes — `interbitstream.go:252`
- [ ] **High**: Reset `coeffProbs` to defaults on key frame when using prob updates — `encoder.go:328-333`
- [ ] **High**: Add first partition size overflow check in `buildFrameTag` — `bitstream.go:754-760`
- [ ] **Medium**: Fix `predict4x4HE` to use actual top-left pixel P — `bpred.go:284-316`
- [ ] **Medium**: Evaluate both U and V planes for chroma mode selection — `macroblock.go:95`
- [ ] **Medium**: Add B_PRED evaluation to inter frame intra fallback — `inter.go:41-87`
- [ ] **Medium**: Differentiate chroma macroblock vs sub-block edge filtering — `loopfilter.go:44-45`
- [ ] **Medium**: Remove dead code `encodeInterMBMode` — `interbitstream.go:113-167`
- [ ] **Medium**: Remove dead code `encodeYMode` — `bitstream.go:237-280`
- [ ] **Medium**: Compute optimal `prob_skip_false` from actual statistics — `bitstream.go:129`
- [ ] **Medium**: Implement sub-pixel motion estimation — `motion.go`
- [ ] **Low**: Add explicit 14-bit dimension masking in key frame header — `bitstream.go:775-777`
- [ ] **Low**: Remove `macroblock.go.bak` from repository — `macroblock.go.bak`
- [ ] **Low**: Remove or gate `debugMB` variable behind build tags — `macroblock.go:5`
- [ ] **Low**: Document external decoder dependency — `encoder.go:1-31`
- [ ] **Low**: Implement segmentation support — `bitstream.go:41`

## Summary Statistics

- **Total source files audited:** 16 (plus 1 stale backup file: `macroblock.go.bak`)
- **Total lines of source code:** 6,326
- **Critical issues:** 3
- **High priority:** 6
- **Medium priority:** 9
- **Low priority:** 5
- **Dead code instances:** 3 (2 functions + 1 backup file)
- **Missing VP8 features:** 7 (decoder, sub-pixel ME, segmentation, AltRef, rate control, skip optimization, temporal scalability)
