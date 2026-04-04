package vp8

import (
	"bytes"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredFollowedBySkip tests B_PRED MB followed by skip 16x16 MB.
func TestBPredFollowedBySkip(t *testing.T) {
	width, height := 32, 16 // 2x1 MB grid

	// Create frame: left MB has high detail (B_PRED), right MB is solid (skip)
	ySize := width * height
	chromaW := width / 2
	chromaH := height / 2
	cSize := chromaW * chromaH
	yuv := make([]byte, ySize+2*cSize)

	// Left MB (0,0): high detail
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			if (row+col)%4 < 2 {
				yuv[row*width+col] = 200
			} else {
				yuv[row*width+col] = 50
			}
		}
	}
	// Right MB (1,0): solid
	for row := 0; row < 16; row++ {
		for col := 16; col < 32; col++ {
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

	t.Logf("Encoded %d bytes", len(data))

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

// TestSkipFollowedByBPred tests skip 16x16 MB followed by B_PRED MB.
func TestSkipFollowedByBPred(t *testing.T) {
	width, height := 32, 16 // 2x1 MB grid

	// Create frame: left MB is solid (skip), right MB has high detail (B_PRED)
	ySize := width * height
	chromaW := width / 2
	chromaH := height / 2
	cSize := chromaW * chromaH
	yuv := make([]byte, ySize+2*cSize)

	// Left MB (0,0): solid
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			yuv[row*width+col] = 128
		}
	}
	// Right MB (1,0): high detail
	for row := 0; row < 16; row++ {
		for col := 16; col < 32; col++ {
			if (row+col)%4 < 2 {
				yuv[row*width+col] = 200
			} else {
				yuv[row*width+col] = 50
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

	t.Logf("Encoded %d bytes", len(data))

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

// TestBPredFollowedByNonSkip tests B_PRED MB followed by non-skip 16x16 MB.
func TestBPredFollowedByNonSkip(t *testing.T) {
	width, height := 32, 16 // 2x1 MB grid

	// Create frame: left MB has high detail (B_PRED), right MB has different value (non-skip)
	ySize := width * height
	chromaW := width / 2
	chromaH := height / 2
	cSize := chromaW * chromaH
	yuv := make([]byte, ySize+2*cSize)

	// Left MB (0,0): high detail
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			if (row+col)%4 < 2 {
				yuv[row*width+col] = 200
			} else {
				yuv[row*width+col] = 50
			}
		}
	}
	// Right MB (1,0): different solid (not matching prediction so non-skip)
	for row := 0; row < 16; row++ {
		for col := 16; col < 32; col++ {
			yuv[row*width+col] = 200 // Different from DC pred
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

	t.Logf("Encoded %d bytes", len(data))

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
