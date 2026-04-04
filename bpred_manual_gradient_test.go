package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredManualGradient manually creates MBs matching the gradient case.
func TestBPredManualGradient(t *testing.T) {
	w, h := 16, 32 // 1x2 MBs

	mbs := make([]macroblock, 2)

	// MB 0: B_PRED with some coefficients
	mbs[0].lumaMode = B_PRED
	mbs[0].chromaMode = DC_PRED_CHROMA
	mbs[0].skip = false
	for i := 0; i < 16; i++ {
		mbs[0].bModes[i] = B_TM_PRED // Use TM_PRED as we saw in gradient
	}
	// Add 14 non-zero coefficients spread across blocks
	mbs[0].yCoeffs[0][0] = -31
	mbs[0].yCoeffs[0][1] = -4
	mbs[0].yCoeffs[1][0] = 10
	mbs[0].yCoeffs[2][0] = 5
	mbs[0].yCoeffs[3][0] = 8
	mbs[0].yCoeffs[4][0] = 3
	mbs[0].yCoeffs[5][0] = 7
	mbs[0].yCoeffs[6][0] = 2
	mbs[0].yCoeffs[7][0] = 6
	mbs[0].yCoeffs[8][0] = 4
	mbs[0].yCoeffs[9][0] = 9
	mbs[0].yCoeffs[10][0] = 1
	mbs[0].yCoeffs[11][0] = 5
	mbs[0].yCoeffs[12][0] = 3

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

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))
	_, _ = dec.DecodeFrameHeader()
	_, err = dec.DecodeFrame()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
	} else {
		fmt.Printf("OK\n")
	}
}
