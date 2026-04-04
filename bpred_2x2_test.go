package vp8

import (
	"bytes"
	"fmt"
	"image"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPred2x2Grid tests exactly 2x2 MB grid (32x32 image).
func TestBPred2x2Grid(t *testing.T) {
	width, height := 32, 32 // 2x2 MB grid

	// Create a horizontal gradient
	img := image.NewYCbCr(image.Rect(0, 0, width, height), image.YCbCrSubsampleRatio420)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Y[y*img.YStride+x] = uint8(16 + x*7)
		}
	}
	for y := 0; y < height/2; y++ {
		for x := 0; x < width/2; x++ {
			img.Cb[y*img.CStride+x] = 128
			img.Cr[y*img.CStride+x] = 128
		}
	}

	// Convert to planar YUV
	ySize := width * height
	cSize := (width / 2) * (height / 2)
	yuv := make([]byte, ySize+2*cSize)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			yuv[y*width+x] = img.Y[y*img.YStride+x]
		}
	}
	for y := 0; y < height/2; y++ {
		for x := 0; x < width/2; x++ {
			yuv[ySize+y*(width/2)+x] = img.Cb[y*img.CStride+x]
			yuv[ySize+cSize+y*(width/2)+x] = img.Cr[y*img.CStride+x]
		}
	}

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	fmt.Printf("32x32 gradient: encoded %d bytes\n", len(data))

	// Try to decode
	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))

	fh, err := dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}
	fmt.Printf("FrameHeader: Key=%v, Width=%d, Height=%d\n", fh.KeyFrame, fh.Width, fh.Height)

	img2, err := dec.DecodeFrame()
	if err != nil {
		t.Errorf("DecodeFrame: %v", err)
	} else {
		t.Logf("Decoded: %v", img2.Bounds())
	}
}

// TestBPredHighDetailAllMBs tests with high detail in all 4 MBs (2x2 grid).
func TestBPredHighDetailAllMBs(t *testing.T) {
	width, height := 32, 32 // 2x2 MB grid
	ySize := width * height
	cSize := (width / 2) * (height / 2)
	yuv := make([]byte, ySize+2*cSize)

	// High detail everywhere - checkerboard pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if (x+y)%2 == 0 {
				yuv[y*width+x] = 200
			} else {
				yuv[y*width+x] = 50
			}
		}
	}
	// Neutral chroma
	for i := 0; i < cSize; i++ {
		yuv[ySize+i] = 128
		yuv[ySize+cSize+i] = 128
	}

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	fmt.Printf("32x32 checkerboard (all B_PRED): encoded %d bytes\n", len(data))

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))

	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	img, err := dec.DecodeFrame()
	if err != nil {
		t.Errorf("DecodeFrame: %v", err)
	} else {
		t.Logf("Decoded: %v", img.Bounds())
	}
}
