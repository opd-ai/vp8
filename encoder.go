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
	width   int
	height  int
	fps     int
	bitrate int // target bitrate in bits/s (used to derive quantizer)
	qi      int // quantizer index [0, 127]
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
		width:   width,
		height:  height,
		fps:     fps,
		bitrate: 500_000, // default 500 kbps
		qi:      24,      // default quantizer index
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
func (e *Encoder) ForceKeyFrame() {}

// Encode encodes a raw YUV420 (I420) frame and returns a VP8 key-frame
// bitstream. The yuv slice must be at least width*height*3/2 bytes long,
// laid out as the full luma plane followed by the Cb then Cr chroma planes.
//
// The returned bytes can be passed directly to pion/rtp's VP8Payloader.Payload.
func (e *Encoder) Encode(yuv []byte) ([]byte, error) {
	// Validate input buffer size against configured dimensions.
	frame, err := NewYUV420Frame(yuv, e.width, e.height)
	if err != nil {
		return nil, err
	}

	mbW := (e.width + 15) / 16
	mbH := (e.height + 15) / 16

	// Get quantization factors for the current quantizer index
	qf := GetQuantFactorsSimple(e.qi)

	// Chroma dimensions (half of luma)
	chromaW := e.width / 2
	chromaH := e.height / 2

	numMBs := mbW * mbH
	mbs := make([]macroblock, numMBs)

	for mbY := 0; mbY < mbH; mbY++ {
		for mbX := 0; mbX < mbW; mbX++ {
			mbIdx := mbY*mbW + mbX

			// Extract 16x16 luma block
			var srcY [256]byte
			for row := 0; row < 16; row++ {
				srcRow := mbY*16 + row
				if srcRow >= e.height {
					srcRow = e.height - 1
				}
				for col := 0; col < 16; col++ {
					srcCol := mbX*16 + col
					if srcCol >= e.width {
						srcCol = e.width - 1
					}
					srcY[row*16+col] = frame.Y[srcRow*e.width+srcCol]
				}
			}

			// Extract 8x8 chroma blocks (U and V)
			var srcU, srcV [64]byte
			for row := 0; row < 8; row++ {
				srcRow := mbY*8 + row
				if srcRow >= chromaH {
					srcRow = chromaH - 1
				}
				for col := 0; col < 8; col++ {
					srcCol := mbX*8 + col
					if srcCol >= chromaW {
						srcCol = chromaW - 1
					}
					srcU[row*8+col] = frame.Cb[srcRow*chromaW+srcCol]
					srcV[row*8+col] = frame.Cr[srcRow*chromaW+srcCol]
				}
			}

			// Build neighbor context
			ctx := e.buildMBContext(frame, mbX, mbY, mbW, mbH)

			// Process the macroblock
			mbs[mbIdx] = processMacroblock(srcY[:], srcU[:], srcV[:], ctx, qf)
		}
	}

	return BuildKeyFrame(e.width, e.height, e.qi, mbs)
}

// buildMBContext extracts neighbor pixels for prediction.
func (e *Encoder) buildMBContext(frame *Frame, mbX, mbY, mbW, mbH int) *mbContext {
	ctx := &mbContext{}

	chromaW := e.width / 2

	// Extract luma neighbors (16 pixels above, 16 to the left)
	if mbY > 0 {
		ctx.lumaAbove = make([]byte, 16)
		aboveRow := (mbY*16 - 1) * e.width
		for i := 0; i < 16; i++ {
			col := mbX*16 + i
			if col < e.width {
				ctx.lumaAbove[i] = frame.Y[aboveRow+col]
			}
		}
	}

	if mbX > 0 {
		ctx.lumaLeft = make([]byte, 16)
		leftCol := mbX*16 - 1
		for i := 0; i < 16; i++ {
			row := mbY*16 + i
			if row < e.height {
				ctx.lumaLeft[i] = frame.Y[row*e.width+leftCol]
			}
		}
	}

	if mbX > 0 && mbY > 0 {
		ctx.lumaTopLeft = frame.Y[(mbY*16-1)*e.width+(mbX*16-1)]
	} else {
		ctx.lumaTopLeft = 128
	}

	// Extract chroma neighbors (8 pixels above, 8 to the left)
	if mbY > 0 {
		ctx.chromaAboveU = make([]byte, 8)
		ctx.chromaAboveV = make([]byte, 8)
		aboveRow := (mbY*8 - 1) * chromaW
		for i := 0; i < 8; i++ {
			col := mbX*8 + i
			if col < chromaW {
				ctx.chromaAboveU[i] = frame.Cb[aboveRow+col]
				ctx.chromaAboveV[i] = frame.Cr[aboveRow+col]
			}
		}
	}

	if mbX > 0 {
		ctx.chromaLeftU = make([]byte, 8)
		ctx.chromaLeftV = make([]byte, 8)
		leftCol := mbX*8 - 1
		chromaH := e.height / 2
		for i := 0; i < 8; i++ {
			row := mbY*8 + i
			if row < chromaH {
				ctx.chromaLeftU[i] = frame.Cb[row*chromaW+leftCol]
				ctx.chromaLeftV[i] = frame.Cr[row*chromaW+leftCol]
			}
		}
	}

	if mbX > 0 && mbY > 0 {
		ctx.chromaTopLeftU = frame.Cb[(mbY*8-1)*chromaW+(mbX*8-1)]
		ctx.chromaTopLeftV = frame.Cr[(mbY*8-1)*chromaW+(mbX*8-1)]
	} else {
		ctx.chromaTopLeftU = 128
		ctx.chromaTopLeftV = 128
	}

	return ctx
}

// Width returns the configured frame width in pixels.
func (e *Encoder) Width() int { return e.width }

// Height returns the configured frame height in pixels.
func (e *Encoder) Height() int { return e.height }

// FPS returns the configured frame rate.
func (e *Encoder) FPS() int { return e.fps }
