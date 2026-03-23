package vp8

import (
	"testing"
)

func TestMacroblockSkip(t *testing.T) {
	// Create simple 32x16 image (2 macroblocks)
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

	frame, _ := NewYUV420Frame(yuv, width, height)
	qf := GetQuantFactorsSimple(enc.qi)

	mbW := 2
	mbH := 1

	// Process MB0 (left, Y=0)
	var srcY0 [256]byte
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			srcY0[row*16+col] = frame.Y[row*width+col]
		}
	}
	var srcU0, srcV0 [64]byte
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			srcU0[row*8+col] = frame.Cb[row*(width/2)+col]
			srcV0[row*8+col] = frame.Cr[row*(width/2)+col]
		}
	}
	ctx0 := enc.buildMBContext(frame, 0, 0, mbW, mbH)
	mb0 := processMacroblock(srcY0[:], srcU0[:], srcV0[:], ctx0, qf)

	t.Logf("MB0 (Y=0): skip=%v, lumaMode=%v", mb0.skip, mb0.lumaMode)
	t.Logf("MB0 Y2 coeffs: %v", mb0.y2Coeffs)

	// Process MB1 (right, Y=255)
	var srcY1 [256]byte
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			srcY1[row*16+col] = frame.Y[row*width+16+col]
		}
	}
	var srcU1, srcV1 [64]byte
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			srcU1[row*8+col] = frame.Cb[row*(width/2)+8+col]
			srcV1[row*8+col] = frame.Cr[row*(width/2)+8+col]
		}
	}
	ctx1 := enc.buildMBContext(frame, 1, 0, mbW, mbH)
	mb1 := processMacroblock(srcY1[:], srcU1[:], srcV1[:], ctx1, qf)

	t.Logf("MB1 (Y=255): skip=%v, lumaMode=%v", mb1.skip, mb1.lumaMode)
	t.Logf("MB1 Y2 coeffs: %v", mb1.y2Coeffs)
}
