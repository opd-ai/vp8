# Implementation Gaps — 2026-03-26

This document identifies gaps between the project's stated goals (README.md) and its current implementation.

---

## Gap 1: Loop Filter Not Encoded in Bitstream

- **Stated Goal**: README.md §Features claims "**Loop filter** for reference frame quality" and §API documents `SetLoopFilterLevel(level int)` with "Sets the loop filter strength (0–63)."

- **Current State**: 
  - `loopfilter.go:21-151` implements a complete simple loop filter
  - `encoder.go:133-145` provides `SetLoopFilterLevel()` API that stores the level
  - However, `bitstream.go:67` always encodes `loop_filter_level=0` regardless of configuration
  - `encoder.go:309` has `applyLoopFilter()` commented out with explanatory note

- **Impact**: 
  - Users calling `SetLoopFilterLevel(20)` see no effect
  - Inter-frame quality is degraded by blocking artifacts in reference frames
  - API is misleading — appears functional but has no effect

- **Closing the Gap**:
  1. Modify `bitstream.go:67` to accept loop filter level as parameter: `enc.putLiteral(uint32(loopFilterLevel), 6)`
  2. Pass `loopFilter.level` from `Encoder` struct through `BuildKeyFrame()` and `BuildInterFrame()`
  3. Uncomment `applyLoopFilter(&recon, e.loopFilter)` in `encoder.go:309`
  4. Add test verifying loop filter level appears in decoded frame header
  5. Validation: Encode frame with level=30, decode with ffprobe, verify `loop_filter_level` field

---

## Gap 2: B_PRED Mode Disabled

- **Stated Goal**: ROADMAP.md §1.3 claims full B_PRED implementation with all 10 sub-modes. The `bpred.go` file implements 400+ lines of B_PRED prediction code.

- **Current State**:
  - All 10 B_PRED sub-modes are implemented in `bpred.go:41-397`
  - `SelectBest4x4Mode()` evaluates all modes (`bpred.go:424-460`)
  - Mode selection in `macroblock.go:97` uses threshold comparison
  - **But:** `bPredSADThreshold=0` at `macroblock.go:72` means B_PRED is never selected
  - TODO comment at `macroblock.go:70` states "B_PRED encoding has a bitstream issue causing decode failures"

- **Impact**:
  - ~400 lines of B_PRED code provide no value to users
  - Compression efficiency reduced for high-detail content where B_PRED would excel
  - Blocks with local texture variations get suboptimal 16×16 predictions

- **Closing the Gap**:
  1. Debug `encodeBPredModes()` in `bitstream.go:137-142` — verify sub-block mode encoding matches decoder expectations
  2. Create minimal test case: single macroblock with B_PRED, verify round-trip
  3. Check probability tables used for sub-block mode encoding
  4. Once decode succeeds, set `bPredSADThreshold = 90` (10% improvement threshold)
  5. Validation: `TestBPredModeSelection` that encodes high-detail content and verifies B_PRED macroblocks decode correctly

---

## Gap 3: Token Probability Updates Not Used

- **Stated Goal**: ROADMAP.md §1.6 claims "Implement serialisation of coefficient probability table updates" is complete.

- **Current State**:
  - `token.go:843-874` implements `EncodeCoeffProbUpdates()` with full probability update logic
  - `token.go:877-889` implements `EncodeNoCoeffProbUpdates()` for no-update case
  - `bitstream.go:97` always calls `EncodeNoCoeffProbUpdates(enc)` — never uses updates
  - `interbitstream.go:224` also always calls `EncodeNoCoeffProbUpdates(enc)`

- **Impact**:
  - Encoder always uses VP8 default probability tables
  - Cannot adapt to content characteristics for better compression
  - Sequences with consistent coefficient distributions get suboptimal entropy coding

- **Closing the Gap**:
  1. Add coefficient histogram tracking during encoding (count token occurrences)
  2. After encoding first pass (or using previous frame stats), compute updated probabilities
  3. Compare update cost (bits to encode updates) vs compression benefit
  4. If beneficial, call `EncodeCoeffProbUpdates()` instead of `EncodeNoCoeffProbUpdates()`
  5. Validation: Compare file sizes for 30-frame video with/without probability updates

---

