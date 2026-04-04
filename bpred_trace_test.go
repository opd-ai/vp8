package vp8

import (
	"bytes"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredAllSameModes tests all MBs with B_PRED mode (no mixing).
func TestBPredAllSameModes(t *testing.T) {
	width, height := 32, 32

	// Create a checkerboard frame that triggers B_PRED for all MBs
	yuv := makeAllBPredYUV(width, height)

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

// TestBPredAllNonBPredModes tests all MBs with non-B_PRED mode.
func TestBPredAllNonBPredModes(t *testing.T) {
	width, height := 32, 32

	// Create a solid gray frame (no detail, won't trigger B_PRED)
	yuv := makeSolidYUV(width, height, 128)

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

func makeAllBPredYUV(width, height int) []byte {
	ySize := width * height
	chromaW := width / 2
	chromaH := height / 2
	cSize := chromaW * chromaH

	yuv := make([]byte, ySize+2*cSize)
	y := yuv[:ySize]

	// Create a checkerboard pattern that triggers B_PRED for all MBs
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			// Local checkerboard within each 4x4 block
			if (row+col)%4 < 2 {
				y[row*width+col] = 200
			} else {
				y[row*width+col] = 50
			}
		}
	}

	// Neutral chroma
	for i := 0; i < cSize; i++ {
		yuv[ySize+i] = 128
		yuv[ySize+cSize+i] = 128
	}

	return yuv
}

func makeSolidYUV(width, height int, value byte) []byte {
	ySize := width * height
	chromaW := width / 2
	chromaH := height / 2
	cSize := chromaW * chromaH

	yuv := make([]byte, ySize+2*cSize)

	for i := 0; i < ySize; i++ {
		yuv[i] = value
	}
	for i := 0; i < cSize; i++ {
		yuv[ySize+i] = 128
		yuv[ySize+cSize+i] = 128
	}

	return yuv
}

// TestBPredMixedPattern tests a specific mix of B_PRED and 16x16 modes.
func TestBPredMixedPattern(t *testing.T) {
	width, height := 32, 32

	// Create a frame where left MBs trigger B_PRED, right MBs use 16x16
	yuv := makeMixedModeYUV(width, height)

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

func makeMixedModeYUV(width, height int) []byte {
	ySize := width * height
	chromaW := width / 2
	chromaH := height / 2
	cSize := chromaW * chromaH

	yuv := make([]byte, ySize+2*cSize)
	y := yuv[:ySize]

	// Left half: high-detail pattern (should trigger B_PRED)
	// Right half: solid color (should use DC_PRED)
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			if col < width/2 {
				// High detail for left MBs
				if (row+col)%4 < 2 {
					y[row*width+col] = 200
				} else {
					y[row*width+col] = 50
				}
			} else {
				// Solid for right MBs
				y[row*width+col] = 128
			}
		}
	}

	// Neutral chroma
	for i := 0; i < cSize; i++ {
		yuv[ySize+i] = 128
		yuv[ySize+cSize+i] = 128
	}

	return yuv
}

// TestBPredExactGradient tests the exact gradient pattern that fails.
func TestBPredExactGradient(t *testing.T) {
	width, height := 32, 32

	// Exactly replicate makeGradientYUV420
	ySize := width * height
	uvSize := (width / 2) * (height / 2)
	yuv := make([]byte, ySize+2*uvSize)
	// Luma: horizontal gradient 16-235 (studio swing)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := 16 + int(float64(x)/float64(width-1)*219)
			yuv[y*width+x] = byte(val)
		}
	}
	// Chroma: neutral gray (128)
	for i := ySize; i < len(yuv); i++ {
		yuv[i] = 128
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
