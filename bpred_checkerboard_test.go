package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredCheckerboard tests with checkerboard pattern.
func TestBPredCheckerboard(t *testing.T) {
	w, h := 16, 32
	ySize := w * h
	cSize := (w / 2) * (h / 2)
	yuv := make([]byte, ySize+2*cSize)

	// Top MB: checkerboard
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			if (row+col)%4 < 2 {
				yuv[row*w+col] = 200
			} else {
				yuv[row*w+col] = 50
			}
		}
	}
	// Bottom MB: solid
	for row := 16; row < 32; row++ {
		for col := 0; col < 16; col++ {
			yuv[row*w+col] = 128
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
			fmt.Printf("MB(%d,%d): lumaMode=%d, chromaMode=%d, skip=%v\n",
				mbX, mbY, mb.lumaMode, mb.chromaMode, mb.skip)
		}
	}

	data, _ := enc.Encode(yuv)
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
