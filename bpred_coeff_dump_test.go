package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredCoeffDump dumps coefficients for failing case.
func TestBPredCoeffDump(t *testing.T) {
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

	mbs := make([]macroblock, mbW*mbH)
	for mbY := 0; mbY < mbH; mbY++ {
		for mbX := 0; mbX < mbW; mbX++ {
			mbIdx := mbY*mbW + mbX
			srcY := extractLumaBlock(frame, mbX, mbY, w, h)
			srcU, srcV := extractChromaBlocks(frame, mbX, mbY, chromaW, chromaH)
			ctx := enc.buildMBContext(frame, mbX, mbY, mbW, mbH)
			mbs[mbIdx] = processMacroblock(srcY, srcU, srcV, ctx, qf)

			mb := &mbs[mbIdx]
			fmt.Printf("MB(%d,%d): mode=%d, skip=%v\n", mbX, mbY, mb.lumaMode, mb.skip)

			// Count non-zero coefficients
			yNonZero := 0
			for i := 0; i < 16; i++ {
				for j := 0; j < 16; j++ {
					if mb.yCoeffs[i][j] != 0 {
						yNonZero++
					}
				}
			}
			fmt.Printf("  Y non-zero: %d\n", yNonZero)

			// Show first few Y coefficients
			if yNonZero > 0 {
				fmt.Printf("  First Y block: %v\n", mb.yCoeffs[0][:8])
			}
		}
	}

	// Build and test
	loopFilter := loopFilterParams{level: 0}
	data, _ := BuildKeyFrame(w, h, enc.qi, 0, 0, 0, 0, 0, OnePartition, loopFilter, mbs)

	fmt.Printf("\nEncoded: %d bytes\n", len(data))

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
