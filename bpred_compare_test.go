package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestCompareSkipNonSkip compares skip vs non-skip after B_PRED.
func TestCompareSkipNonSkip(t *testing.T) {
	width, height := 32, 16 // 2x1 MB grid

	// Test 1: B_PRED + non-skip (passes)
	yuv1 := make([]byte, width*height*3/2)
	// Left MB: high detail (B_PRED)
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			if (row+col)%4 < 2 {
				yuv1[row*width+col] = 200
			} else {
				yuv1[row*width+col] = 50
			}
		}
	}
	// Right MB: different solid (non-skip)
	for row := 0; row < 16; row++ {
		for col := 16; col < 32; col++ {
			yuv1[row*width+col] = 200
		}
	}
	// Chroma
	for i := width * height; i < len(yuv1); i++ {
		yuv1[i] = 128
	}

	// Test 2: B_PRED + skip (fails)
	yuv2 := make([]byte, width*height*3/2)
	// Left MB: high detail (B_PRED)
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			if (row+col)%4 < 2 {
				yuv2[row*width+col] = 200
			} else {
				yuv2[row*width+col] = 50
			}
		}
	}
	// Right MB: solid 128 (skip)
	for row := 0; row < 16; row++ {
		for col := 16; col < 32; col++ {
			yuv2[row*width+col] = 128
		}
	}
	// Chroma
	for i := width * height; i < len(yuv2); i++ {
		yuv2[i] = 128
	}

	enc1, _ := NewEncoder(width, height, 30)
	enc2, _ := NewEncoder(width, height, 30)

	data1, _ := enc1.Encode(yuv1)
	data2, _ := enc2.Encode(yuv2)

	fmt.Printf("\nNon-skip case: %d bytes\n", len(data1))
	fmt.Printf("Skip case: %d bytes\n", len(data2))

	// Parse frame tags
	parseTag := func(data []byte, name string) {
		tag := int(data[0]) | int(data[1])<<8 | int(data[2])<<16
		keyFrame := (tag & 1) == 0
		partSize := tag >> 5
		fmt.Printf("%s: keyFrame=%v, partition1Size=%d\n", name, keyFrame, partSize)
		fmt.Printf("First 40 bytes: % x\n", data[:min(40, len(data))])
	}

	parseTag(data1, "Non-skip")
	parseTag(data2, "Skip")

	// Try to decode both
	tryDecode := func(data []byte, name string) {
		dec := vp8.NewDecoder()
		dec.Init(bytes.NewReader(data), len(data))
		_, err := dec.DecodeFrameHeader()
		if err != nil {
			fmt.Printf("%s: DecodeFrameHeader error: %v\n", name, err)
			return
		}
		_, err = dec.DecodeFrame()
		if err != nil {
			fmt.Printf("%s: DecodeFrame error: %v\n", name, err)
		} else {
			fmt.Printf("%s: Decode OK\n", name)
		}
	}

	fmt.Println()
	tryDecode(data1, "Non-skip")
	tryDecode(data2, "Skip")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
