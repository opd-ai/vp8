package vp8

import (
	"fmt"
	"testing"
)

func TestTraceEncode(t *testing.T) {
	// What does the encoder write for -177?
	// This is a cat6 value (|v| >= 67)

	// -177: base is 67, extra is 110 (11 bits)
	// 177 - 67 = 110
	// 110 in binary: 0b001101110

	// The token tree for -177:
	// p[1]: non-zero=true
	// p[2]: not 1 -> true
	// p[3]: not 2 -> true
	// p[4]: not 3-4 -> true
	// p[6]: not 5-6 -> true (cat)
	// p[8]: not cat1-2 -> true (cat3+)
	// p[10]: not cat3-4 -> true (cat5-6)
	// cat5-6 split: cat6 -> true (prob 254)
	// 11 extra bits for cat6
	// sign bit: true (negative)

	fmt.Println("Expected encoding path for -177:")
	fmt.Println("p[1]=35: true (non-zero)")
	fmt.Println("p[2]=237: true (not 1)")
	fmt.Println("p[3]=223: true (not 2)")
	fmt.Println("p[4]=193: true (not 3-4)")
	fmt.Println("p[6]=162: true (not 5-6, is cat)")
	fmt.Println("p[8]=145: true (not cat1-2)")
	fmt.Println("p[10]=62: true (not cat3-4, is cat5-6)")
	fmt.Println("254: true (cat6, not cat5)")
	fmt.Println("11 extra bits for 110: 00001101110")
	fmt.Println("sign=true (negative)")

	fmt.Println("\nLet's see what the encoder actually writes...")

	// Create encoder and write a single -177 value
	enc := newBoolEncoder()

	// Use Y2 plane (1), band 0, ctx 0 probs
	probs := DefaultCoeffProbs[1][0][0][:]
	fmt.Printf("Probs: %v\n", probs)

	// Write the token for -177
	v := int16(-177)
	av := v
	if av < 0 {
		av = -av
	}

	fmt.Printf("\nEncoding value %d (abs=%d):\n", v, av)

	// p[1]: non-zero
	enc.putBit(probs[1], true)
	fmt.Printf("p[1]=%d: true\n", probs[1])

	// p[2]: not 1
	enc.putBit(probs[2], true)
	fmt.Printf("p[2]=%d: true\n", probs[2])

	// p[3]: not 2
	enc.putBit(probs[3], true)
	fmt.Printf("p[3]=%d: true\n", probs[3])

	// p[4]: not 3-4
	enc.putBit(probs[4], true)
	fmt.Printf("p[4]=%d: true\n", probs[4])

	// p[6]: not 5-6 (cat)
	enc.putBit(probs[6], true)
	fmt.Printf("p[6]=%d: true\n", probs[6])

	// p[8]: not cat1-2
	enc.putBit(probs[8], true)
	fmt.Printf("p[8]=%d: true\n", probs[8])

	// p[10]: cat5-6 (not cat3-4)
	enc.putBit(probs[10], true)
	fmt.Printf("p[10]=%d: true\n", probs[10])

	// cat5 vs cat6 - need prob 254
	// For cat6, we write true
	enc.putBit(254, true)
	fmt.Println("254: true (cat6)")

	// Write 11 extra bits with cat6 probs
	extra := int(av) - 67
	fmt.Printf("Extra bits for %d: %d (binary: %011b)\n", av, extra, extra)

	for i, p := range catProbs[5] {
		bit := (extra >> (10 - i)) & 1
		enc.putBit(p, bit == 1)
		fmt.Printf("  cat6[%d] p=%d: %v\n", i, p, bit == 1)
	}

	// Sign bit
	enc.putBit(128, true)
	fmt.Println("sign=true")

	enc.flush()
	fmt.Printf("\nEncoded bytes: %x\n", enc.buf)

	// Now compare with what the frame's second partition has
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

	encFull, _ := NewEncoder(width, height, 30)
	frame, _ := encFull.Encode(yuv)

	firstPartLen := uint32(frame[0])>>5 | uint32(frame[1])<<3 | uint32(frame[2])<<11
	secondPart := frame[10+firstPartLen:]

	fmt.Printf("\nActual second partition: %x\n", secondPart)
}
