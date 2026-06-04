# Implementation Gaps — 2026-06-04

## 1. MV Prediction Algorithm Deviates from RFC 6386 §18.2

- **Stated Goal**: RFC 6386 compliant VP8 encoding
- **Current State**: The encoder uses a frequency-based nearest/near MV candidate selection (`motion.go:268-340`) — it collects candidates from left, above, and above-right neighbors and selects the most common. RFC 6386 §18.2 specifies a weighted accumulation algorithm with specific priority ordering and tie-breaking rules for the `nearest_mv` and `near_mv` values communicated to the decoder.
- **Impact**: The encoder still produces valid VP8 bitstreams (the decoder doesn't enforce how the encoder selected MVs), but MV coding efficiency is reduced. The predictor mismatch means MVs are coded as larger deltas than necessary, increasing bitstream size for equivalent quality.
- **Closing the Gap**: Implement the RFC 6386 §18.2 algorithm: scan left, above, and above-left neighbors with spec-defined weights, accumulate into `cnt[]` array, and apply the spec's sorting/selection logic to produce `nearest_mv` and `near_mv`.

## 2. Inter-Frame Output Not Decode-Validated

- **Stated Goal**: Working P-frame (inter-frame) encoding with motion estimation
- **Current State**: Tests for inter-frame encoding (`inter_test.go`) only verify structural properties: the frame-type bit is set correctly, and the output exceeds a minimum byte length. The bitstream is never decoded by a VP8 decoder to verify correctness.
- **Impact**: Bitstream-level bugs in inter-frame encoding (incorrect MV encoding, wrong residual coefficients, invalid partition structure) would go undetected by the test suite. Users may receive corrupted video output from multi-frame sequences without any test catching the regression.
- **Closing the Gap**: Add integration tests that decode P-frame output using an external VP8 decoder. Options: (1) CGo test build linking libvpx, (2) shell out to `ffmpeg`/`vpxdec` in tests, (3) wait for `golang.org/x/image/vp8` to support P-frame decoding.

## 3. Limited Inter-Frame Prediction Modes

- **Stated Goal**: Inter-frame encoding with motion estimation
- **Current State**: The encoder implements only `MV_NEW` (explicit MV per macroblock) for inter-frame prediction (`inter.go:60-90`). VP8 supports additional inter modes: `MV_NEAREST` (use nearest MV from neighbors), `MV_NEAR` (use second-nearest), and `MV_ZERO` (zero motion). The skip mode is implemented but only for macroblocks with zero residual.
- **Impact**: Bitstream size is significantly larger than necessary for sequences with uniform or low motion. Every macroblock encodes a full MV delta even when the optimal MV is the same as a neighbor's (which would be free with `MV_NEAREST`) or zero (free with `MV_ZERO`). This directly reduces compression efficiency.
- **Closing the Gap**: Implement mode decision logic that evaluates `MV_ZERO`, `MV_NEAREST`, and `MV_NEAR` candidates alongside `MV_NEW` in the motion estimation pipeline. Select the mode with the best rate-distortion tradeoff.

## 4. No Input Validation at API Boundary for Frame Data

- **Stated Goal**: Usable encoder library for Go developers
- **Current State**: `NewYUV420Frame` (`frame.go:29`) accepts any width/height without validation. `SetPartitionCount` (`encoder.go:200`) accepts arbitrary integer values without range checking. `NewEncoder` doesn't validate even dimensions (only checked later in `Encode()`). These deferred validations mean callers get errors deep in the encoding pipeline rather than at the configuration point.
- **Impact**: Poor developer experience — errors are reported far from where the invalid configuration was set, making debugging harder. A caller might configure an encoder, process significant data, and only discover the configuration was invalid when `Encode()` is called.
- **Closing the Gap**: Move all validation to the construction/configuration API boundary: validate dimensions in `NewEncoder` and `NewYUV420Frame`, validate partition count in `SetPartitionCount`, and return errors immediately for invalid configurations.
