package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredModeByMode tests each B_PRED mode.
func TestBPredModeByMode(t *testing.T) {
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
		w, h := 16, 32
		mbs := make([]macroblock, 2)

		mbs[0].lumaMode = B_PRED
		mbs[0].chromaMode = DC_PRED_CHROMA
		mbs[0].skip = false
		for i := 0; i < 16; i++ {
			mbs[0].bModes[i] = m.mode
		}
		mbs[0].yCoeffs[0][0] = 10

		mbs[1].lumaMode = V_PRED
		mbs[1].chromaMode = DC_PRED_CHROMA
		mbs[1].skip = true

		loopFilter := loopFilterParams{level: 0}
		data, _ := BuildKeyFrame(w, h, 50, 0, 0, 0, 0, 0, OnePartition, loopFilter, mbs)

		dec := vp8.NewDecoder()
		dec.Init(bytes.NewReader(data), len(data))
		_, _ = dec.DecodeFrameHeader()
		_, err := dec.DecodeFrame()

		if err != nil {
			fmt.Printf("%s (value=%d): FAIL (%d bytes)\n", m.name, m.mode, len(data))
		} else {
			fmt.Printf("%s (value=%d): OK (%d bytes)\n", m.name, m.mode, len(data))
		}
	}
}
