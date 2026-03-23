package vp8

import (
	"bytes"
	"testing"

	"golang.org/x/image/vp8"
)

func TestTwoMBsBlackWhite(t *testing.T) {
	// 32x16: MB0=black(0), MB1=white(255)
	width, height := 32, 16
	yuv := make([]byte, width*height+2*(width/2)*(height/2))

	for y := 0; y < height; y++ {
		for x := 0; x < width/2; x++ {
			yuv[y*width+x] = 0
		}
		for x := width / 2; x < width; x++ {
			yuv[y*width+x] = 255
		}
	}
	for i := width * height; i < len(yuv); i++ {
		yuv[i] = 128
	}

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	vp8Data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	t.Logf("Encoded %d bytes: %x", len(vp8Data), vp8Data)

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))
	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	decoded, err := dec.DecodeFrame()
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	mb0y := decoded.Y[8*decoded.YStride+8]
	mb1y := decoded.Y[8*decoded.YStride+24]
	t.Logf("MB0 Y: %d (expected 0)", mb0y)
	t.Logf("MB1 Y: %d (expected 255)", mb1y)
}

func TestTwoMBsWhiteBlack(t *testing.T) {
	// 32x16: MB0=white(255), MB1=black(0)
	width, height := 32, 16
	yuv := make([]byte, width*height+2*(width/2)*(height/2))

	for y := 0; y < height; y++ {
		for x := 0; x < width/2; x++ {
			yuv[y*width+x] = 255
		}
		for x := width / 2; x < width; x++ {
			yuv[y*width+x] = 0
		}
	}
	for i := width * height; i < len(yuv); i++ {
		yuv[i] = 128
	}

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	vp8Data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	t.Logf("Encoded %d bytes: %x", len(vp8Data), vp8Data)

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))
	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	decoded, err := dec.DecodeFrame()
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	mb0y := decoded.Y[8*decoded.YStride+8]
	mb1y := decoded.Y[8*decoded.YStride+24]
	t.Logf("MB0 Y: %d (expected 255)", mb0y)
	t.Logf("MB1 Y: %d (expected 0)", mb1y)
}

func TestTwoMBsAllBlack(t *testing.T) {
	// 32x16: Both black
	width, height := 32, 16
	yuv := make([]byte, width*height+2*(width/2)*(height/2))

	for i := 0; i < width*height; i++ {
		yuv[i] = 0
	}
	for i := width * height; i < len(yuv); i++ {
		yuv[i] = 128
	}

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	vp8Data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	t.Logf("Encoded %d bytes: %x", len(vp8Data), vp8Data)

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))
	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	decoded, err := dec.DecodeFrame()
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	mb0y := decoded.Y[8*decoded.YStride+8]
	mb1y := decoded.Y[8*decoded.YStride+24]
	t.Logf("MB0 Y: %d (expected 0)", mb0y)
	t.Logf("MB1 Y: %d (expected 0)", mb1y)
}

func TestTwoMBsAllWhite(t *testing.T) {
	// 32x16: Both white
	width, height := 32, 16
	yuv := make([]byte, width*height+2*(width/2)*(height/2))

	for i := 0; i < width*height; i++ {
		yuv[i] = 255
	}
	for i := width * height; i < len(yuv); i++ {
		yuv[i] = 128
	}

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	vp8Data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	t.Logf("Encoded %d bytes: %x", len(vp8Data), vp8Data)

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))
	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	decoded, err := dec.DecodeFrame()
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	mb0y := decoded.Y[8*decoded.YStride+8]
	mb1y := decoded.Y[8*decoded.YStride+24]
	t.Logf("MB0 Y: %d (expected 255)", mb0y)
	t.Logf("MB1 Y: %d (expected 255)", mb1y)
}
