package vp8

import (
	"testing"
)

func TestTraceReconstruction(t *testing.T) {
	// If decoder produces 103, what happened?

	// V_PRED gives pred=127
	// If final=103, residual = 103 - 127 = -24

	// Or if H_PRED gives pred=0
	// If final=103, residual = 103 - 0 = 103

	// Let's trace what residual=103 implies
	// If all 16 pixels in a 4x4 block are 103:
	// Inverse DCT input: DC coefficient only
	// For flat 103 residual: DC = 103 * 16 / some_factor

	// Actually, inverse DCT of [DC, 0, 0, ...] gives:
	// pixel = (DC + 4) / 8 for each position
	// So if pixel=103: DC = 103 * 8 - 4 = 820

	// And inverse WHT of [Y2DC, 0, ...]:
	// DC[i] = (Y2DC + 3) / 8 per block
	// So if DC=820: Y2DC = 820 * 8 - 3 = 6557

	// If Y2DC=6557 came from dequant:
	// Y2DC = coeff * dequant_factor
	// 6557 = coeff * dequant_factor

	// If coeff=178: factor = 6557/178 ≈ 36.8
	// If coeff=177: factor = 6557/177 ≈ 37.0

	// Actual factors for qi=24: Y2DC=46
	// qi=36 gives approximately factor=38

	t.Log("Working backwards from output=103:")
	t.Log("  If H_PRED (pred=0): residual=103")
	t.Log("  pixel=103 in flat block means DC coefficient")

	// Actually, let me test the inverse transforms
	var dctCoeffs [16]int16

	// What DC gives pixel 103 after inverse DCT?
	// InverseDCT output pixel = (DC + 4) >> 3 approximately
	// We want 103, so DC ≈ 103 * 8 - 4 = 820
	dctCoeffs[0] = 820
	pixels := InverseDCT4x4(dctCoeffs)
	t.Logf("  DC=820 gives pixels: %v", pixels[0:4])

	// Now what Y2DC gives block DC=820 after inverse WHT?
	var whtCoeffs [16]int16

	// InverseWHT output DC[i] ≈ (Y2DC + rounding) / something
	// Let me check by running inverse WHT
	for y2dc := int16(6000); y2dc < 7000; y2dc += 100 {
		whtCoeffs[0] = y2dc
		dcs := InverseWHT4x4(whtCoeffs)
		if dcs[0] >= 800 && dcs[0] <= 850 {
			t.Logf("  Y2DC=%d gives block DC=%d", y2dc, dcs[0])
		}
	}

	// Test with our actual Y2DC after dequant
	// Coefficient 178 * factor 46 = 8188
	whtCoeffs[0] = 8188
	dcs := InverseWHT4x4(whtCoeffs)
	dctCoeffs[0] = dcs[0]
	pixels = InverseDCT4x4(dctCoeffs)
	t.Logf("  Actual: Y2DC=8188 → block DC=%d → pixel=%d", dcs[0], pixels[0])
}
