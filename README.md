# vp8 — Pure-Go VP8 I-frame Encoder

[![Go Reference](https://pkg.go.dev/badge/github.com/opd-ai/vp8.svg)](https://pkg.go.dev/github.com/opd-ai/vp8)
[![CI](https://github.com/opd-ai/vp8/actions/workflows/ci.yml/badge.svg)](https://github.com/opd-ai/vp8/actions/workflows/ci.yml)

A minimal, pure-Go VP8 I-frame (key-frame) encoder with no CGo dependencies.

## Features

- Pure Go — no C libraries, no CGo
- Produces valid VP8 key-frame bitstreams (RFC 6386)
- Compatible with WebRTC stacks (pion/rtp VP8Payloader, ivfwriter)
- Configurable quantizer via bitrate target

## Limitations

- **I-frame only** — every `Encode` call produces a key frame
- No loop filter, segmentation, or temporal scalability

## Installation

```sh
go get github.com/opd-ai/vp8
```

## Usage

```go
import "github.com/opd-ai/vp8"

enc, err := vp8.NewEncoder(640, 480, 30)
if err != nil {
    log.Fatal(err)
}

// yuv is a YUV420 (I420) frame: Y plane then Cb then Cr
vp8Bytes, err := enc.Encode(yuv)
if err != nil {
    log.Fatal(err)
}
// vp8Bytes can be passed to pion/rtp VP8Payloader.Payload(mtu, vp8Bytes)
```

### Bitrate control

```go
enc.SetBitrate(1_000_000) // 1 Mbps → maps to a lower quantizer index
```

## API

### `NewEncoder(width, height, fps int) (*Encoder, error)`

Creates an encoder. Width and height must be positive even integers; fps must be > 0.

### `(*Encoder) Encode(yuv []byte) ([]byte, error)`

Encodes a YUV420 frame. The slice must be at least `width*height*3/2` bytes (Y plane, then Cb, then Cr).

### `(*Encoder) SetBitrate(bitrate int)`

Sets the target bitrate in bits/s (clamped to 100 kbps–8 Mbps). Maps to a VP8 quantizer index.

### `(*Encoder) ForceKeyFrame()`

No-op (every frame is already a key frame); provided for API compatibility.

### `NewYUV420Frame(yuv []byte, width, height int) (*Frame, error)`

Wraps a raw I420 buffer in a `Frame` struct for direct use with `BuildKeyFrame`.

### `BuildKeyFrame(width, height, qi int, mbs []macroblock) ([]byte, error)`

Low-level function that assembles the VP8 bitstream from pre-processed macroblocks.

## Output format

The returned byte slice is a raw VP8 bitstream as described in RFC 6386:

```
[3-byte frame tag][3-byte start code 9D 01 2A][2-byte width][2-byte height]
[first partition (bool-encoded header + MB modes)]
[second partition (DCT/WHT coefficient tokens for each macroblock)]
```

Residuals are computed via forward DCT (4×4 luma/chroma blocks) and WHT (16×16 DC values), then quantized and entropy-coded.

## Performance

Benchmark results on a typical developer machine (results may vary):

| Resolution | Time per frame | Throughput |
|------------|----------------|------------|
| 320×240 | ~1.5 ms | ~670 fps |
| 640×480 | ~5.8 ms | ~170 fps |
| 1280×720 | ~18 ms | ~55 fps |
| 1920×1080 | ~42 ms | ~24 fps |

Run benchmarks yourself with:

```sh
go test -bench=. -benchmem
```

## License
VP8 Encoder in Pure Go
