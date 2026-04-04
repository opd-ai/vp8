package vp8

import (
	"bytes"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredForcedSingleMB tests B_PRED mode by directly creating an MB with B_PRED.
// This test verifies that the B_PRED bitstream encoding is correct.
func TestBPredForcedSingleMB(t *testing.T) {
	width, height := 16, 16

	// Create high-detail frame
	yuv := makeHighDetailYUV(width, height)

	// Build frame
	frame, err := NewYUV420Frame(yuv, width, height)
	if err != nil {
		t.Fatalf("NewYUV420Frame: %v", err)
	}

	// Get quant factors
	qi := 24
	qf := GetQuantFactorsSimple(qi)

	// Extract sources
	srcY := make([]byte, 256)
	srcU := make([]byte, 64)
	srcV := make([]byte, 64)
	copy(srcY, frame.Y[:256])
	copy(srcU, frame.Cb[:64])
	copy(srcV, frame.Cr[:64])

	// Build context (first MB, no neighbors)
	ctx := &mbContext{}

	// Create MB with forced B_PRED selection
	mb := macroblock{skip: true}

	// Force B_PRED mode
	_, bModes := evaluateBPredMode(srcY, ctx)
	mb.lumaMode = B_PRED
	mb.bModes = bModes

	// Process Y blocks in B_PRED mode
	processYBlocksBPred(srcY, ctx, &mb, qf)

	// Select and process chroma
	bestChromaMode, _ := SelectBest8x8ChromaMode(srcU, ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU)
	mb.chromaMode = bestChromaMode
	processChromaPlane(srcU, ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU, mb.chromaMode, mb.uCoeffs[:], &mb.skip, qf)
	processChromaPlane(srcV, ctx.chromaAboveV, ctx.chromaLeftV, ctx.chromaTopLeftV, mb.chromaMode, mb.vCoeffs[:], &mb.skip, qf)

	t.Logf("MB luma mode: %v", mb.lumaMode)
	t.Logf("MB skip: %v", mb.skip)
	t.Logf("B_PRED modes: %v", mb.bModes)

	// Now encode the full frame using the standard API
	mbs := []macroblock{mb}
	data, err := BuildKeyFrame(width, height, qi, 0, 0, 0, 0, 0, OnePartition, loopFilterParams{}, mbs)
	if err != nil {
		t.Fatalf("BuildKeyFrame: %v", err)
	}
	t.Logf("Encoded %d bytes", len(data))

	// Try to decode
	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))

	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	img, err := dec.DecodeFrame()
	if err != nil {
		t.Errorf("DecodeFrame with B_PRED: %v", err)
	} else {
		t.Logf("Decoded: %v", img.Bounds())
	}
}

func makeHighDetailYUV(width, height int) []byte {
	ySize := width * height
	chromaW := width / 2
	chromaH := height / 2
	cSize := chromaW * chromaH

	yuv := make([]byte, ySize+2*cSize)
	y := yuv[:ySize]

	// Create a checkerboard pattern (high detail)
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			if (row+col)%2 == 0 {
				y[row*width+col] = 200
			} else {
				y[row*width+col] = 50
			}
		}
	}

	// Neutral chroma
	for i := 0; i < cSize; i++ {
		yuv[ySize+i] = 128
		yuv[ySize+cSize+i] = 128
	}

	return yuv
}

