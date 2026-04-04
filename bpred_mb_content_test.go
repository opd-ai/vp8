package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredMBContent traces actual MB content and modes.
func TestBPredMBContent(t *testing.T) {
	for _, tc := range []struct {
		w, h int
	}{
		{32, 16}, // 2x1 - OK
		{16, 32}, // 1x2 - FAIL
	} {
		fmt.Printf("\n=== %dx%d (%dx%d MBs) ===\n", tc.w, tc.h, (tc.w+15)/16, (tc.h+15)/16)

		ySize := tc.w * tc.h
		cSize := (tc.w / 2) * (tc.h / 2)
		yuv := make([]byte, ySize+2*cSize)

		// Same gradient pattern
		for y := 0; y < tc.h; y++ {
			for x := 0; x < tc.w; x++ {
				yuv[y*tc.w+x] = uint8(16 + (x%16)*14)
			}
		}
		for i := 0; i < cSize; i++ {
			yuv[ySize+i] = 128
			yuv[ySize+cSize+i] = 128
		}

		// Print actual pixel values for each MB
		mbW := (tc.w + 15) / 16
		mbH := (tc.h + 15) / 16
		for mbY := 0; mbY < mbH; mbY++ {
			for mbX := 0; mbX < mbW; mbX++ {
				fmt.Printf("MB(%d,%d) first row: ", mbX, mbY)
				for x := 0; x < 16; x++ {
					px := mbX*16 + x
					py := mbY * 16
					if px < tc.w && py < tc.h {
						fmt.Printf("%d ", yuv[py*tc.w+px])
					}
				}
				fmt.Println()
			}
		}

		enc, _ := NewEncoder(tc.w, tc.h, 30)
		data, _ := enc.Encode(yuv)

		dec := vp8.NewDecoder()
		dec.Init(bytes.NewReader(data), len(data))
		_, _ = dec.DecodeFrameHeader()
		_, err := dec.DecodeFrame()
		if err != nil {
			fmt.Printf("Decode: FAIL %v\n", err)
		} else {
			fmt.Printf("Decode: OK\n")
		}
	}
}