## Gap 4: Inter-Frame Decode Verification Limited

- **Stated Goal**: README.md claims inter frames are "valid VP8 bitstreams (RFC 6386)" compatible with WebRTC.

- **Current State**:
  - `inter_test.go:255-310` (`TestInterFrameKeyFrameDecodable`) only verifies key frames in a sequence
  - Inter frames are checked for correct tag bits but not decoded
  - `golang.org/x/image/vp8` decoder only supports key frames, so inter frames cannot be verified locally
  - No integration test with external decoder (ffmpeg, libvpx)

- **Impact**:
  - Inter-frame correctness is unverified by automated tests
  - Subtle bitstream errors could go undetected
  - Users must trust inter frames work without evidence

- **Closing the Gap**:
  1. Add integration test using `ffmpeg` or `ffprobe` to decode inter frames
  2. Create test that writes IVF file with key+inter sequence, runs `ffprobe -show_frames`
  3. Parse ffprobe output to verify frame count and types
  4. Alternatively, use CGo to call libvpx decoder in test (build tag guarded)
  5. Validation: `go test -tags=integration ./...` with external decoder

---

## Gap 5: Golden/AltRef Frame Updates Not Exposed

- **Stated Goal**: README.md §Features claims "Reference frame management (last, golden, alternate reference)."

- **Current State**:
  - `refframe.go:98-151` implements `updateLast()`, `updateGolden()`, `updateAltRef()`, `copyLastToGolden()`, `copyLastToAltRef()`
  - However, only `updateLast()` is called in encoding (`encoder.go:312`)
  - No API to trigger golden/altref frame updates
  - Inter-frame bitstream header always signals `refFrameLast` (`interbitstream.go`)

- **Impact**:
  - Only "last" reference frame is used for inter prediction
  - Long GOP sequences cannot benefit from golden frame for scene-cut recovery
  - Temporal scalability features are unavailable

- **Closing the Gap**:
  1. Add `SetGoldenFrameInterval(n int)` API to periodically copy last→golden
  2. Add `ForceGoldenFrame()` to manually trigger golden update
  3. Update inter-frame header encoding to signal golden frame usage when appropriate
  4. Validation: Test that golden frame updates improve quality after scene cuts

---

## Gap 6: Documentation Drift in README Limitations

- **Stated Goal**: README.md accurately describes encoder capabilities and limitations.

- **Current State**:
  - README §Limitations states "Simple loop filter only (no normal filter)" but loop filter is disabled
  - README does not mention B_PRED is disabled
  - README claims loop filter is functional ("recommended value: 20–40")

- **Impact**:
  - Users may configure loop filter expecting effect
  - Documentation does not match actual encoder behavior

- **Closing the Gap**:
  1. Add note to README §Limitations: "Loop filter parameters are accepted but not currently encoded in bitstream"
  2. Add note: "B_PRED (4×4 sub-block) mode is implemented but disabled due to bitstream encoding issue"
  3. Update when gaps 1 and 2 are resolved
  4. Validation: Manual review of README vs code behavior

---

## Priority Matrix

| Gap | Severity | Effort | Impact on Users |
|-----|----------|--------|-----------------|
| Gap 1: Loop filter disabled | HIGH | LOW | API misleading; quality loss |
| Gap 2: B_PRED disabled | HIGH | MEDIUM | Compression efficiency loss |
| Gap 3: Token updates unused | MEDIUM | MEDIUM | Suboptimal compression |
| Gap 4: Inter decode unverified | MEDIUM | LOW | Test coverage gap |
| Gap 5: Golden/AltRef unused | MEDIUM | MEDIUM | Feature incomplete |
| Gap 6: Documentation drift | LOW | LOW | User confusion |

---

## Summary

The encoder **achieves its core goal** of pure-Go VP8 encoding with valid key frames and functional inter frames. The main gaps are:

1. **API-behavior mismatch**: Loop filter API exists but has no effect
2. **Dead code**: B_PRED mode (~400 lines) and probability updates (~100 lines) are implemented but unused
3. **Test coverage**: Inter frames not decoded in tests
4. **Documentation**: Doesn't reflect disabled features

These gaps don't prevent the encoder from working for its primary use case (WebRTC video encoding), but they limit compression efficiency and mislead users about available features.
