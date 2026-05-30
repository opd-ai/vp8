# Implementation Gaps — 2026-05-30

## Inter‑frame motion‑vector prediction does not follow RFC 6386 §18.2
- **Stated Goal**: The README documents inter (P) frame encoding with motion estimation and compensation, positioning the encoder for real VP8 consumers (e.g. WebRTC/WebM stacks) that use a spec‑compliant decoder.
- **Current State**: The MV predictor (`motion.go:331` `selectBestTwo`, `motion.go:367` `findNearestMV`) picks the **most frequent** neighbor MV by a plain count. NEWMV is delta‑coded against this value (`inter.go:53`, `interbitstream.go:148-153`). A spec decoder (libvpx `vp8_find_near_mvs`) instead applies per‑neighbor weighting/merging and then **clamps** the predictor to a bounded range. Neither the weighting nor the clamping is implemented here.
- **Impact**: For any macroblock with two or more distinct neighbor MVs — or a single large neighbor MV that the decoder would clamp — the encoder's predictor differs from the decoder's. Since the absolute MV is reconstructed as `decoderPred + delta`, a real decoder recovers the **wrong** motion vector, corrupting inter‑frame pixels. This is invisible to the current test suite because `golang.org/x/image/vp8` cannot decode non‑key frames (it returns `"vp8: Golden / AltRef frames are not implemented"`), so no test reconstructs inter‑frame pixels.
- **Closing the Gap**: Reimplement the predictor in `motion.go` per RFC 6386 §18.2 (weighted left/above/above‑right accumulation, equal‑candidate merging, predictor clamping relative to the MB) and store the clamped result in `mb.predMV`. Add an integration test that decodes encoded P‑frames with an inter‑capable decoder (libvpx, or `ffmpeg`/`ffprobe` pixel comparison as scaffolded in `TestInterFrameFFprobe`) and asserts reconstructed‑pixel equality within tolerance.

## Inter‑frame output is not validated against any inter‑capable decoder
- **Stated Goal**: Produce standard‑compliant VP8 bitstreams that real decoders can play back, for both key and inter frames.
- **Current State**: Key frames are round‑trip validated via `golang.org/x/image/vp8` (`verifyKeyFrameDecodable`, `inter_test.go:870`). Inter frames are checked only for structural properties — the frame‑type bit and a minimum length (`inter_test.go:861-866`). No test decodes inter‑frame pixels because the bundled decoder rejects non‑key frames.
- **Impact**: Bitstream‑level defects in P‑frame coding (MV prediction, mode signaling, residual context) can pass CI undetected. Finding H1 in `AUDIT.md` is a concrete example that current tests cannot catch.
- **Closing the Gap**: Introduce an inter‑frame decode test path using a decoder that implements VP8 inter prediction (libvpx bindings or an `ffmpeg` subprocess), and compare decoded frames against the encoder's own reconstruction buffers within a PSNR/SAD tolerance.

## Only ZEROMV and NEWMV inter modes are emitted
- **Stated Goal**: "Diamond search motion estimation" implying effective inter‑frame compression.
- **Current State**: `estimateMotion` (`motion.go:112-114`) selects only `mvModeZeroMV` or `mvModeNewMV`. NEARESTMV/NEARMV candidates are collected by `findNearestMV`/`selectBestTwo` but never chosen, so every non‑zero motion pays the full NEWMV delta cost even when a neighbor MV would have coded for free.
- **Impact**: Lower compression efficiency than a complete VP8 mode set; the collected `near` candidate is effectively dead. Correctness is unaffected.
- **Closing the Gap**: Extend the inter mode decision in `motion.go`/`inter.go` to evaluate NEARESTMV and NEARMV (zero‑delta) against NEWMV by rate‑distortion cost, and select the cheapest. Add a size‑regression test on representative motion content.

## Stale deprecation annotations without a migration path
- **Stated Goal**: A clean, documented public API surface.
- **Current State**: Symbols at `quant.go:125` and `token.go:40-43` carry deprecation annotations but remain exported with no documented replacement or removal version.
- **Impact**: API consumers cannot tell which replacement to adopt or when the symbols disappear, encouraging accidental use of deprecated code.
- **Closing the Gap**: Document the replacement symbol and intended removal version in each deprecated declaration's GoDoc, or remove the symbols if no longer used internally.
