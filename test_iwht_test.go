package vp8

import (
	"testing"
)

func TestInverseWHT(t *testing.T) {
	// Y2[0]=178, dequantized = 178 * 46 = 8188
	// Other Y2 coeffs = 0

	var whtCoeffs [16]int16
	whtCoeffs[0] = 8188 // dequantized Y2[0]

	// Inverse WHT should give DC for each of the 16 4x4 blocks
	dc := InverseWHT4x4(whtCoeffs)
	t.Logf("Inverse WHT output: %v", dc)

	// Each DC value should be approximately 8188/16 = 512 (before rounding adjustments)
	// Actually WHT preserves energy, so output sum = input[0]

	// Now for one block with DC=512:
	// Inverse DCT of [512, 0, 0, ..., 0] should give flat block
	var dctCoeffs [16]int16
	dctCoeffs[0] = dc[0]

	pixels := InverseDCT4x4(dctCoeffs)
	t.Logf("One block pixels after inverse DCT: %v", pixels)

	// Prediction was 127 for V_PRED
	// Final = 127 + pixel_residual
	t.Logf("With V_PRED (pred=127), final pixels: %d", 127+int(pixels[0]))
}
