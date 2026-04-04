package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredSkipVPred tests B_PRED followed by skip V_PRED.
func TestBPredSkipVPred(t *testing.T) {
	// Create 16x32 where second MB is V_PRED skip
	w, h := 16, 32
	ySize := w * h
	cSize := (w / 2) * (h / 2)
	yuv := make([]byte, ySize+2*cSize)

	// First MB: gradient (will select B_PRED)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			yuv[y*w+x] = uint8(16 + x*14)
		}
	}
	// Second MB: copy from first MB (same gradient)
	for y := 16; y < 32; y++ {
		for x := 0; x < 16; x++ {
			yuv[y*w+x] = uint8(16 + x*14) // same gradient
		}
	}
	for i := 0; i < cSize; i++ {
		yuv[ySize+i] = 128
		yuv[ySize+cSize+i] = 128
	}

	enc, _ := NewEncoder(w, h, 30)
	data, _ := enc.Encode(yuv)

	fmt.Printf("Case 1 - gradient in both rows: %d bytes\n", len(data))

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))
	_, _ = dec.DecodeFrameHeader()
	_, err := dec.DecodeFrame()
	if err != nil {
		fmt.Printf("  FAIL: %v\n", err)
	} else {
		fmt.Printf("  OK\n")
	}

	// Now try: first MB gradient, second MB solid
	for y := 16; y < 32; y++ {
		for x := 0; x < 16; x++ {
			yuv[y*w+x] = 128 // solid
		}
	}

	enc2, _ := NewEncoder(w, h, 30)
	data2, _ := enc2.Encode(yuv)

	fmt.Printf("Case 2 - gradient then solid: %d bytes\n", len(data2))

	dec2 := vp8.NewDecoder()
	dec2.Init(bytes.NewReader(data2), len(data2))
	_, _ = dec2.DecodeFrameHeader()
	_, err = dec2.DecodeFrame()
	if err != nil {
		fmt.Printf("  FAIL: %v\n", err)
	} else {
		fmt.Printf("  OK\n")
	}
}
