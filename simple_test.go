package vp8

import (
	"bytes"
	"fmt"
	"testing"

	xvp8 "golang.org/x/image/vp8"
)

func TestSimpleGradient(t *testing.T) {
	width, height := 32, 16

	ySize := width * height
	uvSize := ((width + 1) / 2) * ((height + 1) / 2)
	yuv := make([]byte, ySize+2*uvSize)

	// Simple gradient: left=0, right=255
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Each macroblock has a solid color
			if x < 16 {
				yuv[y*width+x] = 0
			} else {
				yuv[y*width+x] = 255
			}
		}
	}
	for i := ySize; i < ySize+2*uvSize; i++ {
		yuv[i] = 128
	}

	enc, _ := NewEncoder(width, height, 30)
	frame, _ := enc.Encode(yuv)

	fmt.Printf("Frame: %x\n", frame)

	// Decode with x/image/vp8
	dec := xvp8.NewDecoder()
	dec.Init(bytes.NewReader(frame), len(frame))

	fh, err := dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}
	fmt.Printf("FrameHeader: KeyFrame=%v, Width=%d, Height=%d\n",
		fh.KeyFrame, fh.Width, fh.Height)

	img, err := dec.DecodeFrame()
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	bounds := img.Bounds()
	fmt.Printf("Image bounds: %v\n", bounds)

	// Get Y plane
	for y := 0; y < height; y++ {
		fmt.Printf("Row %d: ", y)
		for x := 0; x < width; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			yVal := r >> 8
			fmt.Printf("%3d ", yVal)
		}
		fmt.Println()
	}

	// Check specific pixels
	r0, _, _, _ := img.At(0, 0).RGBA()
	r1, _, _, _ := img.At(16, 0).RGBA()
	fmt.Printf("\nMB0 pixel [0,0]: source=%d, decoded=%d\n", yuv[0], r0>>8)
	fmt.Printf("MB1 pixel [16,0]: source=%d, decoded=%d\n", yuv[16], r1>>8)
}
