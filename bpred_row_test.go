package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredRowBoundary tests B_PRED with skip MBs across row boundary.
func TestBPredRowBoundary(t *testing.T) {
	width, height := 16, 32 // 1x2 MB grid

	// Create frame: top MB high detail (B_PRED), bottom MB solid (skip)
	ySize := width * height
	chromaW := width / 2
	chromaH := height / 2
	cSize := chromaW * chromaH
	yuv := make([]byte, ySize+2*cSize)

	// Top MB (0,0): high detail
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			if (row+col)%4 < 2 {
				yuv[row*width+col] = 200
			} else {
				yuv[row*width+col] = 50
			}
		}
	}
	// Bottom MB (0,1): solid
	for row := 16; row < 32; row++ {
		for col := 0; col < 16; col++ {
			yuv[row*width+col] = 128
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

	fmt.Printf("Encoded %d bytes\n", len(data))

	// Try to decode
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
