# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-06-04

## Project Profile

| Field | Value |
|-------|-------|
| **Module** | `github.com/opd-ai/vp8` |
| **Purpose** | Pure-Go VP8 encoder supporting I-frames and P-frames with motion estimation |
| **Target users** | Go developers needing VP8 encoding for WebRTC (pion/rtp) and media applications |
| **Deployment model** | Library — imported as a Go package |
| **Go version** | 1.25.0 |
| **Dependencies** | `golang.org/x/image v0.37.0` (test-only, used for decode verification) |
| **Critical paths** | `Encode()` → macroblock processing → DCT/quantization → bitstream assembly |

## Audit Scope

- **Packages audited**: 1 (`github.com/opd-ai/vp8`)
- **Source files**: 16 (excluding test files)
- **Total functions**: 216 (135 free functions, 81 methods)
- **Total structs**: 20
- **go-stats-generator metrics**: avg complexity 4.1, max complexity 10.6, doc coverage 96.1%, duplication 1.88%
- **Functions > 50 lines**: 3
- **Functions with cyclomatic complexity > 15**: 0
- **All tests pass**: yes (`go test -race ./...` clean)
- **go vet warnings**: 0

## Coverage Log

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API | 3k Perf |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|---------|
| `vp8`   | ✅       | ✅     | ✅        | ✅           | ✅             | ✅          | ✅          | ✅      | ✅     | ✅      |

## Goal-Achievement Summary

| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| RFC 6386 compliant VP8 key-frame encoding | ⚠️ | MEDIUM-1 (B_PRED context), LOW-4 (MV prediction spec deviation) |
| Inter-frame (P-frame) encoding with motion estimation | ⚠️ | MEDIUM-2 (no iteration cap on diamond search), LOW-5 (no decode-validation) |
| WebRTC-compatible bitstream output | ✅ | — |
| Pure Go, no CGo | ✅ | — |
| Support configurable quality and partitions | ⚠️ | MEDIUM-3 (SetPartitionCount lacks validation) |

## Findings

### CRITICAL

_No CRITICAL findings._

### HIGH

_No HIGH findings._

### MEDIUM

- [ ] **MEDIUM-1: `build4x4Context` — heap allocations in hot path** — `macroblock.go:238-241` — Performance — Every call to `build4x4Context` allocates two slices (`make([]byte, 9)` and `make([]byte, 4)`) on the heap. This function is called 16 times per macroblock for B_PRED evaluation, meaning 16×N allocations per frame where N is the total macroblock count. For a 1080p frame (8160 macroblocks), this results in ~130,560 unnecessary heap allocations per frame during mode selection. **Remediation:** Replace `make([]byte, 9)` and `make([]byte, 4)` with stack-allocated arrays `var aboveBuf [9]byte; var leftBuf [4]byte` and take slices of them. Validate with `go test -race ./... && go test -bench=.`.

- [ ] **MEDIUM-2: `smallDiamondSearch` — unbounded iteration count** — `motion.go:184-212` — Logic — The `smallDiamondSearch` function uses an infinite `for {}` loop that only breaks when no neighbor improves the current cost. While the 2-pixel step constraint makes convergence likely in practice, there is no iteration cap. A pathological cost surface (e.g., with quantization-induced plateaus or ties) could theoretically cause excessive iterations, impacting encoding latency. **Remediation:** Add an iteration counter with a reasonable cap (e.g., 100 iterations) and break if exceeded. Validate with `go test -race ./...`.

- [ ] **MEDIUM-3: `SetPartitionCount` — no input validation** — `encoder.go:200-202` — API — `SetPartitionCount` accepts any `PartitionCount` value without verifying it is one of the valid constants (0–3). An invalid value like 4 would cause `1 << count` = 16 partitions, exceeding the VP8 spec maximum of 8. The subsequent `BuildPartitionSizes` and frame assembly would produce a malformed bitstream. **Remediation:** Add validation: `if count < OnePartition || count > EightPartitions { return error }` or clamp to valid range. Validate with `go test -race ./...`.

