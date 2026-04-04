package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredTraceMBOrder traces the order of MB encoding for 32x32.
func TestBPredTraceMBOrder(t *testing.T) {
	width, height := 32, 32 // 2x2 MB grid

	// Create a horizontal gradient
	// This should produce:
	// MB(0,0): B_PRED, MB(1,0): B_PRED
	// MB(0,1): skip (solid), MB(1,1): skip (solid)
	ySize := width * height
	cSize := (width / 2) * (height / 2)
	yuv := make([]byte, ySize+2*cSize)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			yuv[y*width+x] = uint8(16 + x*7) // horizontal gradient
		}
	}
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

	fmt.Printf("Gradient 32x32: %d bytes\n", len(data))

	// Now create all-B_PRED frame
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if (x+y)%2 == 0 {
				yuv[y*width+x] = 200
			} else {
				yuv[y*width+x] = 50
			}
		}
	}

	enc2, _ := NewEncoder(width, height, 30)
	data2, _ := enc2.Encode(yuv)
	fmt.Printf("Checkerboard 32x32: %d bytes\n", len(data2))

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data2), len(data2))
	_, _ = dec.DecodeFrameHeader()
	_, err = dec.DecodeFrame()
	if err != nil {
		fmt.Printf("Checkerboard (all B_PRED) decode: %v\n", err)
	} else {
		fmt.Println("Checkerboard decode OK")
	}

	// Try solid frame (all skip)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			yuv[y*width+x] = 128
		}
	}

	enc3, _ := NewEncoder(width, height, 30)
	data3, _ := enc3.Encode(yuv)
	fmt.Printf("Solid 32x32: %d bytes\n", len(data3))

	dec3 := vp8.NewDecoder()
	dec3.Init(bytes.NewReader(data3), len(data3))
	_, _ = dec3.DecodeFrameHeader()
	_, err = dec3.DecodeFrame()
	if err != nil {
		fmt.Printf("Solid (all skip) decode: %v\n", err)
	} else {
		fmt.Println("Solid decode OK")
	}

	// Try 2x1 (32x16) - this should work
	for y := 0; y < 16; y++ {
		for x := 0; x < 32; x++ {
			yuv[y*32+x] = uint8(16 + x*7) // horizontal gradient
		}
	}
	cSize16 := 16 * 8
	for i := 0; i < cSize16; i++ {
		yuv[32*16+i] = 128
		yuv[32*16+cSize16+i] = 128
	}

	enc4, _ := NewEncoder(32, 16, 30)
	data4, _ := enc4.Encode(yuv[:32*16+2*cSize16])
	fmt.Printf("Gradient 32x16: %d bytes\n", len(data4))

	dec4 := vp8.NewDecoder()
	dec4.Init(bytes.NewReader(data4), len(data4))
	_, _ = dec4.DecodeFrameHeader()
	_, err = dec4.DecodeFrame()
	if err != nil {
		fmt.Printf("32x16 gradient decode: %v\n", err)
	} else {
		fmt.Println("32x16 gradient decode OK")
	}
}
