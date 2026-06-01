# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-05-30

## Project Profile
- **Purpose**: `github.com/opd-ai/vp8` is a pure‑Go VP8 video **encoder** implementing RFC 6386. It produces key (intra) and inter (P) frames with DCT/quantization, boolean entropy coding, intra prediction (16×16, 4×4 B_PRED, chroma), motion estimation/compensation, reference‑frame management, a simple loop filter, multi‑partition output, and adaptive coefficient‑probability updates.
- **Target users**: Go developers needing a dependency‑light VP8 encoder (e.g. to feed WebRTC/WebM stacks) without cgo/libvpx.
- **Deployment model**: In‑process library. All input (frame dimensions, pixel data, encoder config) originates from the calling Go program — there is **no untrusted network/file parsing** on the encode path. No `os/exec`, no SQL, no `net/http`, no goroutines/channels/mutexes in the core encoder.
- **Critical paths**: (1) key‑frame bitstream assembly + token coding (decoder‑validated), (2) boolean encoder carry/normalization, (3) inter‑frame motion estimation + MV delta coding, (4) reconstruction into reference buffers used by subsequent frames.

## Audit Scope
- **Packages audited**: 1 (`github.com/opd-ai/vp8`, the module root — a single flat package).
- **Source inspected**: all 16 non‑test `.go` files (~6,506 LOC): `encoder.go`, `frame.go`, `quant.go`, `partition.go`, `dct.go`, `macroblock.go`, `prediction.go`, `bpred.go`, `bool_encoder.go`, `bitstream.go`, `token.go`, `loopfilter.go`, `refframe.go`, `motion.go`, `inter.go`, `interbitstream.go`. Test files were read where needed to confirm validation strategy.
- **go-stats-generator metrics**: 216 functions/methods, 20 structs, avg cyclomatic complexity **4.1**, **0** functions above complexity 15 (max overall‑score function `encodeFrameHeaderWithProbs`, complexity ≈10.6, 74 lines), doc coverage **96%**, duplication ratio **1.87%** (10 clone pairs, all benign probability‑table literals).
- **Baseline tooling**: `go vet ./...` → clean; `go test -race ./...` → all pass (~2.3s).

## Coverage Log
Single package; all checklist categories completed.

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| `vp8`   | ✅       | ✅     | ✅        | ✅           | ✅ (N/A)       | ✅          | ✅          | ✅      | ✅     |

Concurrency (3f) is marked N/A: the encoder contains no goroutines, channels, mutexes, or shared mutable global state on the encode path; `go test -race` passes.

## Goal-Achievement Summary
| Stated Goal (README) | Status | Blocking Findings |
|----------------------|--------|-------------------|
| Pure Go, no cgo/external libvpx | ✅ | — |
| Key (intra) frame encoding, decodable by a real VP8 decoder | ✅ (validated by `golang.org/x/image/vp8` round‑trip in tests) | — |
| Inter (P) frame encoding with motion estimation/compensation | ⚠️ | H1 (MV predictor diverges from RFC 6386 §18.2; never validated against an inter‑capable decoder) |
| Diamond‑search motion estimation | ⚠️ | L-INTER (only ZEROMV/NEWMV are ever emitted; NEARESTMV/NEARMV never used despite candidate collection) |
| Adaptive coefficient probability updates | ✅ | — |
| Loop filter (simple) | ✅ | — |
| Multi‑partition output | ✅ | — |
| Integer‑pel motion only (documented limitation) | ✅ (matches code) | — |

## Findings

### CRITICAL
_None confirmed._ No data‑corruption, security‑exploit, or fully‑non‑functional‑documented‑feature bug was found with a reachable data path. Key‑frame output is validated against a real decoder; `go vet` and `go test -race` are clean.

