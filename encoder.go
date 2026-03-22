// Package vp8 provides a minimal pure-Go VP8 I-frame encoder.
//
// This implementation produces VP8 key-frames (I-frames) only. Inter-frame
// (P-frame) coding is not supported. The output is a raw VP8 bitstream
// suitable for RTP packetisation with github.com/pion/rtp/codecs.VP8Payloader,
// or for writing into an IVF container with github.com/pion/webrtc ivfwriter.
//
// Limitations:
//   - I-frame only (every Encode call produces a key frame).
//   - Residuals are skipped (all macroblocks coded as DC_PRED with zero
//     residuals). Quality is limited but the bitstream is valid.
//   - No loop filter, no segmentation, no temporal scalability.
//
// Usage:
//
//	enc, err := vp8.NewEncoder(640, 480, 30)
//	if err != nil { ... }
//	vp8Bytes, err := enc.Encode(yuvFrame)
package vp8

import (
	"errors"
	"fmt"
)

// Encoder encodes raw YUV420 frames into VP8 key-frame bitstreams.
type Encoder struct {
	width    int
	height   int
	fps      int
	bitrate  int  // target bitrate in bits/s (used to derive quantizer)
	qi       int  // quantizer index [0, 127]
	forceKey bool // force next frame to be a key frame (always true here)
}

// NewEncoder creates a new VP8 Encoder for frames of the given dimensions
// and frame rate. Both width and height must be positive even integers.
//
// fps is used for bitrate-to-quantizer mapping and must be > 0.
func NewEncoder(width, height, fps int) (*Encoder, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("vp8: width and height must be positive, got %dx%d", width, height)
	}
	if width%2 != 0 || height%2 != 0 {
		return nil, fmt.Errorf("vp8: width and height must be even, got %dx%d", width, height)
	}
	if fps <= 0 {
		return nil, errors.New("vp8: fps must be positive")
	}
	return &Encoder{
		width:    width,
		height:   height,
		fps:      fps,
		bitrate:  500_000, // default 500 kbps
		qi:       24,      // default quantizer index
		forceKey: true,
	}, nil
}

// SetBitrate configures the target output bitrate in bits per second.
// The encoder maps this to a VP8 quantizer index. Values outside the
// typical WebRTC range (100 kbps – 8 Mbps) are clamped.
func (e *Encoder) SetBitrate(bitrate int) {
	if bitrate < 100_000 {
		bitrate = 100_000
	}
	if bitrate > 8_000_000 {
		bitrate = 8_000_000
	}
	e.bitrate = bitrate
	// Rough linear mapping: higher bitrate → lower QI (better quality).
	// qi ∈ [4, 63]: 8 Mbps → qi=4, 100 kbps → qi=63.
	ratio := float64(e.bitrate-100_000) / float64(8_000_000-100_000)
	e.qi = 63 - int(ratio*59)
}

// ForceKeyFrame causes the next call to Encode to produce a key frame.
// In this implementation every frame is already a key frame, so this is
// a no-op kept for API compatibility.
func (e *Encoder) ForceKeyFrame() {
	e.forceKey = true
}

// Encode encodes a raw YUV420 (I420) frame and returns a VP8 key-frame
// bitstream. The yuv slice must be at least width*height*3/2 bytes long,
// laid out as the full luma plane followed by the Cb then Cr chroma planes.
//
// The returned bytes can be passed directly to pion/rtp's VP8Payloader.Payload.
func (e *Encoder) Encode(yuv []byte) ([]byte, error) {
	f, err := NewYUV420Frame(yuv, e.width, e.height)
	if err != nil {
		return nil, err
	}

	mbW := (e.width + 15) / 16
	mbH := (e.height + 15) / 16
	qp := quantIndexToQp(e.qi)

	mbs := make([]macroblock, 0, mbW*mbH)
	for by := 0; by < mbH; by++ {
		for bx := 0; bx < mbW; bx++ {
			mbs = append(mbs, processMacroblock(f, bx, by, qp))
		}
	}

	return BuildKeyFrame(e.width, e.height, e.qi, mbs)
}

// Width returns the configured frame width in pixels.
func (e *Encoder) Width() int { return e.width }

// Height returns the configured frame height in pixels.
func (e *Encoder) Height() int { return e.height }

// FPS returns the configured frame rate.
func (e *Encoder) FPS() int { return e.fps }
