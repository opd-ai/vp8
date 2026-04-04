package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredCombo tests B_PRED + 16x16 mode combinations.
func TestBPredCombo(t *testing.T) {
	bModes := []struct {
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

	// Test with V_PRED in second MB
	fmt.Println("=== B_PRED + V_PRED ===")
	for _, m := range bModes {
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
			fmt.Printf("%s: FAIL\n", m.name)
		} else {
			fmt.Printf("%s: OK\n", m.name)
		}
	}

	// Test with B_PRED in second MB
	fmt.Println("\n=== B_PRED + B_PRED ===")
	for _, m := range bModes {
		w, h := 16, 32
		mbs := make([]macroblock, 2)

		mbs[0].lumaMode = B_PRED
		mbs[0].chromaMode = DC_PRED_CHROMA
		mbs[0].skip = false
		for i := 0; i < 16; i++ {
			mbs[0].bModes[i] = m.mode
		}
		mbs[0].yCoeffs[0][0] = 10

		mbs[1].lumaMode = B_PRED
		mbs[1].chromaMode = DC_PRED_CHROMA
		mbs[1].skip = true
		for i := 0; i < 16; i++ {
			mbs[1].bModes[i] = B_DC_PRED
		}

		loopFilter := loopFilterParams{level: 0}
		data, _ := BuildKeyFrame(w, h, 50, 0, 0, 0, 0, 0, OnePartition, loopFilter, mbs)

		dec := vp8.NewDecoder()
		dec.Init(bytes.NewReader(data), len(data))
		_, _ = dec.DecodeFrameHeader()
		_, err := dec.DecodeFrame()

		if err != nil {
			fmt.Printf("%s: FAIL\n", m.name)
		} else {
			fmt.Printf("%s: OK\n", m.name)
		}
	}
}
