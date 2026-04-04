# vp8 — Pure-Go VP8 Encoder

[![Go Reference](https://pkg.go.dev/badge/github.com/opd-ai/vp8.svg)](https://pkg.go.dev/github.com/opd-ai/vp8)
[![CI](https://github.com/opd-ai/vp8/actions/workflows/ci.yml/badge.svg)](https://github.com/opd-ai/vp8/actions/workflows/ci.yml)

A pure-Go VP8 encoder with no CGo dependencies. Supports both key frames (I-frames) and inter frames (P-frames) with motion estimation.

## Features

- Pure Go — no C libraries, no CGo
- Produces valid VP8 bitstreams (RFC 6386)
- Compatible with WebRTC stacks (pion/rtp VP8Payloader, ivfwriter)
- Configurable quantizer via bitrate target
- **Inter-frame (P-frame) encoding** with motion estimation
- **Reference frame management** (last, golden, alternate reference)
- **Diamond search motion estimation** for efficient temporal prediction
- **Loop filter** for reference frame quality
- **Configurable key frame interval** for optimal compression
- Multi-partition support (1, 2, 4, or 8 partitions)
- Per-plane quantizer deltas for fine-tuned quality

## Limitations

- No sub-pixel motion estimation (integer-pel only)
- No segmentation or temporal scalability
- Simple loop filter only (no normal filter)

## Installation

```sh
go get github.com/opd-ai/vp8
```

## Usage

### I-frame only mode (default, backward compatible)

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

### Inter-frame mode (P-frames with motion estimation)

```go
enc, err := vp8.NewEncoder(640, 480, 30)
if err != nil {
    log.Fatal(err)
}

// Enable inter-frame encoding: key frame every 30 frames (1 per second at 30fps)
enc.SetKeyFrameInterval(30)

// Optional: enable loop filter for better reference frame quality
enc.SetLoopFilterLevel(20)

// Encode a sequence of frames
for _, yuv := range frames {
    vp8Bytes, err := enc.Encode(yuv)
    if err != nil {
        log.Fatal(err)
    }
    // First frame is a key frame, subsequent frames are inter frames
    // Inter frames use motion estimation from the previous frame
}
```

### Bitrate control

```go
enc.SetBitrate(1_000_000) // 1 Mbps → maps to a lower quantizer index
```

## API

### `NewEncoder(width, height, fps int) (*Encoder, error)`

Creates an encoder. Width and height must be positive even integers; fps must be > 0.

### `(*Encoder) Encode(yuv []byte) ([]byte, error)`

Encodes a YUV420 frame. The slice must be at least `width*height*3/2` bytes (Y plane, then Cb, then Cr). Returns either a key frame or inter frame depending on configuration.

### `(*Encoder) SetBitrate(bitrate int)`

Sets the target bitrate in bits/s (clamped to 100 kbps–8 Mbps). Maps to a VP8 quantizer index.

### `(*Encoder) SetKeyFrameInterval(interval int)`

Sets the maximum number of inter frames between key frames. A value of 0 (default) means every frame is a key frame. A value of N means one key frame followed by N-1 inter frames.

### `(*Encoder) ForceKeyFrame()`

Forces the next `Encode` call to produce a key frame, resetting the inter-frame prediction chain.

### `(*Encoder) SetLoopFilterLevel(level int)`

Sets the loop filter strength (0–63). The loop filter reduces blocking artifacts in reconstructed reference frames. Recommended value: 20–40 for inter-frame encoding.

### `(*Encoder) SetPartitionCount(count PartitionCount)`

Sets the number of DCT partitions (1, 2, 4, or 8).

### `(*Encoder) SetQuantizerDeltas(y1dc, y2dc, y2ac, uvdc, uvac int)`

Fine-tunes per-plane quantization with delta values added to the base quantizer index.

### `(*Encoder) SetProbabilityUpdates(enabled bool)`

Enables or disables adaptive coefficient probability updates. When enabled, the encoder tracks token statistics and updates probability tables in frame headers when doing so improves compression efficiency. Default is false.

### `NewYUV420Frame(yuv []byte, width, height int) (*Frame, error)`

Wraps a raw I420 buffer in a `Frame` struct for direct use with `BuildKeyFrame`.

## Output format

The returned byte slice is a raw VP8 bitstream as described in RFC 6386:

**Key frames:**
```
[3-byte frame tag (bit0=0)][3-byte start code 9D 01 2A][2-byte width][2-byte height]
[first partition (bool-encoded header + MB modes)]
[residual partitions (DCT/WHT coefficient tokens)]
```

**Inter frames:**
```
[3-byte frame tag (bit0=1)]
[first partition (header + MB modes + motion vectors)]
[residual partitions (DCT/WHT coefficient tokens)]
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

Note: Inter-frame encoding includes motion estimation overhead but typically produces smaller bitstreams for similar content.

Run benchmarks yourself with:

```sh
go test -bench=. -benchmem
```

## License
VP8 Encoder in Pure Go
