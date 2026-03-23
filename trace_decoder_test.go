package vp8

import (
	"fmt"
	"testing"
)

func TestTraceDecoder(t *testing.T) {
	// Decode path for Y2[0]=178

	// 1. Dequantize: 178 * 46
	dequant := int16(178 * 46)
	fmt.Printf("Dequantized Y2[0]: 178 * 46 = %d\n", dequant)

	// 2. Inverse WHT
	whtIn := [16]int16{}
	whtIn[0] = dequant
	whtOut := InverseWHT4x4(whtIn)
	fmt.Printf("\nInverse WHT output (DC values for each block): %v\n", whtOut)

	// 3. Inverse DCT for one block (DC only)
	dctIn := [16]int16{}
	dctIn[0] = whtOut[0]
	dctOut := InverseDCT4x4(dctIn)
	fmt.Printf("\nInverse DCT output (residual for one 4x4 block): %v\n", dctOut)

	// 4. Add V_PRED (127)
	final := int(dctOut[0]) + 127
	fmt.Printf("\nFinal pixel: %d + 127 = %d\n", dctOut[0], final)

	// Clamp
	if final < 0 {
		final = 0
	} else if final > 255 {
		final = 255
	}
	fmt.Printf("Clamped: %d\n", final)
}