### HIGH
- [x] **MV predictor diverges from RFC 6386 §18.2; inter‑frame output never validated against an inter‑capable decoder** — `motion.go:331` (`selectBestTwo`), `motion.go:367` (`findNearestMV`), `inter.go:53` (`mb.predMV = nearestMV`), `interbitstream.go:148-153` (`encodeMV` codes `mv - predMV`) — bug class: API/behavioral contract + logic. 
  - **Data flow**: For each inter MB, `findNearestMV` collects up to three neighbor MVs and `selectBestTwo` returns the **most frequent** neighbor MV as `nearest`. `inter.go:53` stores that as `mb.predMV`. NEWMV is then delta‑coded as `mv - predMV` (`interbitstream.go:149-153`). A spec‑compliant decoder (libvpx `vp8_find_near_mvs`) does **not** use a plain frequency count: it accumulates left/above/above‑right candidates with specific weights (left and above contribute weight 2 and are merged when equal), then **clamps** the resulting predictor to a bounded range around the macroblock (`vp8_clamp_mv2`). The encoder here applies neither the weighting/merge rules nor predictor clamping. Whenever (a) two or more distinct neighbor MVs are present, or (b) a single neighbor MV is large enough to be clamped by the decoder, the encoder's `predMV` differs from the decoder's predictor, so the decoder reconstructs `decoderPred + delta ≠ mv` — i.e. **wrong motion vectors and corrupted inter‑frame pixels** on any real VP8 decoder.
  - **Why it is undetected**: the bundled reference decoder `golang.org/x/image/vp8` **refuses to decode non‑key frames** — `DecodeFrame` returns `"vp8: Golden / AltRef frames are not implemented"` for any frame with `KeyFrame == false` (confirmed in the dependency's `decode.go`). The test helper `verifyKeyFrameDecodable` (`inter_test.go:870`) is only ever applied to key frames; inter frames are asserted only for structural properties (`frame2[0]&1 == 1`, `len ≥ 4`, `inter_test.go:861-866`). No test reconstructs inter‑frame pixels, so this defect cannot surface in CI.
  - **Confidence / uncertainty**: the algorithmic divergence is established by reading both implementations; it has **not** been empirically reproduced here because no inter‑frame‑capable VP8 decoder is available in this environment. Severity is HIGH (core documented feature, concrete code path, decode‑corrupting consequence) but the empirical‑decode step remains open.
  - **Remediation**: In `motion.go`, replace the frequency‑count predictor with the RFC 6386 §18.2 algorithm: weight left/above/above‑right candidates per spec, merge equal candidates, and clamp the chosen predictor to the allowed range relative to the MB position before storing it in `mb.predMV` (`inter.go:53`). Validate by adding an integration test that decodes encoded P‑frames with an inter‑capable decoder (e.g. libvpx via a test harness, or `ffmpeg`/`ffprobe` pixel comparison as already scaffolded in `TestInterFrameFFprobe`) and asserts reconstructed‑pixel equality within tolerance; gate with `go test -race ./...`.

### MEDIUM
- [x] **`coeffHistogram` counts can overflow `uint32` arithmetic in `computeSingleProb`** — `token.go:1020` (`falseCount*256/total`) — bug class: arithmetic overflow.
  - **Data flow**: `coeffHistogram` is `Reset()` only after key frames (`buildKeyFrameBitstream`) and accumulates across every inter frame within a GOP. In `computeSingleProb`, `falseCount` (a `uint32`) is multiplied by 256 before division. If a single histogram bin accumulates more than ~16.7M counts (`> 2^32/256`) over a very long key‑frame interval at high resolution, the multiplication wraps, producing a corrupted probability and (since the same probabilities are signaled to the decoder) a needlessly inefficient or skewed entropy model. It does not desynchronize the decoder (the wrong‑but‑agreed probability is transmitted), so it is a quality/robustness issue, not a decode‑corruption one.
  - **Remediation**: In `computeSingleProb` (`token.go` ~line 1015‑1025) widen the intermediate to `uint64` (`uint64(falseCount)*256/uint64(total)`) before truncating to the `uint8` probability, or periodically rescale/halve histogram bins. Validate with a unit test feeding a histogram bin > 2^24 and asserting the returned probability is in `[1,255]`; `go test -race ./...`.

### LOW
- [x] **`propagateCarry` carry‑propagation logic is incorrect (latent / unreachable)** — `bool_encoder.go:83-95` — bug class: logic / boolean. The function first zeroes a trailing run of `0xff` bytes, then a second loop re‑scans from the end and increments the **last (now‑zeroed)** byte instead of carrying into the byte **before** the run. Verified divergence in isolation: input `[5,255,255]` yields `[5,0,1]` instead of the correct `[6,0,0]`. **However**, instrumentation plus a 200‑iteration randomized stress test (varied sizes, partition counts, prob‑update settings) showed `propagateCarry` is **never invoked** — `hasCarry()` never returns true because `lowvalue` is masked to 24 bits (`& 0xffffff`) in `outputByte`, so the carry bit is never set at output time. No reachable data path; key‑frame output is decoder‑validated. **Remediation**: fix the second loop to increment `buf[i-1]` (the byte preceding the zeroed run) rather than `buf[i]`; add a direct unit test that forces a carry (e.g. drive `boolEncoder` so `lowvalue` overflows bit 24) and asserts byte‑exact output. Optional given unreachability. `go test -race ./...`.
- [x] **Dead `Predict4x4` computation in B_PRED mode evaluation** — `macroblock.go:211-217` — bug class: performance / dead code. `evaluateBPredMode` computes `Predict4x4(pred[:], …)` then discards `pred`, copying `src4x4` into the reconstruction buffer instead. The prediction result is unused, wasting work in the inner mode‑selection loop. This is a documented intentional approximation (mode cost is judged on source, not predicted, residual), so it is correctness‑neutral. **Remediation**: either drop the unused `Predict4x4` call or use its output for an accurate residual cost; benchmark with `go test -bench` on the macroblock path. Low priority.
- [x] **Only ZEROMV and NEWMV inter modes are ever emitted** — `motion.go:112-114` (`estimateMotion` sets `mode = mvModeZeroMV` or `mvModeNewMV`), candidate `near` from `selectBestTwo` is unused for mode selection — bug class: API/behavioral gap. NEARESTMV/NEARMV are collected but never chosen, so every non‑zero MV pays full NEWMV delta cost. Correctness‑neutral but reduces compression efficiency relative to the implied "motion estimation" capability. **Remediation**: extend `estimateMotion`/`inter.go` mode decision to test NEARESTMV/NEARMV (zero‑delta) candidates against NEWMV by rate‑distortion cost. Validate with `go test -race ./...` plus a size‑regression check. (Tracked in `GAPS.md`.)
- [x] **Theoretical `int16` overflow in `QuantizeBlock`** — `dct.go:98-117` — bug class: arithmetic. `coeffs[i] + acQ/2` is computed in `int16`; for pathological coefficient magnitudes near the `int16` bound this could wrap. In practice DCT outputs for 8‑bit input stay well within range and quant factors are clamped ≥4 (never 0, so no divide‑by‑zero). No reachable failure. **Remediation**: widen the rounding arithmetic to `int32` before storing back to `int16`; add a boundary unit test. Low priority. `go test -race ./...`.
- [x] **Stale deprecation annotations** — `quant.go:125`, `token.go:40-43` — bug class: documentation. Functions/vars are marked deprecated but remain in the exported surface without a removal plan, risking confusion for API consumers. **Remediation**: either remove the deprecated symbols or document the replacement and removal version in their GoDoc. Doc‑only; no test needed.

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total functions/methods | 216 |
| Functions above complexity 15 | 0 |
| Avg cyclomatic complexity | 4.1 |
| Doc coverage | 96% |
| Duplication ratio | 1.87% |
| Test pass rate | All pass (`go test -race ./...`, 1/1 package) |
| go vet warnings | 0 |

## False Positives Considered and Rejected
| Candidate | Reason Rejected |
|-----------|----------------|
| Loop‑filter out‑of‑bounds buffer access (`loopfilter.go`) | Edge loops iterate multiples of 4 strictly less than the even frame dimensions; `idx+1` and `(y+1)*stride` provably stay within the exact‑sized buffers. Verified by bounds analysis. |
| `propagateCarry` carry bug as HIGH/CRITICAL | Provably divergent in isolation but `propagateCarry` is never invoked (carry bit never set; `lowvalue` masked to 24 bits). No reachable path → demoted to LOW latent. |
| Reference buffers not padded to multiples of 16 (`refframe.go:60-71`) | Reconstruction loops guard every pixel write with `if py < height && px < width`; dimensions validated even/positive in `NewEncoder`/`NewYUV420Frame`. No OOB. |
| Map‑nil / type‑assertion panics | No user‑reachable nil maps or single‑value type assertions on the encode path; inputs are typed Go values, not decoded `any`. |
| `encodeMVComponent` "is_short" comment wording (`interbitstream.go:48`) | Comment is confusingly worded but the emitted bit values match the decoder; key‑frame tables and structure are decoder‑validated. Cosmetic only. |
| B_PRED probability‑table row "rearrangement" comments (`bitstream.go:320-560`) | Key‑frame B_PRED output is fully decoded by `golang.org/x/image/vp8` in tests; any wrong table row would fail the round‑trip. Validated. |
| Histogram persistence across inter frames | Matches VP8 persistent‑probability semantics and is consistent with the decoder; intentional, not a bug (overflow concern tracked separately as MEDIUM). |

## Remaining Scope
Audit complete for the single package `vp8`. No packages remain unaudited. The only open item is the **empirical** confirmation of finding **H1**, which requires an inter‑frame‑capable VP8 decoder not available in this environment (the bundled `golang.org/x/image/vp8` decodes key frames only). Recommend reproducing H1 with libvpx or `ffmpeg` in a follow‑up.

| Package | Status | Notes |
|---------|--------|-------|
| `vp8` | Audited (complete) | H1 pending empirical decode confirmation with an inter‑capable decoder. |
