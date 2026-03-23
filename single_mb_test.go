package vp8

import (
	"bytes"
	"testing"

	"golang.org/x/image/vp8"
)

func TestSingleMBWhite(t *testing.T) {
	// Create 16x16 image (single MB) with Y=255
	width, height := 16, 16
	yuv := make([]byte, width*height+2*(width/2)*(height/2))

	// Y plane: all 255
	for i := 0; i < width*height; i++ {
		yuv[i] = 255
	}
	// U/V: 128
	for i := width * height; i < len(yuv); i++ {
		yuv[i] = 128
	}

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	vp8Data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	t.Logf("Encoded %d bytes: %x", len(vp8Data), vp8Data)

	// Decode
	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))
	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	decoded, err := dec.DecodeFrame()
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	// Check center pixel
	y := decoded.Y[8*decoded.YStride+8]
	t.Logf("Center Y: %d (expected 255)", y)

	// Check all pixels
	for r := 0; r < 16; r++ {
		for c := 0; c < 16; c++ {
			got := decoded.Y[r*decoded.YStride+c]
			if got != 255 {
				t.Logf("Y[%d,%d] = %d (expected 255)", c, r, got)
			}
		}
	}
}

func TestSingleMBBlack(t *testing.T) {
	// Create 16x16 image (single MB) with Y=0
	width, height := 16, 16
	yuv := make([]byte, width*height+2*(width/2)*(height/2))

	// Y plane: all 0
	for i := 0; i < width*height; i++ {
		yuv[i] = 0
	}
	// U/V: 128
	for i := width * height; i < len(yuv); i++ {
		yuv[i] = 128
	}

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	vp8Data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	t.Logf("Encoded %d bytes: %x", len(vp8Data), vp8Data)

	// Decode
	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))
	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	decoded, err := dec.DecodeFrame()
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	// Check center pixel
	y := decoded.Y[8*decoded.YStride+8]
	t.Logf("Center Y: %d (expected 0)", y)
}
