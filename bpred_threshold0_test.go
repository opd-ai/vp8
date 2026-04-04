package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredThreshold0 tests with B_PRED disabled (threshold=0).
func TestBPredThreshold0(t *testing.T) {
	// Create test frames
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{"16x16", 16, 16},
		{"32x16", 32, 16},
		{"16x32", 16, 32},
		{"32x32", 32, 32},
		{"64x64", 64, 64},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ySize := tc.width * tc.height
			cSize := (tc.width / 2) * (tc.height / 2)
			yuv := make([]byte, ySize+2*cSize)

			// Create gradient
			for y := 0; y < tc.height; y++ {
				for x := 0; x < tc.width; x++ {
					yuv[y*tc.width+x] = uint8((16 + x*200/tc.width) % 256)
				}
			}
			for i := 0; i < cSize; i++ {
				yuv[ySize+i] = 128
				yuv[ySize+cSize+i] = 128
			}

			enc, _ := NewEncoder(tc.width, tc.height, 30)
			data, _ := enc.Encode(yuv)

			fmt.Printf("%s: %d bytes\n", tc.name, len(data))

			dec := vp8.NewDecoder()
			dec.Init(bytes.NewReader(data), len(data))
			_, _ = dec.DecodeFrameHeader()
			_, err := dec.DecodeFrame()
			if err != nil {
				t.Errorf("DecodeFrame: %v", err)
			}
		})
	}
}
