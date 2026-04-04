package vp8

import (
	"bytes"
	"fmt"
	"image"
	"testing"

	"golang.org/x/image/vp8"
)

// TestBPredDumpPartitions dumps partition info for 32x32 gradient.
func TestBPredDumpPartitions(t *testing.T) {
	width, height := 32, 32 // 2x2 MB grid

	// Create a horizontal gradient
	img := image.NewYCbCr(image.Rect(0, 0, width, height), image.YCbCrSubsampleRatio420)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Y[y*img.YStride+x] = uint8(16 + x*7)
		}
	}
	for y := 0; y < height/2; y++ {
		for x := 0; x < width/2; x++ {
			img.Cb[y*img.CStride+x] = 128
			img.Cr[y*img.CStride+x] = 128
		}
	}

	// Convert to planar YUV
	ySize := width * height
	cSize := (width / 2) * (height / 2)
	yuv := make([]byte, ySize+2*cSize)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			yuv[y*width+x] = img.Y[y*img.YStride+x]
		}
	}
	for y := 0; y < height/2; y++ {
		for x := 0; x < width/2; x++ {
			yuv[ySize+y*(width/2)+x] = img.Cb[y*img.CStride+x]
			yuv[ySize+cSize+y*(width/2)+x] = img.Cr[y*img.CStride+x]
		}
	}

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	fmt.Printf("Encoded %d bytes\n", len(data))

	// Parse VP8 frame header
	if len(data) < 10 {
		t.Fatalf("Data too short")
	}

	frameTag := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
	keyFrame := (frameTag & 1) == 0
	firstPartSize := int(frameTag >> 5)

	fmt.Printf("Frame tag: keyFrame=%v, firstPartSize=%d\n", keyFrame, firstPartSize)
	fmt.Printf("Total size=%d, header+partition1=%d+3=%d, partition2=%d\n",
		len(data), firstPartSize, firstPartSize+3, len(data)-firstPartSize-3)

	// Hex dump
	fmt.Printf("First 30 bytes: ")
	for i := 0; i < 30 && i < len(data); i++ {
		fmt.Printf("%02x ", data[i])
	}
	fmt.Printf("\n")

	// Try to decode
	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))

	fh, err := dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}
	fmt.Printf("FrameHeader: Key=%v, Width=%d, Height=%d\n", fh.KeyFrame, fh.Width, fh.Height)

	_, err = dec.DecodeFrame()
	if err != nil {
		t.Errorf("DecodeFrame: %v", err)
	} else {
		t.Log("Decode OK")
	}
}

// TestBPredDumpPartitions16x16 tests 16x16 which should work.
func TestBPredDumpPartitions16x16(t *testing.T) {
	width, height := 16, 16 // 1x1 MB grid

	// Create a horizontal gradient
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

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	fmt.Printf("Encoded %d bytes\n", len(data))

	// Parse VP8 frame header
	frameTag := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
	keyFrame := (frameTag & 1) == 0
	firstPartSize := int(frameTag >> 5)

	fmt.Printf("Frame tag: keyFrame=%v, firstPartSize=%d\n", keyFrame, firstPartSize)
	fmt.Printf("Total size=%d, partition1=%d, partition2=%d\n",
		len(data), firstPartSize+10, len(data)-firstPartSize-10)

	// Try to decode
	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(data), len(data))

	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	_, err = dec.DecodeFrame()
	if err != nil {
		t.Errorf("DecodeFrame: %v", err)
	} else {
		t.Log("Decode OK")
	}
}
