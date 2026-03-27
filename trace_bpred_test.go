package vp8

import (
	"bytes"
	"testing"

	"golang.org/x/image/vp8"
)

func testBPredSize(t *testing.T, width, height int) {
	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	yuv := makeGradientYUV420(width, height)

	vp8Data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	t.Logf("%dx%d: Encoded %d bytes", width, height, len(vp8Data))

	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))

	_, err = dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}

	img, err := dec.DecodeFrame()
	if err != nil {
		t.Errorf("%dx%d: DecodeFrame: %v", width, height, err)
	} else {
		t.Logf("%dx%d: Decoded successfully: %v", width, height, img.Bounds())
	}
}

func TestBPredSize16x16(t *testing.T) { testBPredSize(t, 16, 16) }
func TestBPredSize32x16(t *testing.T) { testBPredSize(t, 32, 16) }
func TestBPredSize32x32(t *testing.T) { testBPredSize(t, 32, 32) }
func TestBPredSize48x48(t *testing.T) { testBPredSize(t, 48, 48) }
func TestBPredSize64x64(t *testing.T) { testBPredSize(t, 64, 64) }
func TestBPredSize80x80(t *testing.T) { testBPredSize(t, 80, 80) }

func TestBPredSize16x32(t *testing.T) { testBPredSize(t, 16, 32) }
func TestBPredSize16x48(t *testing.T) { testBPredSize(t, 16, 48) }
