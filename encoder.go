// Package vp8 provides a pure-Go VP8 encoder supporting both key frames
// (I-frames) and inter frames (P-frames) with motion estimation.
//
// The encoder produces VP8 bitstreams per RFC 6386, compatible with
// WebRTC stacks (pion/rtp VP8Payloader, ivfwriter).
//
// In the default mode (keyFrameInterval=0), every Encode call produces a
// key frame for backward compatibility. When SetKeyFrameInterval is called
// with a positive value, the encoder produces inter frames using motion
// estimation from reference frames between key frames.
//
// Limitations:
//   - No sub-pixel motion estimation (integer-pel only).
//   - No segmentation, no temporal scalability.
//
// Usage (I-frame only):
//
//	enc, err := vp8.NewEncoder(640, 480, 30)
//	if err != nil { ... }
//	vp8Bytes, err := enc.Encode(yuvFrame)
//
// Usage (inter-frame):
//
//	enc, err := vp8.NewEncoder(640, 480, 30)
//	if err != nil { ... }
//	enc.SetKeyFrameInterval(30)
//	enc.SetLoopFilterLevel(20)
//	for _, yuv := range frames {
//	    vp8Bytes, err := enc.Encode(yuv)
//	}
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

	// Quantizer delta fields for per-plane adjustments.
	// These are added to the base qi for specific coefficient types.
	y1DCDelta int // Y1 DC coefficient delta
	y2DCDelta int // Y2 (WHT DC-of-DC) DC coefficient delta
	y2ACDelta int // Y2 AC coefficient delta
	uvDCDelta int // Chroma DC coefficient delta
	uvACDelta int // Chroma AC coefficient delta

	// partitionCount controls the number of DCT/residual partitions.
	// Default is OnePartition. Use SetPartitionCount to enable multi-partition encoding.
	partitionCount PartitionCount

	// Inter-frame encoding state.
	// refFrames manages the three reference frame buffers (last, golden, altref).
	refFrames *refFrameManager
	// frameCount tracks the number of frames encoded since the last key frame.
	frameCount int
	// keyFrameInterval is the maximum number of frames between key frames.
	// 0 means every frame is a key frame (I-frame only mode).
	// Default is 0 for backward compatibility.
	keyFrameInterval int
	// forceNextKeyFrame forces the next Encode call to produce a key frame.
	forceNextKeyFrame bool
	// loopFilter controls the loop filter parameters.
	loopFilter loopFilterParams
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
		refFrames: newRefFrameManager(width, height),
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
// This resets the inter-frame prediction chain.
func (e *Encoder) ForceKeyFrame() {
	e.forceNextKeyFrame = true
}

// SetKeyFrameInterval configures the maximum number of inter frames between
// key frames. A value of 0 means every frame is a key frame (I-frame only mode,
// which is the default for backward compatibility). A value of N means that
// every Nth frame will be a key frame, with inter frames in between.
//
// For example, SetKeyFrameInterval(30) at 30fps means one key frame per second.
func (e *Encoder) SetKeyFrameInterval(interval int) {
	if interval < 0 {
		interval = 0
	}
	e.keyFrameInterval = interval
}

// SetLoopFilterLevel configures the loop filter strength (0–63).
// The loop filter reduces blocking artifacts in reconstructed frames used
// as reference for inter-frame prediction. Level 0 disables the filter.
// A moderate level (e.g., 20–40) is recommended for inter-frame encoding.
func (e *Encoder) SetLoopFilterLevel(level int) {
	if level < 0 {
		level = 0
	}
	if level > 63 {
		level = 63
	}
	e.loopFilter.level = level
}

// SetPartitionCount configures the number of DCT/residual partitions.
// VP8 supports 1, 2, 4, or 8 partitions. Multiple partitions can enable
// parallel decoding and provide error resilience.
//
// Valid values: OnePartition, TwoPartitions, FourPartitions, EightPartitions.
// Default is OnePartition.
func (e *Encoder) SetPartitionCount(count PartitionCount) {
	e.partitionCount = count
}

// SetQuantizerDeltas configures per-plane quantizer delta values.
// These deltas are added to the base quantizer index (qi) for specific
// coefficient types, allowing fine-tuned quality trade-offs.
//
// Parameters:
//   - y1dc: delta for Y1 (luma 4x4 block) DC coefficients
//   - y2dc: delta for Y2 (WHT DC-of-DC) DC coefficients
//   - y2ac: delta for Y2 AC coefficients
//   - uvdc: delta for chroma DC coefficients
//   - uvac: delta for chroma AC coefficients
//
// Positive deltas increase quantization (lower quality, smaller size).
// Negative deltas decrease quantization (higher quality, larger size).
// Deltas are clamped so that qi+delta stays within [0, 127].
func (e *Encoder) SetQuantizerDeltas(y1dc, y2dc, y2ac, uvdc, uvac int) {
	e.y1DCDelta = y1dc
	e.y2DCDelta = y2dc
	e.y2ACDelta = y2ac
	e.uvDCDelta = uvdc
	e.uvACDelta = uvac
}

