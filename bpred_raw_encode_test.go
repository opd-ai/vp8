package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredRawEncode traces raw encoding for failing and passing cases.
func TestBPredRawEncode(t *testing.T) {
	for _, tc := range []struct {
		w, h int
	}{
		{16, 32}, // FAIL
		{32, 32}, // OK
	} {
		fmt.Printf("\n=== %dx%d ===\n", tc.w, tc.h)

		ySize := tc.w * tc.h
		cSize := (tc.w / 2) * (tc.h / 2)
		yuv := make([]byte, ySize+2*cSize)

		for y := 0; y < tc.h; y++ {
			for x := 0; x < tc.w; x++ {
				yuv[y*tc.w+x] = uint8((16 + x*200/tc.w) % 256)
			}
		}
		for i := 0; i < cSize; i++ {
			yuv[ySize+i] = 128
			yuv[ySize+cSize+i] = 128
		}

		enc, _ := NewEncoder(tc.w, tc.h, 30)
		data, _ := enc.Encode(yuv)

		fmt.Printf("Encoded: %d bytes\n", len(data))
		fmt.Printf("Hex: ")
		for i, b := range data {
			if i < 50 || i > len(data)-10 {
				fmt.Printf("%02x ", b)
			} else if i == 50 {
				fmt.Printf("... ")
			}
		}
		fmt.Println()

		// Parse
		frameTag := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
		p1Size := int(frameTag >> 5)
		fmt.Printf("Frame tag: firstPartSize=%d\n", p1Size)

		// Try decode
		dec := vp8.NewDecoder()
		dec.Init(bytes.NewReader(data), len(data))
		fh, _ := dec.DecodeFrameHeader()
		fmt.Printf("Decoded header: %dx%d, keyframe=%v\n", fh.Width, fh.Height, fh.KeyFrame)

		_, err := dec.DecodeFrame()
		if err != nil {
			fmt.Printf("DecodeFrame ERROR: %v\n", err)
		} else {
			fmt.Printf("DecodeFrame OK\n")
		}
	}
}
