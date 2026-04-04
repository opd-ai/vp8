package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredDecodeDebug examines what the decoder is trying to read.
func TestBPredDecodeDebug(t *testing.T) {
	// Test 1: Minimal 16x32 with very uniform content in second row
	// First row: gradient (B_PRED), second row: totally flat (skip with no residual)
	width, height := 16, 32
	ySize := width * height
	cSize := (width / 2) * (height / 2)
	yuv := make([]byte, ySize+2*cSize)

	// First row: gradient
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			yuv[y*width+x] = uint8(16 + x*14)
		}
	}
	// Second row: flat 128
	for y := 16; y < 32; y++ {
		for x := 0; x < 16; x++ {
			yuv[y*width+x] = 128
		}
	}
	// Chroma flat
	for i := 0; i < cSize; i++ {
		yuv[ySize+i] = 128
		yuv[ySize+cSize+i] = 128
	}

	enc, _ := NewEncoder(width, height, 30)
	data, _ := enc.Encode(yuv)

	fmt.Printf("16x32 (gradient row0, flat row1): %d bytes\n", len(data))

	// Hex dump first 60 bytes
	fmt.Printf("Hex: ")
	for i := 0; i < len(data) && i < 60; i++ {
		fmt.Printf("%02x ", data[i])
	}
	fmt.Printf("\n")

	// Try with modified content: second row also gradient (so no skip)
	for y := 16; y < 32; y++ {
		for x := 0; x < 16; x++ {
			yuv[y*width+x] = uint8(16 + x*14) // same gradient
		}
	}

	enc2, _ := NewEncoder(width, height, 30)
	data2, _ := enc2.Encode(yuv)

	fmt.Printf("16x32 (gradient both rows): %d bytes\n", len(data2))

	dec2 := vp8.NewDecoder()
	dec2.Init(bytes.NewReader(data2), len(data2))
	_, _ = dec2.DecodeFrameHeader()
	_, err2 := dec2.DecodeFrame()
	if err2 != nil {
		fmt.Printf("  DecodeFrame: %v\n", err2)
	} else {
		fmt.Println("  Decode: OK")
	}

	// Try with first row flat (skip), second row gradient (B_PRED) - reverse
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			yuv[y*width+x] = 128
		}
	}
	for y := 16; y < 32; y++ {
		for x := 0; x < 16; x++ {
			yuv[y*width+x] = uint8(16 + x*14)
		}
	}

	enc3, _ := NewEncoder(width, height, 30)
	data3, _ := enc3.Encode(yuv)

	fmt.Printf("16x32 (flat row0, gradient row1): %d bytes\n", len(data3))

	dec3 := vp8.NewDecoder()
	dec3.Init(bytes.NewReader(data3), len(data3))
	_, _ = dec3.DecodeFrameHeader()
	_, err3 := dec3.DecodeFrame()
	if err3 != nil {
		fmt.Printf("  DecodeFrame: %v\n", err3)
	} else {
		fmt.Println("  Decode: OK")
	}
}
