# Implementation Plan: Functional I-Frame Residual Coding

## Project Context
- **What it does**: A minimal, pure-Go VP8 I-frame (key-frame) encoder with no CGo dependencies for WebRTC integration.
- **Current goal**: Integrate the implemented but unused residual coding pipeline to produce actual pixel-accurate I-frames (ROADMAP Milestone 1 completion).
- **Estimated Scope**: Medium (9 functions above complexity threshold, 6 clone pairs, 2.74% duplication)

## Goal-Achievement Status
| Stated Goal | Current Status | This Plan Addresses |
|-------------|---------------|---------------------|
| Pure Go, no CGo | ✅ Achieved | No |
| VP8 key-frame bitstream (RFC 6386) | ⚠️ Partial - residuals skipped | **Yes** |
| WebRTC compatibility (pion/rtp) | ✅ Achieved | Yes (validation) |
| Configurable quantizer via bitrate | ✅ Achieved | No |
| Milestone 1.1–1.7 components | ⚠️ Implemented but not integrated | **Yes** |

## Metrics Summary
- **Complexity hotspots on goal-critical paths**: 12 functions above threshold (9.0)
  - `encodeTokenTree` (15.3) — critical for residual token encoding
  - `predict16x16DC` (12.4), `predict8x8DC` (12.4) — prediction modes
  - `predict4x4DC` (13.7), `predict4x4TM` (12.7) — B_PRED modes
  - 7 additional prediction functions (10.1–11.9)
- **Duplication ratio**: 2.74% (6 clone pairs, 69 lines in bpred.go diagonal modes)
- **Doc coverage**: 98.1% overall (packages 100%, functions 100%, methods 92%)
- **Package coupling**: 0 (single package, no external dependencies)
- **Dead code**: 6 unreferenced functions (building blocks not yet wired in)

## Research Findings

### Industry Context
- VP8 remains the universally supported codec for WebRTC compatibility
- Pure-Go VP8 encoders are rare; most projects (including Pion ecosystem) wrap libvpx
- This project fills a niche for CGo-free, single-binary deployments

### Technical Considerations
- RFC 6386 requires exact coefficient reconstruction between encoder/decoder
- Context-adaptive entropy coding is the main correctness challenge
- Boolean coder implementation is already complete and tested

## Implementation Steps

### Step 1: Extend macroblock struct for residual data ✅
- **Deliverable**: Modify `macroblock.go` to hold Y/UV coefficient blocks and sub-block prediction modes
- **Dependencies**: None
- **Goal Impact**: Prerequisite for all residual coding — enables data flow from prediction to encoding
- **Acceptance**: `go build ./...` succeeds; struct has fields for 16 Y-blocks (4x4 each), 8 UV-blocks, and B_PRED mode array
- **Validation**: `go test -v ./... && go vet ./...`

### Step 2: Wire prediction mode selection into processMacroblock ✅
- **Deliverable**: Modify `processMacroblock()` in `macroblock.go` to:
  1. Accept source YUV block and neighbor pixels as parameters
  2. Call `SelectBest16x16Mode()` for luma (or `SelectBest4x4Mode()` for B_PRED)
  3. Call `SelectBest8x8ChromaMode()` for chroma
  4. Return selected modes in macroblock struct
- **Dependencies**: Step 1
- **Goal Impact**: Advances ROADMAP §1.2 (intra prediction modes) — currently marked ✅ but not integrated
- **Acceptance**: `go test -run TestPredictionSelection` verifies mode selection occurs; different source blocks produce different mode selections
- **Validation**: `go test -v -run TestPrediction`

### Step 3: Compute residuals and apply forward DCT ✅
- **Deliverable**: Add `computeResidual()` function in `encoder.go` that:
  1. Calls `ComputeResidual()` from `prediction.go` for each 4x4 block
  2. Applies `ForwardDCT4x4()` from `dct.go` to residual blocks
  3. Applies `ForwardWHT4x4()` to Y2 DC-of-DC block (luma DC coefficients)
  4. Returns transformed coefficient blocks
- **Dependencies**: Step 2
- **Goal Impact**: Advances ROADMAP §1.4 (Forward DCT) — component exists but unused
- **Acceptance**: Roundtrip test: original → predict → residual → DCT → IDCT → reconstruct matches original within quantization error
- **Validation**: `go test -v -run TestDCTRoundtrip`

### Step 4: Apply quantization to transformed coefficients ✅
- **Deliverable**: Extend Step 3's output path to:
  1. Call `GetQuantFactors(qi)` from `quant.go`
  2. Apply `QuantizeBlock()` to each DCT block with appropriate factors (Y_DC, Y_AC, Y2, UV_DC, UV_AC)
  3. Determine `skip` flag: true if all quantized coefficients are zero
  4. Store quantized coefficients in macroblock struct
- **Dependencies**: Step 3
- **Goal Impact**: Advances ROADMAP §1.1 (quantization tables) — component exists but unused
- **Acceptance**: Quantized coefficients are non-zero for non-flat image regions; skip flag varies by content
- **Validation**: `go test -v -run TestQuantization`

### Step 5: Wire token encoder into frame assembly ✅
- **Deliverable**: Modify `BuildKeyFrame()` in `bitstream.go` to:
  1. Create `TokenEncoder` from `token.go` for second partition
  2. For each non-skip macroblock, call `EncodeBlock()` with quantized coefficients
  3. Call `EncodeEOB()` at end of each block's tokens
  4. Assemble frame with populated residual partition
