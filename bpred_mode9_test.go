package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredMode9 tests encoding with B_HU_PRED (mode 9).
func TestBPredMode9(t *testing.T) {
	w, h := 16, 32 // 1x2 MBs

	mbs := make([]macroblock, 2)

	// MB 0: B_PRED with mixed modes including B_HU_PRED
	mbs[0].lumaMode = B_PRED
	mbs[0].chromaMode = DC_PRED_CHROMA
	mbs[0].skip = false
	mbs[0].bModes = [16]intraBMode{
		B_VE_PRED, B_DC_PRED, B_VE_PRED, B_HU_PRED,
		B_TM_PRED, B_TM_PRED, B_TM_PRED, B_TM_PRED,
		B_TM_PRED, B_TM_PRED, B_TM_PRED, B_TM_PRED,
		B_TM_PRED, B_TM_PRED, B_TM_PRED, B_TM_PRED,
	}
	// Minimal coefficient
	mbs[0].yCoeffs[0][0] = 10

	// MB 1: V_PRED, skip=true
	mbs[1].lumaMode = V_PRED
	mbs[1].chromaMode = DC_PRED_CHROMA
	mbs[1].skip = true

	loopFilter := loopFilterParams{level: 0}

	data, _ := BuildKeyFrame(w, h, 50, 0, 0, 0, 0, 0, OnePartition, loopFilter, mbs)
	fmt.Printf("Encoded: %d bytes\n", len(data))

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))
	_, _ = dec.DecodeFrameHeader()
	_, err := dec.DecodeFrame()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
	} else {
		fmt.Printf("OK\n")
	}
}
