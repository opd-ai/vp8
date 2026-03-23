package vp8

import (
	"testing"
)

func TestCoefficientMath(t *testing.T) {
	// Simulate what happens for Y=255 block with V_PRED
	// Prediction from above row (mby=0) is 127 for all positions
	// Residual = 255 - 127 = 128 per pixel

	// Create 4x4 residual block with all 128s
	var residual [16]int16
	for i := 0; i < 16; i++ {
		residual[i] = 128
	}
	t.Logf("Residual block: %v", residual)

	// Forward DCT
	dctOutput := ForwardDCT4x4(residual[:])
	t.Logf("DCT output: %v", dctOutput)

	// With V_PRED and Y2 present, the DC coefficient goes into Y2
	// For a flat block, only DC should be non-zero
	// DC coefficient = sum of all pixels * some factor

	// Now let's see what the Y2 WHT produces
	// For 16 blocks, each with DC=1024 (from flat 128 residual)
	var whtInput [16]int16
	for i := 0; i < 16; i++ {
		whtInput[i] = 1024 // DC from each 4x4 block
	}

	whtOutput := ForwardWHT4x4(whtInput)
	t.Logf("WHT output: %v", whtOutput)

	// What gets quantized?
	// Y2 coefficient 0 = whtOutput[0]
	// With qi=4, y2Quant = 7 (from quant.go)
	y2Quant := int16(7) // From deriveBestQi
	quantized := (whtOutput[0] + y2Quant/2) / y2Quant
	t.Logf("Quantized Y2[0]: %d", quantized)

	// Dequantized
	dequant := quantized * y2Quant
	t.Logf("Dequantized Y2[0]: %d", dequant)
}