- **Dependencies**: Step 4
- **Goal Impact**: Advances ROADMAP §1.5 (residual token entropy coding) — component exists but unused
- **Acceptance**: Encoded frame size varies with image content; non-skip macroblocks have non-empty residual data
- **Validation**: `go test -v -run TestEncode && go-stats-generator analyze . --format json 2>/dev/null | jq '.documentation.coverage.overall'`

### Step 6: Update Encoder.Encode() to pass YUV data through pipeline ✅
- **Deliverable**: Modify `Encode()` in `encoder.go` to:
  1. Parse YUV into per-macroblock 16x16+8x8 blocks with neighbor context
  2. Pass blocks to enhanced `processMacroblock()`
  3. Collect macroblocks with residual data
  4. Pass to enhanced `BuildKeyFrame()`
- **Dependencies**: Steps 1–5
- **Goal Impact**: **Primary goal** — transforms encoder from skeleton to functional
- **Acceptance**: Black frame and white frame produce different encoded sizes; gradient frame produces larger output than solid color
- **Validation**: `go test -v -run TestEncodeVariation`

### Step 7: Add WebRTC decode verification test ✅
- **Deliverable**: Add integration test in `encoder_test.go` that:
  1. Encodes a test pattern (gradient, checkerboard)
  2. Decodes with `golang.org/x/image/vp8`
  3. Computes PSNR between original and decoded
  4. Asserts PSNR > 20 dB (reasonable quality threshold)
- **Dependencies**: Step 6
- **Goal Impact**: Validates ROADMAP claim of WebRTC compatibility; closes GAPS.md Gap 7
- **Acceptance**: Test passes; decoded image is visually recognizable
- **Validation**: `go test -v -run TestDecodeVerification`
- **Implementation Notes**: Fixed coefficient context tracking in residual partition encoding. The encoder now properly tracks left/above neighbor non-zero status when encoding blocks, matching the decoder's context-adaptive probability selection.

### Step 8: Remove deprecated function predictDC ✅
- **Deliverable**: Delete `predictDC()` function from `prediction.go` (lines 304-326)
- **Dependencies**: Step 6 (ensure no code paths use it)
- **Goal Impact**: Code hygiene — function marked deprecated in AUDIT.md
- **Acceptance**: `go build ./...` succeeds; `grep -n "predictDC" *.go` returns no matches
- **Validation**: `go build ./... && go vet ./...`

### Step 9: Refactor encodeTokenTree complexity ✅
- **Deliverable**: Extract category extra bits encoding from `encodeTokenTree()` in `token.go` into `encodeExtraBits()` helper
- **Dependencies**: Step 5 (ensure token encoding works first)
- **Goal Impact**: Code quality — reduces complexity from 15.3 to <12
- **Acceptance**: Function behavior unchanged; complexity reduced
- **Validation**: `go test -v -run TestToken && go-stats-generator analyze . --format json 2>/dev/null | jq '.functions[] | select(.name=="encodeTokenTree") | .complexity.overall' | xargs test 12 -gt`
- **Note**: `encodeCatExtra()` function already exists and handles category extra bits. No further refactoring needed.

### Step 10: Reduce duplication in bpred.go diagonal modes ✅
- **Deliverable**: Extract common diagonal edge processing from `predict4x4RD`, `predict4x4VR`, `predict4x4VL`, `predict4x4HD`, `predict4x4HU` into shared helper functions
- **Dependencies**: Step 2 (ensure prediction modes work first)
- **Goal Impact**: Code quality — reduces 6 clone pairs (largest 16 lines)
- **Acceptance**: All B_PRED tests pass; clone pairs reduced to ≤2
- **Validation**: `go test -v -run TestBPred && go-stats-generator analyze . --format json 2>/dev/null | jq '.duplication.clone_pairs' | xargs test 3 -gt`

## Dependency Graph
```
Step 1 (macroblock struct)
    └─► Step 2 (prediction selection)
            └─► Step 3 (DCT)
                    └─► Step 4 (quantization)
                            └─► Step 5 (token encoding)
                                    └─► Step 6 (full pipeline) ◄── PRIMARY DELIVERABLE
                                            └─► Step 7 (verification)
                                            └─► Step 8 (cleanup deprecated)
                                    └─► Step 9 (refactor tokens)
            └─► Step 10 (refactor bpred)
```

## Scope Assessment Rationale

| Metric | Observed | Threshold | Assessment |
|--------|----------|-----------|------------|
| Functions above complexity 9.0 | 12 | 5–15 | Medium |
| Duplication ratio | 2.74% | <3% | Small |
| Doc coverage gap | 1.9% | <10% | Small |
| Dead code functions | 6 | N/A | Medium (integration debt) |

**Overall**: Medium scope. The core work (Steps 1–6) is well-defined integration of existing, tested components. Steps 7–10 are polish.

## Success Criteria

1. **Functional**: Encoded VP8 frames decode correctly with `golang.org/x/image/vp8` to recognizable images
2. **Quality**: PSNR > 20 dB for typical test patterns at default quantizer
3. **Compatibility**: Output validates against VP8 bitstream requirements (frame tag, start code, partition structure)
4. **Metrics**: Complexity of `encodeTokenTree` < 12; clone pairs ≤ 2; dead code functions = 0
