package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredMinimal creates a minimal reproducible case.
func TestBPredMinimal(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		fill   func(yuv []byte, w, h int)
	}{
		{
			name:   "16x16 gradient",
			width:  16,
			height: 16,
			fill: func(yuv []byte, w, h int) {
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						yuv[y*w+x] = uint8(16 + x*14)
					}
				}
			},
		},
		{
			name:   "32x16 gradient (single row)",
			width:  32,
			height: 16,
			fill: func(yuv []byte, w, h int) {
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						yuv[y*w+x] = uint8(16 + x*7)
					}
				}
			},
		},
		{
			name:   "16x32 gradient column",
			width:  16,
			height: 32,
			fill: func(yuv []byte, w, h int) {
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						yuv[y*w+x] = uint8(16 + x*14)
					}
				}
			},
		},
		{
			name:   "32x32 gradient",
			width:  32,
			height: 32,
			fill: func(yuv []byte, w, h int) {
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						yuv[y*w+x] = uint8(16 + x*7)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ySize := tt.width * tt.height
			cSize := (tt.width / 2) * (tt.height / 2)
			yuv := make([]byte, ySize+2*cSize)

			tt.fill(yuv, tt.width, tt.height)

			// Neutral chroma
			for i := 0; i < cSize; i++ {
				yuv[ySize+i] = 128
				yuv[ySize+cSize+i] = 128
			}

			enc, _ := NewEncoder(tt.width, tt.height, 30)
			data, err := enc.Encode(yuv)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}

			fmt.Printf("%s: %d bytes\n", tt.name, len(data))

			dec := vp8.NewDecoder()
			dec.Init(bytes.NewReader(data), len(data))
			_, err = dec.DecodeFrameHeader()
			if err != nil {
				t.Fatalf("DecodeFrameHeader: %v", err)
			}
			_, err = dec.DecodeFrame()
			if err != nil {
				t.Errorf("DecodeFrame: %v", err)
			}
		})
	}
}
