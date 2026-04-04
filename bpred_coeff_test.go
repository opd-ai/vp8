package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredCoeffTracking traces the coefficient encoding for failing case.
func TestBPredCoeffTracking(t *testing.T) {
	// 16x32 gradient - should trigger B_PRED in row 0, skip in row 1
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

	fmt.Printf("Encoded: %d bytes\n", len(data))

	// Parse frame tag
	frameTag := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
	firstPartSize := int(frameTag >> 5)

	fmt.Printf("Frame tag: %02x %02x %02x\n", data[0], data[1], data[2])
	fmt.Printf("First partition size: %d\n", firstPartSize)

	// For keyframe, after 3 bytes frame tag comes:
	// - 3 bytes start code (9d 01 2a)
	// - 4 bytes for width/height

	headerOffset := 10 // 3 + 3 + 4
	part1End := 3 + firstPartSize
	part2Start := part1End

	fmt.Printf("Part 1: bytes %d-%d (%d bytes)\n", headerOffset, part1End, firstPartSize-7)
	fmt.Printf("Part 2: bytes %d-end (%d bytes)\n", part2Start, len(data)-part2Start)

	// Decode to find where it fails
	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))
	_, err := dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	img, err := dec.DecodeFrame()
	if err != nil {
		t.Errorf("DecodeFrame: %v", err)
	} else {
		t.Logf("Decoded: %v", img.Bounds())
	}
}
