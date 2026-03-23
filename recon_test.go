package vp8

import (
	"fmt"
	"testing"
)

func TestReconstruction(t *testing.T) {
	// For source=255 with V_PRED:
	// - V_PRED predicts 127 (when no row above)
	// - Residual = 255 - 127 = 128
	// - After DCT and quantization, Y2[0] should capture this

	// Forward DCT: a 16x16 block with value 128 everywhere
	// Each 4x4 block gets DCT'd
	// DCT(128 everywhere) = [2048, 0, 0, ...] (DC = 128*16 = 2048)
	// Then Y2 gets the DC values from the 16 4x4 blocks
	// Y2 input: [2048, 0, 0, ...] since all blocks have same DC
	// Forward WHT: first element = 2048*4 = 8192 (sum of all 4 DC values in first column)
	// Wait, let me trace through the actual math...

	// Let's trace what the encoder does:
	residual := make([]int16, 16)
	for i := range residual {
		residual[i] = 128 // All pixels have residual of 128
	}

	// Forward DCT
	dctOut := make([]int16, 16)
	forwardDCT4(residual, dctOut)

	fmt.Printf("After DCT of 4x4 block with value 128:\n")
	for i := 0; i < 4; i++ {
		fmt.Printf("  %v\n", dctOut[i*4:i*4+4])
	}
	fmt.Printf("DC coefficient: %d\n", dctOut[0])

	// Now for Y2: we have 16 4x4 blocks, each with DC=dctOut[0]
	// The Y2 input is a 4x4 array of these DC values
	y2In := make([]int16, 16)
	for i := range y2In {
		y2In[i] = dctOut[0]
	}

	fmt.Printf("\nY2 input (DC values from 16 blocks): %v\n", y2In)

	// Forward WHT
	y2Out := make([]int16, 16)
	forwardWHT4(y2In, y2Out)

	fmt.Printf("After WHT:\n")
	for i := 0; i < 4; i++ {
		fmt.Printf("  %v\n", y2Out[i*4:i*4+4])
	}
	fmt.Printf("Y2[0] coefficient: %d\n", y2Out[0])

	// Now quantize
	// QI=24: Y2DC=46, Y2AC=60
	// Y2[0] / 46 = ?
	quantized := y2Out[0] / 46
	fmt.Printf("\nQuantized Y2[0] / 46 = %d\n", quantized)

	// Actually encoder multiplies by (1/quant) and rounds
	// Let's see what value encoder should produce

	// Now decode path:
	// 1. Dequantize: coeff * 46
	dequant := quantized * 46
	fmt.Printf("Dequantized: %d * 46 = %d\n", quantized, dequant)

	// 2. Inverse WHT to get DC values for each 4x4 block
	// Input: [dequant, 0, 0, ...]
	whtIn := make([]int16, 16)
	whtIn[0] = dequant

	dcValues := make([]int16, 16)
	inverseWHT4(whtIn, dcValues)

	fmt.Printf("\nAfter inverse WHT (DC values for each 4x4 block):\n")
	for i := 0; i < 4; i++ {
		fmt.Printf("  %v\n", dcValues[i*4:i*4+4])
	}

	// 3. Inverse DCT for each 4x4 block
	// Input: [dcValues[i], 0, 0, ...] for each block
	dctIn := make([]int16, 16)
	dctIn[0] = dcValues[0]

	residualOut := make([]int16, 16)
	inverseDCT4(dctIn, residualOut)

	fmt.Printf("\nAfter inverse DCT (residual values for 4x4 block):\n")
	for i := 0; i < 4; i++ {
		fmt.Printf("  %v\n", residualOut[i*4:i*4+4])
	}

	// 4. Add prediction (127)
	final := int(residualOut[0]) + 127
	fmt.Printf("\nFinal pixel: %d + 127 = %d\n", residualOut[0], final)

	// Clamp to 0-255
	if final < 0 {
		final = 0
	} else if final > 255 {
		final = 255
	}
	fmt.Printf("Clamped: %d\n", final)
}

