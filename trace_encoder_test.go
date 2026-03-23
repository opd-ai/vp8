package vp8

import (
	"fmt"
	"testing"
)

func TestTraceEncoder(t *testing.T) {
	// Trace what the actual encoder produces for MB1 (source=255)

	// Input: 16x16 block of value 255
	// V_PRED predicts 127
	// Residual = 255 - 127 = 128

	// Create residual block
	residual := make([]int16, 16)
	for i := range residual {
		residual[i] = 128
	}

	// Forward DCT
	dctOut := ForwardDCT4x4(residual)
	fmt.Printf("Forward DCT of residual[128]: %v\n", dctOut)
	fmt.Printf("DC coefficient: %d\n", dctOut[0])

	// All 16 blocks have the same DC
	y2In := [16]int16{}
	for i := range y2In {
		y2In[i] = dctOut[0]
	}

	// Forward WHT
	y2Out := ForwardWHT4x4(y2In)
	fmt.Printf("\nForward WHT of [%d x 16]: %v\n", dctOut[0], y2Out)
	fmt.Printf("Y2[0]: %d\n", y2Out[0])

	// Quantize Y2 (QI=24, Y2DC=46, Y2AC=60)
	quantized := QuantizeBlock(y2Out, 46, 60)
	fmt.Printf("\nQuantized Y2: %v\n", quantized)
	fmt.Printf("Quantized Y2[0]: %d\n", quantized[0])
}
