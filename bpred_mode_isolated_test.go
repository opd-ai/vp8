package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredModeIsolated tests B_PRED mode with a single MB (no second MB).
func TestBPredModeIsolated(t *testing.T) {
	modes := []struct {
		name string
		mode intraBMode
	}{
		{"B_DC_PRED", B_DC_PRED},
		{"B_TM_PRED", B_TM_PRED},
		{"B_VE_PRED", B_VE_PRED},
		{"B_HE_PRED", B_HE_PRED},
		{"B_RD_PRED", B_RD_PRED},
		{"B_VR_PRED", B_VR_PRED},
		{"B_LD_PRED", B_LD_PRED},
		{"B_VL_PRED", B_VL_PRED},
		{"B_HD_PRED", B_HD_PRED},
		{"B_HU_PRED", B_HU_PRED},
	}

	for _, m := range modes {
		// Single MB (16x16)
		w, h := 16, 16
		mbs := make([]macroblock, 1)

		mbs[0].lumaMode = B_PRED
		mbs[0].chromaMode = DC_PRED_CHROMA
		mbs[0].skip = false
		for i := 0; i < 16; i++ {
			mbs[0].bModes[i] = m.mode
		}
		mbs[0].yCoeffs[0][0] = 10

		loopFilter := loopFilterParams{level: 0}
		data, _ := BuildKeyFrame(w, h, 50, 0, 0, 0, 0, 0, OnePartition, loopFilter, mbs)

		dec := vp8.NewDecoder()
		dec.Init(bytes.NewReader(data), len(data))
		_, _ = dec.DecodeFrameHeader()
		_, err := dec.DecodeFrame()

		if err != nil {
			fmt.Printf("%s: FAIL - %v\n", m.name, err)
		} else {
			fmt.Printf("%s: OK\n", m.name)
		}
	}
}