func forwardDCT4(in, out []int16) {
	// Simple forward DCT
	var tmp [16]int32
	for i := 0; i < 4; i++ {
		a0 := int32(in[i*4+0])
		a1 := int32(in[i*4+1])
		a2 := int32(in[i*4+2])
		a3 := int32(in[i*4+3])

		b0 := a0 + a3
		b1 := a1 + a2
		b2 := a1 - a2
		b3 := a0 - a3

		tmp[0*4+i] = b0 + b1
		tmp[1*4+i] = (2*b3 + b2)
		tmp[2*4+i] = b0 - b1
		tmp[3*4+i] = (b3 - 2*b2)
	}

	for i := 0; i < 4; i++ {
		c0 := tmp[i*4+0]
		c1 := tmp[i*4+1]
		c2 := tmp[i*4+2]
		c3 := tmp[i*4+3]

		d0 := c0 + c3
		d1 := c1 + c2
		d2 := c1 - c2
		d3 := c0 - c3

		out[i*4+0] = int16((d0 + d1) >> 3)
		out[i*4+1] = int16((2*d3 + d2) >> 3)
		out[i*4+2] = int16((d0 - d1) >> 3)
		out[i*4+3] = int16((d3 - 2*d2) >> 3)
	}
}

func forwardWHT4(in, out []int16) {
	var tmp [16]int32
	for i := 0; i < 4; i++ {
		a0 := int32(in[i*4+0])
		a1 := int32(in[i*4+1])
		a2 := int32(in[i*4+2])
		a3 := int32(in[i*4+3])

		b0 := a0 + a3
		b1 := a1 + a2
		b2 := a1 - a2
		b3 := a0 - a3

		tmp[0*4+i] = b0 + b1
		tmp[1*4+i] = b3 + b2
		tmp[2*4+i] = b0 - b1
		tmp[3*4+i] = b3 - b2
	}

	for i := 0; i < 4; i++ {
		c0 := tmp[i*4+0]
		c1 := tmp[i*4+1]
		c2 := tmp[i*4+2]
		c3 := tmp[i*4+3]

		d0 := c0 + c3
		d1 := c1 + c2
		d2 := c1 - c2
		d3 := c0 - c3

		out[i*4+0] = int16((d0 + d1) >> 2)
		out[i*4+1] = int16((d3 + d2) >> 2)
		out[i*4+2] = int16((d0 - d1) >> 2)
		out[i*4+3] = int16((d3 - d2) >> 2)
	}
}

func inverseWHT4(in, out []int16) {
	var tmp [16]int32
	for i := 0; i < 4; i++ {
		a0 := int32(in[0*4+i])
		a1 := int32(in[1*4+i])
		a2 := int32(in[2*4+i])
		a3 := int32(in[3*4+i])

		b0 := a0 + a2
		b1 := a1 + a3
		b2 := a0 - a2
		b3 := a1 - a3

		tmp[i*4+0] = b0 + b1
		tmp[i*4+1] = b2 + b3
		tmp[i*4+2] = b2 - b3
		tmp[i*4+3] = b0 - b1
	}

	for i := 0; i < 4; i++ {
		c0 := tmp[i*4+0]
		c1 := tmp[i*4+1]
		c2 := tmp[i*4+2]
		c3 := tmp[i*4+3]

		d0 := c0 + c2
		d1 := c1 + c3
		d2 := c0 - c2
		d3 := c1 - c3

		out[i*4+0] = int16((d0 + d1 + 3) >> 3)
		out[i*4+1] = int16((d2 + d3 + 3) >> 3)
		out[i*4+2] = int16((d2 - d3 + 3) >> 3)
		out[i*4+3] = int16((d0 - d1 + 3) >> 3)
	}
}

func inverseDCT4(in, out []int16) {
	var tmp [16]int32
	for i := 0; i < 4; i++ {
		a0 := int32(in[0*4+i])
		a1 := int32(in[1*4+i])
		a2 := int32(in[2*4+i])
		a3 := int32(in[3*4+i])

		b0 := a0 + a2
		b1 := a0 - a2
		b2 := (a1 >> 1) - a3
		b3 := a1 + (a3 >> 1)

		tmp[i*4+0] = b0 + b3
		tmp[i*4+1] = b1 + b2
		tmp[i*4+2] = b1 - b2
		tmp[i*4+3] = b0 - b3
	}

	for i := 0; i < 4; i++ {
		c0 := tmp[i*4+0]
		c1 := tmp[i*4+1]
		c2 := tmp[i*4+2]
		c3 := tmp[i*4+3]

		d0 := c0 + c2
		d1 := c0 - c2
		d2 := (c1 >> 1) - c3
		d3 := c1 + (c3 >> 1)

		out[i*4+0] = int16((d0 + d3 + 4) >> 3)
		out[i*4+1] = int16((d1 + d2 + 4) >> 3)
		out[i*4+2] = int16((d1 - d2 + 4) >> 3)
		out[i*4+3] = int16((d0 - d3 + 4) >> 3)
	}
}
