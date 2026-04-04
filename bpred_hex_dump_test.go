package vp8

import (
	"fmt"
	"testing"
)

// TestBPredHexDump dumps full hex for failing and passing cases.
func TestBPredHexDump(t *testing.T) {
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

		frameTag := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
		p1Size := int(frameTag >> 5)
		p1End := 3 + p1Size

		fmt.Printf("Total: %d bytes, partition1=%d (bytes 3-%d), partition2=%d (bytes %d-end)\n",
			len(data), p1Size, p1End, len(data)-p1End, p1End)

		// Frame tag + keyframe header
		fmt.Printf("Frame tag (3 bytes): ")
		for i := 0; i < 3; i++ {
			fmt.Printf("%02x ", data[i])
		}
		fmt.Println()

		// Keyframe start code + dimensions (7 bytes)
		fmt.Printf("Keyframe header (7 bytes): ")
		for i := 3; i < 10; i++ {
			fmt.Printf("%02x ", data[i])
		}
		fmt.Println()

		// Partition 1 content
		fmt.Printf("Partition 1 (%d bytes): ", p1Size-7)
		for i := 10; i < p1End; i++ {
			fmt.Printf("%02x ", data[i])
		}
		fmt.Println()

		// Partition 2 content
		fmt.Printf("Partition 2 (%d bytes): ", len(data)-p1End)
		for i := p1End; i < len(data); i++ {
			fmt.Printf("%02x ", data[i])
		}
		fmt.Println()
	}
}
