package vp8

import (
	"fmt"
	"testing"
)

// TestAnalyzeMBModes analyzes what modes are selected for each case.
func TestAnalyzeMBModes(t *testing.T) {
	testCases := []struct {
		width  int
		height int
	}{
		{16, 32}, // 1x2 FAIL
		{32, 32}, // 2x2 OK
		{64, 32}, // 4x2 FAIL
		{16, 48}, // 1x3 OK
	}

	for _, tc := range testCases {
		mbW := (tc.width + 15) / 16
		mbH := (tc.height + 15) / 16
		fmt.Printf("\n=== %dx%d (%dx%d MBs) ===\n", tc.width, tc.height, mbW, mbH)

		ySize := tc.width * tc.height
		cSize := (tc.width / 2) * (tc.height / 2)
		yuv := make([]byte, ySize+2*cSize)

		// Gradient
		for y := 0; y < tc.height; y++ {
			for x := 0; x < tc.width; x++ {
				yuv[y*tc.width+x] = uint8((16 + x*200/tc.width) % 256)
			}
		}
		for i := 0; i < cSize; i++ {
			yuv[ySize+i] = 128
			yuv[ySize+cSize+i] = 128
		}

		enc, _ := NewEncoder(tc.width, tc.height, 30)
		frame, _ := NewYUV420Frame(yuv, tc.width, tc.height)
		chromaW := tc.width / 2
		chromaH := tc.height / 2
		qf := GetQuantFactors(enc.qi, 0, 0, 0, 0, 0)

		mbs := make([]macroblock, mbW*mbH)
		for mbY := 0; mbY < mbH; mbY++ {
			for mbX := 0; mbX < mbW; mbX++ {
				mbIdx := mbY*mbW + mbX
				srcY := extractLumaBlock(frame, mbX, mbY, tc.width, tc.height)
				srcU, srcV := extractChromaBlocks(frame, mbX, mbY, chromaW, chromaH)
				ctx := enc.buildMBContext(frame, mbX, mbY, mbW, mbH)
				mbs[mbIdx] = processMacroblock(srcY, srcU, srcV, ctx, qf)

				mb := &mbs[mbIdx]
				fmt.Printf("MB(%d,%d): mode=%v, skip=%v\n", mbX, mbY, mb.lumaMode, mb.skip)
			}
		}
	}
}
