package vp8

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/image/vp8"
)

func encodeAndDump(t *testing.T, name string, yuv []byte, width, height int) {
	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("%s: NewEncoder: %v", name, err)
	}

	data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("%s: Encode: %v", name, err)
	}

	frameTag := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
	firstPartSize := int(frameTag >> 5)

	// Keyframe header is 7 bytes after 3-byte frame tag, so partition 1 starts at byte 10
	// Partition 1 size is in the frame tag
	// Partition 2 starts after partition 1

	headerSize := 3 + 7 // frame tag + keyframe header
	p1Start := headerSize
	p1End := p1Start + firstPartSize - 7 // firstPartSize includes the 7-byte header
	p2Start := 3 + firstPartSize

	fmt.Printf("\n%s: total=%d bytes\n", name, len(data))
	fmt.Printf("  Header: %d bytes (frame tag + keyframe header)\n", headerSize)
	fmt.Printf("  Partition 1 (modes): bytes %d-%d (%d bytes)\n", p1Start, p1End, firstPartSize-7)
	fmt.Printf("  Partition 2 (coeffs): bytes %d-end (%d bytes)\n", p2Start, len(data)-p2Start)

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))
	_, err = dec.DecodeFrameHeader()
	if err != nil {
		fmt.Printf("  DecodeFrameHeader: %v\n", err)
		return
	}
	_, err = dec.DecodeFrame()
	if err != nil {
		fmt.Printf("  DecodeFrame: %v\n", err)
	} else {
		fmt.Printf("  Decode: OK\n")
	}
}

func TestBPredComparePartitions(t *testing.T) {
	// 32x16 (2x1 MBs) - horizontal gradient (B_PRED + skip in same row)
	{
		width, height := 32, 16
		ySize := width * height
		cSize := (width / 2) * (height / 2)
		yuv := make([]byte, ySize+2*cSize)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				yuv[y*width+x] = uint8(16 + x*7)
			}
		}
		for i := 0; i < cSize; i++ {
			yuv[ySize+i] = 128
			yuv[ySize+cSize+i] = 128
		}
		encodeAndDump(t, "32x16 gradient (2x1)", yuv, width, height)
	}

	// 32x32 (2x2 MBs) - horizontal gradient (B_PRED in row 0, skip in row 1)
	{
		width, height := 32, 32
		ySize := width * height
		cSize := (width / 2) * (height / 2)
		yuv := make([]byte, ySize+2*cSize)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				yuv[y*width+x] = uint8(16 + x*7)
			}
		}
		for i := 0; i < cSize; i++ {
			yuv[ySize+i] = 128
			yuv[ySize+cSize+i] = 128
		}
		encodeAndDump(t, "32x32 gradient (2x2)", yuv, width, height)
	}

	// 16x32 (1x2 MBs) - horizontal gradient (B_PRED in row 0, skip in row 1)
	{
		width, height := 16, 32
		ySize := width * height
		cSize := (width / 2) * (height / 2)
		yuv := make([]byte, ySize+2*cSize)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				yuv[y*width+x] = uint8(16 + x*14) // steeper gradient
			}
		}
		for i := 0; i < cSize; i++ {
			yuv[ySize+i] = 128
			yuv[ySize+cSize+i] = 128
		}
		encodeAndDump(t, "16x32 gradient (1x2)", yuv, width, height)
	}
}
