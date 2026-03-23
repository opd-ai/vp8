package vp8

import (
	"bytes"
	"fmt"
	"testing"

	xvp8 "golang.org/x/image/vp8"
)

func TestFullEncodeDecode(t *testing.T) {
	// Create simple 32x16 image (2 macroblocks)
	// Left MB: Y=0, Right MB: Y=255
	width, height := 32, 16
	ySize := width * height
	uvSize := (width / 2) * (height / 2)
	yuv := make([]byte, ySize+2*uvSize)

	// Y plane: left half = 0, right half = 255
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x < 16 {
				yuv[y*width+x] = 0
			} else {
				yuv[y*width+x] = 255
			}
		}
	}
	// U/V: neutral gray
	for i := ySize; i < len(yuv); i++ {
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

	// Decode with x/image/vp8
	dec := xvp8.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))

	fh, err := dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}
	t.Logf("Frame: %dx%d, keyframe=%v", fh.Width, fh.Height, fh.KeyFrame)

	img, err := dec.DecodeFrame()
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	// Show reconstructed Y values
	t.Logf("Reconstructed Y values (first row):")
	line := ""
	for x := 0; x < width; x++ {
		yVal := img.Y[x]
		line += fmt.Sprintf("%3d ", yVal)
	}
	t.Log(line)

	// Check PSNR for left and right halves
	var mseLeft, mseRight float64
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			orig := float64(yuv[y*width+x])
			rec := float64(img.Y[y*img.YStride+x])
			diff := orig - rec
			if x < 16 {
				mseLeft += diff * diff
			} else {
				mseRight += diff * diff
			}
		}
	}
	mseLeft /= float64(height * 16)
	mseRight /= float64(height * 16)

	t.Logf("MSE Left (should be ~0): %.2f", mseLeft)
	t.Logf("MSE Right (should be ~0): %.2f", mseRight)
}