// Encode encodes a raw YUV420 (I420) frame and returns a VP8 bitstream.
// Depending on the configuration, this may produce either a key frame (I-frame)
// or an inter frame (P-frame) using motion estimation from reference frames.
//
// When keyFrameInterval is 0 (default), every frame is a key frame.
// When keyFrameInterval > 0, inter frames are produced between key frames.
//
// The returned bytes can be passed directly to pion/rtp's VP8Payloader.Payload.
func (e *Encoder) Encode(yuv []byte) ([]byte, error) {
	// Validate input buffer size against configured dimensions.
	frame, err := NewYUV420Frame(yuv, e.width, e.height)
	if err != nil {
		return nil, err
	}

	// Determine frame type
	isKeyFrame := e.shouldEncodeKeyFrame()

	mbW := (e.width + 15) / 16
	mbH := (e.height + 15) / 16

	// Get quantization factors for the current quantizer index with deltas
	qf := GetQuantFactors(e.qi, e.y1DCDelta, e.y2DCDelta, e.y2ACDelta, e.uvDCDelta, e.uvACDelta)

	// Chroma dimensions (half of luma)
	chromaW := e.width / 2
	chromaH := e.height / 2

	numMBs := mbW * mbH
	mbs := make([]macroblock, numMBs)

	if isKeyFrame || !e.refFrames.hasReference(refFrameLast) {
		// Encode as key frame (intra only)
		for mbY := 0; mbY < mbH; mbY++ {
			for mbX := 0; mbX < mbW; mbX++ {
				mbIdx := mbY*mbW + mbX

				srcY := extractLumaBlock(frame, mbX, mbY, e.width, e.height)
				srcU, srcV := extractChromaBlocks(frame, mbX, mbY, chromaW, chromaH)
				ctx := e.buildMBContext(frame, mbX, mbY, mbW, mbH)

				mbs[mbIdx] = processMacroblock(srcY, srcU, srcV, ctx, qf)
			}
		}

		// Build key frame bitstream
		result, err := BuildKeyFrame(e.width, e.height, e.qi,
			e.y1DCDelta, e.y2DCDelta, e.y2ACDelta, e.uvDCDelta, e.uvACDelta,
			e.partitionCount, mbs)
		if err != nil {
			return nil, err
		}

		// Reconstruct and store as reference frame
		e.reconstructAndStore(mbs, qf, frame)
		e.frameCount = 1 // Start counting from 1 (next frame is frame 1 after key)
		e.forceNextKeyFrame = false

		return result, nil
	}

	// Encode as inter frame (with motion estimation)
	refBuf := e.refFrames.getRef(refFrameLast)

	for mbY := 0; mbY < mbH; mbY++ {
		for mbX := 0; mbX < mbW; mbX++ {
			mbIdx := mbY*mbW + mbX

			srcY := extractLumaBlock(frame, mbX, mbY, e.width, e.height)
			srcU, srcV := extractChromaBlocks(frame, mbX, mbY, chromaW, chromaH)
			ctx := e.buildMBContext(frame, mbX, mbY, mbW, mbH)

			mbs[mbIdx] = processInterMacroblock(srcY, srcU, srcV, refBuf,
				mbX, mbY, mbW, mbs, qf, ctx)
		}
	}

	// Build inter frame bitstream
	result, err := BuildInterFrame(e.width, e.height, e.qi,
		e.y1DCDelta, e.y2DCDelta, e.y2ACDelta, e.uvDCDelta, e.uvACDelta,
		e.partitionCount, mbs)
	if err != nil {
		return nil, err
	}

	// Reconstruct and store as reference frame
	e.reconstructAndStore(mbs, qf, frame)
	e.frameCount++

	return result, nil
}

// shouldEncodeKeyFrame determines whether the next frame should be a key frame.
func (e *Encoder) shouldEncodeKeyFrame() bool {
	// Forced key frame
	if e.forceNextKeyFrame {
		return true
	}
	// I-frame only mode (backward compatible default)
	if e.keyFrameInterval == 0 {
		return true
	}
	// First frame ever (no frames encoded yet)
	if e.frameCount == 0 {
		return true
	}
	// Key frame interval reached
	if e.frameCount >= e.keyFrameInterval {
		return true
	}
	// No valid reference frame
	if !e.refFrames.hasReference(refFrameLast) {
		return true
	}
	return false
}

// reconstructAndStore reconstructs the encoded frame and stores it as a reference.
func (e *Encoder) reconstructAndStore(mbs []macroblock, qf QuantFactors, frame *Frame) {
	recon := e.refFrames.allocBuffer()
	recon.valid = true

	reconstructFrame(&recon, mbs, qf, e.refFrames, frame)

	// Apply loop filter to the reconstructed frame
	applyLoopFilter(&recon, e.loopFilter)

	// Store as last reference frame
	e.refFrames.updateLast(recon.Y, recon.Cb, recon.Cr)
}

// extractLumaBlock extracts a 16x16 luma block from the frame.
func extractLumaBlock(frame *Frame, mbX, mbY, width, height int) []byte {
	var srcY [256]byte
	for row := 0; row < 16; row++ {
		srcRow := mbY*16 + row
		if srcRow >= height {
			srcRow = height - 1
		}
		for col := 0; col < 16; col++ {
			srcCol := mbX*16 + col
			if srcCol >= width {
				srcCol = width - 1
			}
			srcY[row*16+col] = frame.Y[srcRow*width+srcCol]
		}
	}
	return srcY[:]
}

// extractChromaBlocks extracts 8x8 U and V chroma blocks from the frame.
func extractChromaBlocks(frame *Frame, mbX, mbY, chromaW, chromaH int) ([]byte, []byte) {
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
	return srcU[:], srcV[:]
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