// TestBPredForcedMultipleMBs tests B_PRED with multiple macroblocks.
func TestBPredForcedMultipleMBs(t *testing.T) {
	width, height := 32, 32

	// Create high-detail frame
	yuv := makeHighDetailYUV(width, height)

	// Build frame
	frame, err := NewYUV420Frame(yuv, width, height)
	if err != nil {
		t.Fatalf("NewYUV420Frame: %v", err)
	}

	// Get quant factors
	qi := 24
	qf := GetQuantFactorsSimple(qi)
	chromaW := width / 2

	mbCols := width / 16
	mbRows := height / 16
	mbs := make([]macroblock, mbCols*mbRows)

	for mbY := 0; mbY < mbRows; mbY++ {
		for mbX := 0; mbX < mbCols; mbX++ {
			mbIdx := mbY*mbCols + mbX

			// Extract sources
			srcY := make([]byte, 256)
			srcU := make([]byte, 64)
			srcV := make([]byte, 64)

			for row := 0; row < 16; row++ {
				srcRow := mbY*16 + row
				srcCol := mbX * 16
				copy(srcY[row*16:row*16+16], frame.Y[srcRow*width+srcCol:srcRow*width+srcCol+16])
			}
			for row := 0; row < 8; row++ {
				srcRow := mbY*8 + row
				srcCol := mbX * 8
				copy(srcU[row*8:row*8+8], frame.Cb[srcRow*chromaW+srcCol:srcRow*chromaW+srcCol+8])
				copy(srcV[row*8:row*8+8], frame.Cr[srcRow*chromaW+srcCol:srcRow*chromaW+srcCol+8])
			}

			// Build context
			ctx := buildMBContextForTest(frame, mbX, mbY, width, height, chromaW)

			// Create MB with forced B_PRED
			mb := macroblock{skip: true}
			_, bModes := evaluateBPredMode(srcY, ctx)
			mb.lumaMode = B_PRED
			mb.bModes = bModes

			processYBlocksBPred(srcY, ctx, &mb, qf)

			bestChromaMode, _ := SelectBest8x8ChromaMode(srcU, ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU)
			mb.chromaMode = bestChromaMode
			processChromaPlane(srcU, ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU, mb.chromaMode, mb.uCoeffs[:], &mb.skip, qf)
			processChromaPlane(srcV, ctx.chromaAboveV, ctx.chromaLeftV, ctx.chromaTopLeftV, mb.chromaMode, mb.vCoeffs[:], &mb.skip, qf)

			mbs[mbIdx] = mb
		}
	}

	t.Logf("Built %d MBs with B_PRED", len(mbs))

	data, err := BuildKeyFrame(width, height, qi, 0, 0, 0, 0, 0, OnePartition, loopFilterParams{}, mbs)
	if err != nil {
		t.Fatalf("BuildKeyFrame: %v", err)
	}
	t.Logf("Encoded %d bytes", len(data))

	// Try to decode
	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))

	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	img, err := dec.DecodeFrame()
	if err != nil {
		t.Errorf("DecodeFrame with B_PRED (multi-MB): %v", err)
	} else {
		t.Logf("Decoded: %v", img.Bounds())
	}
}

func buildMBContextForTest(frame *Frame, mbX, mbY, width, height, chromaW int) *mbContext {
	ctx := &mbContext{}
	chromaH := height / 2

	// Build luma context
	if mbY > 0 {
		aboveRow := mbY*16 - 1
		for i := 0; i < 16; i++ {
			col := mbX*16 + i
			if col < width {
				ctx.lumaAboveBuf[i] = frame.Y[aboveRow*width+col]
			}
		}
		ctx.lumaAbove = ctx.lumaAboveBuf[:]
	}

	if mbX > 0 {
		leftCol := mbX*16 - 1
		for i := 0; i < 16; i++ {
			row := mbY*16 + i
			if row < height {
				ctx.lumaLeftBuf[i] = frame.Y[row*width+leftCol]
			}
		}
		ctx.lumaLeft = ctx.lumaLeftBuf[:]
	}

	if mbX > 0 && mbY > 0 {
		ctx.lumaTopLeft = frame.Y[(mbY*16-1)*width+(mbX*16-1)]
	} else {
		ctx.lumaTopLeft = 128
	}

	// Build chroma context
	if mbY > 0 {
		aboveRow := mbY*8 - 1
		for i := 0; i < 8; i++ {
			col := mbX*8 + i
			if col < chromaW {
				ctx.chromaAboveUBuf[i] = frame.Cb[aboveRow*chromaW+col]
				ctx.chromaAboveVBuf[i] = frame.Cr[aboveRow*chromaW+col]
			}
		}
		ctx.chromaAboveU = ctx.chromaAboveUBuf[:]
		ctx.chromaAboveV = ctx.chromaAboveVBuf[:]
	}

	if mbX > 0 {
		leftCol := mbX*8 - 1
		for i := 0; i < 8; i++ {
			row := mbY*8 + i
			if row < chromaH {
				ctx.chromaLeftUBuf[i] = frame.Cb[row*chromaW+leftCol]
				ctx.chromaLeftVBuf[i] = frame.Cr[row*chromaW+leftCol]
			}
		}
		ctx.chromaLeftU = ctx.chromaLeftUBuf[:]
		ctx.chromaLeftV = ctx.chromaLeftVBuf[:]
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