- [ ] **MEDIUM-4: Chroma NZ context uses stale mask for V plane** — `bitstream.go:885-886` — Logic — `encodeChromaPlane` for V blocks uses the same input `ctx.leftNzMaskUV` and `ctx.upNzMaskUV[mbX]` as the U plane encoding on the line above. However, after encoding U, the context should be updated before encoding V. The current code computes `newLeftU, newUpU` from U encoding and `newLeftV, newUpV` from V encoding using the **same** original context, then combines them. Per RFC 6386 §13.3, each chroma plane's context is independent (U uses bits 0-1, V uses bits 2-3 of the mask), so encoding V with the pre-U-update mask is architecturally correct for this implementation — the U and V sub-planes use separate bit positions. However, the initial context extraction for V at `shift+2` vs `shift` means the V context comes from the *previous* macroblock's V results, which is correct. **After review: This is correct by design — WITHDRAWN.** _(Kept in false positives table below.)_

- [ ] **MEDIUM-5: `encodeLargeMV` silently truncates MV magnitudes ≥ 1024** — `interbitstream.go:122-149` — Logic — The function encodes MV magnitude using bits 0–9 (10 bits, max value 1023). Bit 3 is conditionally encoded based on `v & 0xFFF0 != 0`. Values ≥ 1024 would have bits 10+ silently lost during encoding, producing a corrupted MV in the bitstream. The encoder's diamond search range is bounded to ±64 pixels (256 qpel), and MV deltas from the predictor are typically small. However, there is no explicit bounds check on the delta `mv.dx - predMV.dx` before encoding. For large frames with large predictor values, the delta could approach the limit. **Remediation:** Add a bounds check or clamp in `encodeMVComponent` (line 42): `if absVal > 1023 { absVal = 1023 }`. Validate with `go test -race ./...`.

### LOW

- [ ] **LOW-1: `debugMB` mutable global variable** — `macroblock.go:5` — Concurrency — `var debugMB = false` is a package-level mutable variable accessed without synchronization. While the Encoder type is not documented as goroutine-safe, concurrent use of separate Encoder instances in tests (where `debugMB` is toggled in `trace_test.go:28-29`) could race with production encoders. In practice, `debugMB` is only modified in tests with proper defer cleanup. **Remediation:** Convert to a field on the Encoder struct or protect with `sync.Once`/atomic. Validate with `go test -race ./...`.

- [ ] **LOW-2: `SetQuality` silently clamps invalid input** — `encoder.go:190-192` — API — `SetQuality(qi int)` silently clamps values outside [0, 127] rather than returning an error. This is a design choice but may surprise callers who pass e.g. -1 or 200 and don't realize their value was clamped. **Remediation:** Document the clamping behavior in the function's GoDoc, or return an error for out-of-range values. Validate with `go test -race ./...`.

- [ ] **LOW-3: `NewYUV420Frame` — no dimension validation** — `frame.go:29-38` — API — `NewYUV420Frame` allocates Y/Cb/Cr planes based on the provided width/height without checking for zero or negative values. A caller passing width=0 would get zero-length slices, and subsequent encoding would panic at macroblock extraction. The `Encode()` method validates dimensions before use, providing an upstream guard. **Remediation:** Add `if width <= 0 || height <= 0` check in `NewYUV420Frame`. Validate with `go test -race ./...`.

- [ ] **LOW-4: MV prediction algorithm deviates from RFC 6386 §18.2** — `motion.go:268-340` — API/Spec — The encoder uses a frequency-based nearest/near MV selection (selecting the most common candidate) rather than the spec's weighted accumulation algorithm with specific priority ordering. This is a valid simplification for an encoder (the decoder doesn't care how the encoder chose predictions), but it means the encoder's MV coding efficiency is suboptimal compared to a spec-conformant implementation. **Remediation:** Implement the RFC 6386 §18.2 weighted accumulation for better MV prediction coding efficiency. Validate with `go test -race ./...`.

- [ ] **LOW-5: Inter-frame bitstream not decode-validated** — `inter_test.go` — Testing — Tests for inter-frame encoding only verify structural properties (frame-type bit, minimum byte length) because `golang.org/x/image/vp8` cannot decode P-frames ("Golden / AltRef not implemented"). A malformed inter-frame bitstream would not be caught by the test suite. **Remediation:** Add integration tests using an external VP8 decoder (e.g., libvpx via CGo test build, or ffmpeg command-line validation). Validate round-trip decode of multi-frame sequences.

- [ ] **LOW-6: `buildFrameTag` accepts negative `firstPartSize`** — `bitstream.go:712-721` — API — The function checks `firstPartSize > maxFirstPartSize` but not `firstPartSize < 0`. A negative value would underflow when cast to `uint32`, producing a garbage frame tag. In practice, `firstPartSize` is always `len(firstPart)` which is non-negative. **Remediation:** Add `if firstPartSize < 0` guard. Validate with `go test -race ./...`.

