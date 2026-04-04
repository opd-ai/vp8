package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredHexCompare compares hex dumps of passing vs failing cases.
func TestBPredHexCompare(t *testing.T) {
	// Case 1: 16x16 (passes)
	{
		width, height := 16, 16
		ySize := width * height
		cSize := (width / 2) * (height / 2)
		yuv := make([]byte, ySize+2*cSize)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				yuv[y*width+x] = uint8(16 + x*14)
			}
		}
		for i := 0; i < cSize; i++ {
			yuv[ySize+i] = 128
			yuv[ySize+cSize+i] = 128
		}

		enc, _ := NewEncoder(width, height, 30)
		data, _ := enc.Encode(yuv)

		frameTag := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
		p1Size := int(frameTag >> 5)

		fmt.Printf("16x16: %d bytes, partition1=%d\n", len(data), p1Size)
		fmt.Printf("Full: ")
		for _, b := range data {
			fmt.Printf("%02x ", b)
		}
		fmt.Println()
	}

	// Case 2: 16x32 (fails)
	{
		width, height := 16, 32
		ySize := width * height
		cSize := (width / 2) * (height / 2)
		yuv := make([]byte, ySize+2*cSize)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				yuv[y*width+x] = uint8(16 + x*14)
			}
		}
		for i := 0; i < cSize; i++ {
			yuv[ySize+i] = 128
			yuv[ySize+cSize+i] = 128
		}

		enc, _ := NewEncoder(width, height, 30)
		data, _ := enc.Encode(yuv)

		frameTag := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
		p1Size := int(frameTag >> 5)

		fmt.Printf("\n16x32: %d bytes, partition1=%d\n", len(data), p1Size)
		fmt.Printf("Full: ")
		for _, b := range data {
			fmt.Printf("%02x ", b)
		}
		fmt.Println()

		// Try decode
		dec := vp8.NewDecoder()
		dec.Init(bytes.NewReader(data), len(data))
		_, _ = dec.DecodeFrameHeader()
		_, err := dec.DecodeFrame()
		fmt.Printf("Decode: %v\n", err)
	}
}
