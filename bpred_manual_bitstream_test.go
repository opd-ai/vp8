package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredManualBitstream manually encodes a B_PRED + V_PRED frame.
func TestBPredManualBitstream(t *testing.T) {
	w, h := 16, 32 // 1x2 MBs

	// Create minimal macroblocks
	mbs := make([]macroblock, 2)

	// MB 0: B_PRED, all B_DC_PRED sub-blocks, skip=false
	mbs[0].lumaMode = B_PRED
	mbs[0].chromaMode = DC_PRED_CHROMA
	mbs[0].skip = false
	for i := 0; i < 16; i++ {
		mbs[0].bModes[i] = B_DC_PRED
	}
	// Add some minimal coefficients
	mbs[0].yCoeffs[0][0] = 10

	// MB 1: V_PRED, skip=true
	mbs[1].lumaMode = V_PRED
	mbs[1].chromaMode = DC_PRED_CHROMA
	mbs[1].skip = true

	// Build frame
	loopFilter := loopFilterParams{level: 0}

	data, err := BuildKeyFrame(w, h, 50, 0, 0, 0, 0, 0, OnePartition, loopFilter, mbs)
	if err != nil {
		t.Fatalf("BuildKeyFrame: %v", err)
	}

	fmt.Printf("Encoded: %d bytes\n", len(data))
	fmt.Printf("Hex: ")
	for _, b := range data {
		fmt.Printf("%02x ", b)
	}
	fmt.Println()

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))

	fh, err := dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}
	fmt.Printf("Frame header: %dx%d, keyframe=%v\n", fh.Width, fh.Height, fh.KeyFrame)

	img, err := dec.DecodeFrame()
	if err != nil {
		t.Errorf("DecodeFrame: %v", err)
	} else {
		t.Logf("Decoded: %v", img.Bounds())
	}
}
