package vp8

import (
	"fmt"
	"testing"
)

// TestBPredModeTrace traces mode selection for each MB.
func TestBPredModeTrace(t *testing.T) {
	for _, tc := range []struct {
		w, h int
	}{
		{32, 16}, // 2x1 - OK
		{16, 32}, // 1x2 - FAIL
	} {
		fmt.Printf("\n=== %dx%d (%dx%d MBs) ===\n", tc.w, tc.h, (tc.w+15)/16, (tc.h+15)/16)

		ySize := tc.w * tc.h
		cSize := (tc.w / 2) * (tc.h / 2)
		yuv := make([]byte, ySize+2*cSize)

		for y := 0; y < tc.h; y++ {
			for x := 0; x < tc.w; x++ {
				yuv[y*tc.w+x] = uint8(16 + (x%16)*14)
			}
		}
		for i := 0; i < cSize; i++ {
			yuv[ySize+i] = 128
			yuv[ySize+cSize+i] = 128
		}

		enc, _ := NewEncoder(tc.w, tc.h, 30)
		frame, _ := NewYUV420Frame(yuv, tc.w, tc.h)
		mbW := (tc.w + 15) / 16
		mbH := (tc.h + 15) / 16
		chromaW := tc.w / 2
		chromaH := tc.h / 2
		qf := GetQuantFactors(enc.qi, 0, 0, 0, 0, 0)

		mbs := make([]macroblock, mbW*mbH)
		for mbY := 0; mbY < mbH; mbY++ {
			for mbX := 0; mbX < mbW; mbX++ {
				mbIdx := mbY*mbW + mbX
				srcY := extractLumaBlock(frame, mbX, mbY, tc.w, tc.h)
				srcU, srcV := extractChromaBlocks(frame, mbX, mbY, chromaW, chromaH)
				ctx := enc.buildMBContext(frame, mbX, mbY, mbW, mbH)
				mbs[mbIdx] = processMacroblock(srcY, srcU, srcV, ctx, qf)

				mb := &mbs[mbIdx]
				fmt.Printf("MB(%d,%d): lumaMode=%d (%v), chromaMode=%d, skip=%v\n",
					mbX, mbY, mb.lumaMode, mb.lumaMode, mb.chromaMode, mb.skip)
				if mb.lumaMode == B_PRED {
					fmt.Printf("  bModes: %v\n", mb.bModes)
				}
			}
		}
	}
}
