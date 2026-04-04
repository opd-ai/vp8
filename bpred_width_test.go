package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredWidthEffect tests different widths.
func TestBPredWidthEffect(t *testing.T) {
	// Test various widths with 2 MB rows
	heights := []int{32, 48}
	widths := []int{16, 32, 48, 64}

	for _, h := range heights {
		for _, w := range widths {
			name := fmt.Sprintf("%dx%d", w, h)
			t.Run(name, func(t *testing.T) {
				ySize := w * h
				cSize := (w / 2) * (h / 2)
				yuv := make([]byte, ySize+2*cSize)

				// Gradient
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						yuv[y*w+x] = uint8((16 + x*200/w) % 256)
					}
				}
				for i := 0; i < cSize; i++ {
					yuv[ySize+i] = 128
					yuv[ySize+cSize+i] = 128
				}

				enc, _ := NewEncoder(w, h, 30)
				data, _ := enc.Encode(yuv)

				fmt.Printf("%s: %d bytes (%dx%d MBs)\n", name, len(data), (w+15)/16, (h+15)/16)

				dec := vp8.NewDecoder()
				dec.Init(bytes.NewReader(data), len(data))
				_, _ = dec.DecodeFrameHeader()
				_, err := dec.DecodeFrame()
				if err != nil {
					t.Errorf("FAIL: %v", err)
				} else {
					t.Log("OK")
				}
			})
		}
	}
}
