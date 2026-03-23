package vp8

import (
	"fmt"
	"testing"
)

func TestDecodeCoeffFromBitstream(t *testing.T) {
	width, height := 32, 16

	ySize := width * height
	uvSize := ((width + 1) / 2) * ((height + 1) / 2)
	yuv := make([]byte, ySize+2*uvSize)

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			yuv[y*width+x] = 0
		}
		for x := 16; x < 32; x++ {
			yuv[y*width+x] = 255
		}
	}
	for i := ySize; i < ySize+2*uvSize; i++ {
		yuv[i] = 128
	}

	enc, _ := NewEncoder(width, height, 30)
	frame, _ := enc.Encode(yuv)

	firstPartLen := uint32(frame[0])>>5 | uint32(frame[1])<<3 | uint32(frame[2])<<11
	secondPart := frame[10+firstPartLen:]

	fmt.Printf("Second partition: %x\n", secondPart)

	dec := newCorrectPartition(secondPart)

	// Constants from x/image/vp8
	const (
		planeY1WithY2 = 0
		planeY2       = 1
		planeUV       = 2
		planeY1SansY2 = 3
	)

	probs := DefaultCoeffProbs

	// Decode MB0 Y2 block
	fmt.Println("\n=== MB0 Y2 block ===")
	y2_0 := decodeBlockCoeffsTest(dec, &probs, planeY2, 0, 0)
	fmt.Printf("MB0 Y2 coefficients: %v\n", y2_0)

	// Decode MB0 Y blocks (16 blocks, starting at coeff 1)
	for i := 0; i < 16; i++ {
		ctx := 0
		if y2_0[0] != 0 {
			ctx = 2
		}
		decodeBlockCoeffsTest(dec, &probs, planeY1SansY2, ctx, 1)
	}

	// Decode MB0 U blocks
	for i := 0; i < 4; i++ {
		decodeBlockCoeffsTest(dec, &probs, planeUV, 0, 0)
	}

	// Decode MB0 V blocks
	for i := 0; i < 4; i++ {
		decodeBlockCoeffsTest(dec, &probs, planeUV, 0, 0)
	}

	// Decode MB1 Y2 block
	fmt.Println("\n=== MB1 Y2 block ===")
	y2_1 := decodeBlockCoeffsTest(dec, &probs, planeY2, 0, 0)
	fmt.Printf("MB1 Y2 coefficients: %v\n", y2_1)

	fmt.Printf("\nMB0 Y2[0] = %d, expected -177\n", y2_0[0])
	fmt.Printf("MB1 Y2[0] = %d, expected 178\n", y2_1[0])
}

func decodeBlockCoeffsTest(dec *correctPartition, probs *[4][8][3][11]uint8, plane, leftCtx, start int) [16]int16 {
	var coeffs [16]int16
	ctx := leftCtx

	for i := start; i < 16; i++ {
		band := coeffBand[i]
		p := probs[plane][band][ctx][:]

		// p[0]: EOB check (skip for first coeff)
		if i > start {
			if !dec.readBit(p[0]) {
				// EOB
				return coeffs
			}
		}

		// p[1]: zero vs non-zero
		if !dec.readBit(p[1]) {
			coeffs[i] = 0
			ctx = 0
			continue
		}

		// Decode non-zero value
		var v int16

		if !dec.readBit(p[2]) {
			v = 1
		} else if !dec.readBit(p[3]) {
			v = 2
		} else if !dec.readBit(p[4]) {
			if !dec.readBit(p[5]) {
				v = 3
			} else {
				v = 4
			}
		} else if !dec.readBit(p[6]) {
			if !dec.readBit(p[7]) {
				v = 5
			} else {
				v = 6
			}
		} else if !dec.readBit(p[8]) {
			if !dec.readBit(p[9]) {
				// cat1
				v = 7 + int16(readCatTest(dec, catProbs[0][:]))
			} else {
				// cat2
				v = 9 + int16(readCatTest(dec, catProbs[1][:]))
			}
		} else if !dec.readBit(p[10]) {
			// cat3 or cat4
			if dec.readBit(180) { // This prob might be wrong
				// cat4
				v = 25 + int16(readCatTest(dec, catProbs[3][:]))
			} else {
				// cat3
				v = 13 + int16(readCatTest(dec, catProbs[2][:]))
			}
		} else {
			// cat5 or cat6
			if dec.readBit(254) {
				// cat6
				v = 67 + int16(readCatTest(dec, catProbs[5][:]))
			} else {
				// cat5
				v = 35 + int16(readCatTest(dec, catProbs[4][:]))
			}
		}

		// Sign
		if dec.readBit(128) {
			v = -v
		}

		coeffs[i] = v

		if v == 1 || v == -1 {
			ctx = 1
		} else {
			ctx = 2
		}
	}

	return coeffs
}

func readCatTest(dec *correctPartition, probs []uint8) int {
	var v int
	for _, p := range probs {
		v <<= 1
		if dec.readBit(p) {
			v |= 1
		}
	}
	return v
}
