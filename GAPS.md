# Implementation Gaps — 2026-06-04

## 1. MV Prediction Algorithm Deviates from RFC 6386 §18.2

- **Stated Goal**: RFC 6386 compliant VP8 encoding
- **Current State**: `findNearestMV` implements a weighted candidate count (left/above weight 2, diagonal weight 1) and clamps the chosen predictor (`motion.go:316-382`). If its selection/tie-breaking differs from RFC 6386 §18.2 / libvpx, the encoder and decoder can compute different predictors.
- **Impact**: For `MV_NEW`, the bitstream encodes only the delta from the decoder-defined predictor (`encodeMV` in `interbitstream.go:146-154`). If the encoder’s predictor differs, the decoder reconstructs a different MV (`pred_decoder + delta`), corrupting inter-frame prediction. Even when predictors match, suboptimal predictors increase MV delta size and bitstream size.
- **Closing the Gap**: Verify predictor behavior against a reference decoder (e.g., libvpx `vp8_find_near_mvs`) and adjust `findNearestMV` selection/tie-breaking to match RFC 6386 §18.2 (diagonal neighbor is above-right, or above-left at the right edge), ensuring `mb.predMV` matches the decoder’s predictor for `MV_NEW`.

## 2. Inter-Frame Output Not Decode-Validated

- **Stated Goal**: Working P-frame (inter-frame) encoding with motion estimation
- **Current State**: Tests for inter-frame encoding (`inter_test.go`) only verify structural properties: the frame-type bit is set correctly, and the output exceeds a minimum byte length. The bitstream is never decoded by a VP8 decoder to verify correctness.
- **Impact**: Bitstream-level bugs in inter-frame encoding (incorrect MV encoding, wrong residual coefficients, invalid partition structure) would go undetected by the test suite. Users may receive corrupted video output from multi-frame sequences without any test catching the regression.
- **Closing the Gap**: Add integration tests that decode P-frame output using an external VP8 decoder. Options: (1) CGo test build linking libvpx, (2) shell out to `ffmpeg`/`vpxdec` in tests, (3) wait for `golang.org/x/image/vp8` to support P-frame decoding.

## 3. Limited Inter-Frame Prediction Modes

- **Stated Goal**: Inter-frame encoding with motion estimation
- **Current State**: The encoder’s motion estimation currently selects only `MV_ZERO` (zero motion) or `MV_NEW` (explicit MV per macroblock) (`motion.go:85-118`, used by `inter.go:33-55`). VP8 also supports `MV_NEAREST` and `MV_NEAR`, and while those modes are encodable (`interbitstream.go:292-307`), the encoder never selects them during mode decision.
- **Impact**: Bitstream size is significantly larger than necessary for sequences with uniform or low motion. Every macroblock encodes a full MV delta even when the optimal MV is the same as a neighbor's (which would be free with `MV_NEAREST`) or zero (free with `MV_ZERO`). This directly reduces compression efficiency.
- **Closing the Gap**: Implement mode decision logic that evaluates `MV_ZERO`, `MV_NEAREST`, and `MV_NEAR` candidates alongside `MV_NEW` in the motion estimation pipeline. Select the mode with the best rate-distortion tradeoff.

## 4. No Input Validation at API Boundary for Frame Data

- **Stated Goal**: Usable encoder library for Go developers
- **Current State**: `NewYUV420Frame` (`frame.go:29`) accepts any width/height without validation. `SetPartitionCount` (`encoder.go:200`) accepts arbitrary integer values without range checking. `NewEncoder` doesn't validate even dimensions (only checked later in `Encode()`). These deferred validations mean callers get errors deep in the encoding pipeline rather than at the configuration point.
- **Impact**: Poor developer experience — errors are reported far from where the invalid configuration was set, making debugging harder. A caller might configure an encoder, process significant data, and only discover the configuration was invalid when `Encode()` is called.
- **Closing the Gap**: Move all validation to the construction/configuration API boundary: validate dimensions in `NewEncoder` and `NewYUV420Frame`, validate partition count in `SetPartitionCount`, and return errors immediately for invalid configurations.
