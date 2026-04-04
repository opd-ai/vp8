package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredExactCoeff copies exact coefficients from failing case.
func TestBPredExactCoeff(t *testing.T) {
	w, h := 16, 32
	ySize := w * h
	cSize := (w / 2) * (h / 2)
	yuv := make([]byte, ySize+2*cSize)

	// Gradient
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			yuv[y*w+x] = uint8(16 + (x%16)*14)
		}
	}
	for i := 0; i < cSize; i++ {
		yuv[ySize+i] = 128
		yuv[ySize+cSize+i] = 128
	}

	enc, _ := NewEncoder(w, h, 30)
	frame, _ := NewYUV420Frame(yuv, w, h)
	mbW := (w + 15) / 16
	mbH := (h + 15) / 16
	chromaW := w / 2
	chromaH := h / 2
	qf := GetQuantFactors(enc.qi, 0, 0, 0, 0, 0)

	// Process to get exact coefficients
	realMBs := make([]macroblock, mbW*mbH)
	for mbY := 0; mbY < mbH; mbY++ {
		for mbX := 0; mbX < mbW; mbX++ {
			mbIdx := mbY*mbW + mbX
			srcY := extractLumaBlock(frame, mbX, mbY, w, h)
			srcU, srcV := extractChromaBlocks(frame, mbX, mbY, chromaW, chromaH)
			ctx := enc.buildMBContext(frame, mbX, mbY, mbW, mbH)
			realMBs[mbIdx] = processMacroblock(srcY, srcU, srcV, ctx, qf)
		}
	}

	// Now create manual MBs with same data
	mbs := make([]macroblock, 2)

	// Copy MB 0 exactly
	mbs[0] = realMBs[0]

	// Copy MB 1 exactly
	mbs[1] = realMBs[1]

	// Print what we're encoding
	fmt.Printf("MB 0: mode=%d, skip=%v, bModes=%v\n", mbs[0].lumaMode, mbs[0].skip, mbs[0].bModes)
	fmt.Printf("MB 1: mode=%d, skip=%v\n", mbs[1].lumaMode, mbs[1].skip)

	// Build frame
	loopFilter := loopFilterParams{level: 0}

	data, err := BuildKeyFrame(w, h, enc.qi, 0, 0, 0, 0, 0, OnePartition, loopFilter, mbs)
	if err != nil {
		t.Fatalf("BuildKeyFrame: %v", err)
	}

	fmt.Printf("Encoded: %d bytes\n", len(data))

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))
	fh, _ := dec.DecodeFrameHeader()
	fmt.Printf("Header: %dx%d\n", fh.Width, fh.Height)

	_, err = dec.DecodeFrame()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
	} else {
		fmt.Printf("OK\n")
	}
}
