package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredRowVsCol compares 32x16 (2x1) vs 16x32 (1x2).
func TestBPredRowVsCol(t *testing.T) {
	// Create identical content for both
	makeGradient := func(w, h int) []byte {
		ySize := w * h
		cSize := (w / 2) * (h / 2)
		yuv := make([]byte, ySize+2*cSize)
		// Horizontal gradient normalized to same values
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				// Same value at same relative position
				yuv[y*w+x] = uint8(16 + (x%16)*14)
			}
		}
		for i := 0; i < cSize; i++ {
			yuv[ySize+i] = 128
			yuv[ySize+cSize+i] = 128
		}
		return yuv
	}

	// 32x16 (2x1 MBs)
	{
		w, h := 32, 16
		yuv := makeGradient(w, h)
		enc, _ := NewEncoder(w, h, 30)
		data, _ := enc.Encode(yuv)

		frameTag := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
		p1Size := int(frameTag >> 5)

		fmt.Printf("32x16 (2x1 MBs): %d bytes, partition1=%d\n", len(data), p1Size)
		fmt.Printf("Hex: ")
		for _, b := range data {
			fmt.Printf("%02x ", b)
		}
		fmt.Println()

		dec := vp8.NewDecoder()
		dec.Init(bytes.NewReader(data), len(data))
		_, _ = dec.DecodeFrameHeader()
		_, err := dec.DecodeFrame()
		if err != nil {
			fmt.Printf("FAIL: %v\n", err)
		} else {
			fmt.Printf("OK\n")
		}
	}

	// 16x32 (1x2 MBs)
	{
		w, h := 16, 32
		yuv := makeGradient(w, h)
		enc, _ := NewEncoder(w, h, 30)
		data, _ := enc.Encode(yuv)

		frameTag := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
		p1Size := int(frameTag >> 5)

		fmt.Printf("16x32 (1x2 MBs): %d bytes, partition1=%d\n", len(data), p1Size)
		fmt.Printf("Hex: ")
		for _, b := range data {
			fmt.Printf("%02x ", b)
		}
		fmt.Println()

		dec := vp8.NewDecoder()
		dec.Init(bytes.NewReader(data), len(data))
		_, _ = dec.DecodeFrameHeader()
		_, err := dec.DecodeFrame()
		if err != nil {
			fmt.Printf("FAIL: %v\n", err)
		} else {
			fmt.Printf("OK\n")
		}
	}
}
