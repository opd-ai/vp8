package vp8

import (
	"bytes"
	"testing"

	"golang.org/x/image/vp8"
)

func TestDecodeVerifyDetail(t *testing.T) {
	// Create 32x16 image: left half Y=0, right half Y=255
	width, height := 32, 16
	yuv := make([]byte, width*height+2*(width/2)*(height/2))

	// Y plane: left=0, right=255
	ySize := width * height
	for y := 0; y < height; y++ {
		for x := 0; x < width/2; x++ {
			yuv[y*width+x] = 0
		}
		for x := width / 2; x < width; x++ {
			yuv[y*width+x] = 255
		}
	}
	// U plane: 128
	uOffset := ySize
	uSize := (width / 2) * (height / 2)
	for i := 0; i < uSize; i++ {
		yuv[uOffset+i] = 128
	}
	// V plane: 128
	vOffset := ySize + uSize
	for i := 0; i < uSize; i++ {
		yuv[vOffset+i] = 128
	}

	// Encode
	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	vp8Data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	t.Logf("Encoded %d bytes: %x", len(vp8Data), vp8Data)

	// Decode
	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))
	fh, err := dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}
	t.Logf("Frame: %dx%d", fh.Width, fh.Height)

	decoded, err := dec.DecodeFrame()
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	// Check Y values at key positions
	mb0Center := decoded.Y[8*decoded.YStride+8]
	mb1Center := decoded.Y[8*decoded.YStride+24]

	t.Logf("MB0 center Y: %d (expected 0)", mb0Center)
	t.Logf("MB1 center Y: %d (expected 255)", mb1Center)

	// Print column 15 (MB0 right edge) and column 16 (MB1 left edge)
	t.Logf("Y values at boundary (x=15 vs x=16):")
	for y := 0; y < height; y++ {
		y15 := decoded.Y[y*decoded.YStride+15]
		y16 := decoded.Y[y*decoded.YStride+16]
		t.Logf("  row %2d: x=15:%3d  x=16:%3d", y, y15, y16)
	}
}
