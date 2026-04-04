package vp8

import (
	"fmt"
	"testing"
)

// TestAnalyzeGradientModes analyzes mode selection for the gradient pattern.
func TestAnalyzeGradientModes(t *testing.T) {
	width, height := 32, 32

	// Create gradient
	ySize := width * height
	uvSize := (width / 2) * (height / 2)
	yuv := make([]byte, ySize+2*uvSize)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := 16 + int(float64(x)/float64(width-1)*219)
			yuv[y*width+x] = byte(val)
		}
	}
	for i := ySize; i < len(yuv); i++ {
		yuv[i] = 128
	}

	// Build frame
	frame, err := NewYUV420Frame(yuv, width, height)
	if err != nil {
		t.Fatalf("NewYUV420Frame: %v", err)
	}

	qi := 24
	qf := GetQuantFactorsSimple(qi)
	_ = qf // used later
	chromaW := width / 2

	mbCols := width / 16
	mbRows := height / 16

	fmt.Println("\n=== Mode Selection Analysis ===")
	for mbY := 0; mbY < mbRows; mbY++ {
		for mbX := 0; mbX < mbCols; mbX++ {
			// Extract sources
			srcY := make([]byte, 256)
			for row := 0; row < 16; row++ {
				srcRow := mbY*16 + row
				srcCol := mbX * 16
				copy(srcY[row*16:row*16+16], frame.Y[srcRow*width+srcCol:srcRow*width+srcCol+16])
			}

			// Build context
			ctx := buildMBContextForTest(frame, mbX, mbY, width, height, chromaW)

			// Evaluate both modes
			best16x16Mode, best16x16SAD := SelectBest16x16Mode(srcY, ctx.lumaAbove, ctx.lumaLeft, ctx.lumaTopLeft)
			bpredSAD, bModes := evaluateBPredMode(srcY, ctx)

			selectBPred := bpredSAD*100 < best16x16SAD*bPredSADThreshold

			fmt.Printf("MB(%d,%d): 16x16_SAD=%d mode=%d, B_PRED_SAD=%d\n",
				mbX, mbY, best16x16SAD, best16x16Mode, bpredSAD)
			fmt.Printf("  Check: %d*100=%d < %d*%d=%d -> %v\n",
				bpredSAD, bpredSAD*100, best16x16SAD, bPredSADThreshold, best16x16SAD*bPredSADThreshold, selectBPred)

			if selectBPred {
				fmt.Printf("  -> B_PRED selected, modes=%v\n", bModes)
			} else {
				fmt.Printf("  -> 16x16 mode %d selected\n", best16x16Mode)
			}
		}
	}
	fmt.Println()
}