- [ ] **LOW-7: `NewEncoder` does not validate odd dimensions** — `encoder.go:108-120` — API — `NewEncoder` checks `width <= 0 || height <= 0` but the comment and `buildKeyFrameWithProbs` (line 644) also require dimensions to be even (`width%2 != 0 || height%2 != 0`). `NewEncoder` does not enforce this — a caller could create an encoder with odd dimensions and only get an error at `Encode()` time. **Remediation:** Move the even-dimension check to `NewEncoder`. Validate with `go test -race ./...`.

- [ ] **LOW-8: `PartitionCount` type allows arbitrary values** — `partition.go:10-17` — API — `PartitionCount` is defined as `int` with constants 0–3. Nothing prevents callers from using values outside this range (e.g., `encoder.SetPartitionCount(10)`). The `NumPartitions()` method computes `1 << int(pc)` which for pc=10 gives 1024, far exceeding the VP8 spec. Related to MEDIUM-3. **Remediation:** Validate in `NumPartitions()` or make the type unexported. Validate with `go test -race ./...`.

- [ ] **LOW-9: `encodeFrameHeaderWithProbs` maximum complexity function** — `bitstream.go:145-400` (approx) — Maintenance — This function has the highest cyclomatic complexity (10.6) in the project and handles frame header encoding for all modes. While not a bug, its length and branching make it the most likely location for future regressions. **Remediation:** Consider extracting sub-functions for loop filter params, segmentation header, and mode probability encoding. No functional change needed.

- [ ] **LOW-10: Token encoding uses two distinct code paths** — `token.go:600-650` vs `token.go:400-500` — Maintenance — `encodeCoefficients` and `EncodeToken`/`encodeTokenTree` implement coefficient encoding through different logic paths. The former skips the EOB decision point (starts at probability index 1) while the latter handles it. This is correct by design (the caller handles the "has coefficients" signal separately) but creates a maintenance hazard where changes to one path might not be reflected in the other. **Remediation:** Add a cross-reference comment linking the two code paths and their relationship.

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total functions | 216 |
| Functions above complexity 15 | 0 |
| Max cyclomatic complexity | 10.6 (`encodeFrameHeaderWithProbs`) |
| Avg cyclomatic complexity | 4.1 |
| Doc coverage | 96.1% |
| Duplication ratio | 1.88% |
| Test pass rate | All pass |
| go vet warnings | 0 |
| Race detector issues | 0 |

## False Positives Considered and Rejected

| Candidate | Reason Rejected |
|-----------|----------------|
| Loop filter horizontal edge out-of-bounds (`loopfilter.go:228`) | Traced the loop bounds: `y` iterates as `4, 8, ..., < height` (step 4). Max y is at most `height-2` for even heights, so `y+1 ≤ height-1`. Index `(y+1)*stride + x` is always within the `width*height` buffer. Safe for all valid (even, positive) dimensions. |
| `extractLumaBlock` returns slice backed by local array (`encoder.go:451`) | The returned slice is immediately consumed within the same call frame (passed to DCT/predict functions). The array lives on the stack until the calling function returns. No escape after use. Safe. |
| `clampMVPredictor` int64→int16 truncation (`motion.go:438-460`) | The function explicitly clamps values to `[-borderQPel, dim+borderQPel]` before the int16 cast. Maximum possible value is frame_dimension + 128, well within int16 range (32767) for any practical frame size. Safe. |
| Chroma NZ context for V plane uses pre-U-update mask (`bitstream.go:885-886`) | U and V occupy separate bit positions in the context mask (U: bits 0-1, V: bits 2-3). They are extracted independently using different shift values. Encoding order does not affect correctness since the combined mask is assembled after both are encoded. Correct by design. |
| `simpleFilterVerticalEdge` q1 access at right edge (`loopfilter.go:199-213`) | The guard `x >= stride` prevents access when x equals stride. The loop iterates x as `4, 8, ..., < width` (step 4), and stride = width, so max x is always < stride. Access to `plane[idx+1]` at max x is safe. |
| `CopyCoeffProbs` deep copy correctness (`token.go:864-874`) | All array elements are fixed-size `[11]uint8` — `copy()` on these slices produces a true deep copy with no shared backing arrays. Correct. |

## Remaining Scope

All packages and all checklist categories have been audited. No remaining scope.
