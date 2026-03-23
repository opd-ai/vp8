package vp8

import (
	"testing"
)

func TestDCTInverseRoundtrip(t *testing.T) {
	// Test that forward DCT followed by inverse DCT approximately recovers input
	input := [16]int16{
		10, 20, 30, 40,
		50, 60, 70, 80,
		90, 100, 110, 120,
		130, 140, 150, 160,
	}

	dct := ForwardDCT4x4(input[:])
	idct := InverseDCT4x4(dct)

	// Check that we approximately recover the input (within quantization error)
	for i := 0; i < 16; i++ {
		diff := int(input[i]) - int(idct[i])
		if diff < -2 || diff > 2 {
			t.Errorf("DCT roundtrip error at %d: input=%d, output=%d, diff=%d",
				i, input[i], idct[i], diff)
		}
	}
}

func TestDCTZeroInput(t *testing.T) {
	input := make([]int16, 16)

	dct := ForwardDCT4x4(input)

	// Due to rounding constants in the fixed-point math, very small
	// non-zero values can appear. This matches libvpx behavior.
	for i, v := range dct {
		if v > 1 || v < -1 {
			t.Errorf("DCT of zeros: coefficient %d = %d, want ~0", i, v)
		}
	}
}

func TestDCTConstantInput(t *testing.T) {
	// Constant input should produce non-zero DC and near-zero AC
	input := [16]int16{
		100, 100, 100, 100,
		100, 100, 100, 100,
		100, 100, 100, 100,
		100, 100, 100, 100,
	}

	dct := ForwardDCT4x4(input[:])

	// DC coefficient should be non-zero
	if dct[0] == 0 {
		t.Error("DCT DC coefficient should be non-zero for constant input")
	}

	// AC coefficients should be near zero
	for i := 1; i < 16; i++ {
		if dct[i] > 2 || dct[i] < -2 {
			t.Errorf("DCT AC coefficient %d = %d, expected near zero", i, dct[i])
		}
	}
}

func TestWHTInverseRoundtrip(t *testing.T) {
	input := [16]int16{
		10, 20, 30, 40,
		50, 60, 70, 80,
		90, 100, 110, 120,
		130, 140, 150, 160,
	}

	wht := ForwardWHT4x4(input)
	iwht := InverseWHT4x4(wht)

	// WHT is designed to be lossless (integer-to-integer)
	for i := 0; i < 16; i++ {
		diff := int(input[i]) - int(iwht[i])
		if diff < -1 || diff > 1 {
			t.Errorf("WHT roundtrip error at %d: input=%d, output=%d, diff=%d",
				i, input[i], iwht[i], diff)
		}
	}
}

func TestWHTZeroInput(t *testing.T) {
	input := [16]int16{}

	wht := ForwardWHT4x4(input)

	for i, v := range wht {
		if v != 0 {
			t.Errorf("WHT of zeros: coefficient %d = %d, want 0", i, v)
		}
	}
}

func TestQuantizeBlock(t *testing.T) {
	coeffs := [16]int16{
		100, 50, 25, 12,
		-30, -15, -7, 3,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}

	quant := QuantizeBlock(coeffs, 10, 5)

	// DC: 100 / 10 = 10
	if quant[0] != 10 {
		t.Errorf("Quantize DC: got %d, want 10", quant[0])
	}

	// First AC: 50 / 5 = 10
	if quant[1] != 10 {
		t.Errorf("Quantize AC[1]: got %d, want 10", quant[1])
	}

	// Negative value: -30 / 5 = -6
	if quant[4] != -6 {
		t.Errorf("Quantize negative: got %d, want -6", quant[4])
	}

	// Zero stays zero
	if quant[8] != 0 {
		t.Errorf("Quantize zero: got %d, want 0", quant[8])
	}
}

func TestDequantizeBlock(t *testing.T) {
	quantized := [16]int16{
		10, 10, 5, 2,
		-6, -3, -1, 1,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}

	dequant := DequantizeBlock(quantized, 10, 5)

	// DC: 10 * 10 = 100
	if dequant[0] != 100 {
		t.Errorf("Dequantize DC: got %d, want 100", dequant[0])
	}

	// AC: 10 * 5 = 50
	if dequant[1] != 50 {
		t.Errorf("Dequantize AC[1]: got %d, want 50", dequant[1])
	}

	// Negative: -6 * 5 = -30
	if dequant[4] != -30 {
		t.Errorf("Dequantize negative: got %d, want -30", dequant[4])
	}
}

func TestQuantDequantRoundtrip(t *testing.T) {
	coeffs := [16]int16{
		100, 50, 30, 20,
		15, 10, 5, 3,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}

	dcQ, acQ := int16(8), int16(4)

	quant := QuantizeBlock(coeffs, dcQ, acQ)
	dequant := DequantizeBlock(quant, dcQ, acQ)

	// Dequantized values should be within 1 quantum of original
	for i := 0; i < 16; i++ {
		var q int16
		if i == 0 {
			q = dcQ
		} else {
			q = acQ
		}
		diff := coeffs[i] - dequant[i]
		if diff < -q || diff > q {
			t.Errorf("Quant/dequant error at %d: original=%d, dequant=%d, diff=%d",
				i, coeffs[i], dequant[i], diff)
		}
	}
}

func TestComputeResidual(t *testing.T) {
	src := make([]byte, 16)
	pred := make([]byte, 16)

	for i := range src {
		src[i] = 100
		pred[i] = 50
	}

	residual := ComputeResidual(src, pred)

	for i, r := range residual {
		if r != 50 {
			t.Errorf("Residual[%d] = %d, want 50", i, r)
		}
	}

	// Test with negative residual
	for i := range src {
		src[i] = 30
		pred[i] = 80
	}

	residual = ComputeResidual(src, pred)

	for i, r := range residual {
		if r != -50 {
			t.Errorf("Residual[%d] = %d, want -50", i, r)
		}
	}
}

func TestBlockHasNonZeroCoeffs(t *testing.T) {
	zeros := [16]int16{}
	if BlockHasNonZeroCoeffs(zeros) {
		t.Error("All zeros should return false")
	}

	nonzero := [16]int16{}
	nonzero[5] = 1
	if !BlockHasNonZeroCoeffs(nonzero) {
		t.Error("Block with non-zero should return true")
	}
}

func TestCountNonZeroCoeffs(t *testing.T) {
	coeffs := [16]int16{
		1, 0, 2, 0,
		0, 3, 0, 0,
		0, 0, 4, 0,
		0, 0, 0, 5,
	}

	count := CountNonZeroCoeffs(coeffs)
	if count != 5 {
		t.Errorf("CountNonZeroCoeffs = %d, want 5", count)
	}
}

func TestZigzagOrder(t *testing.T) {
	// Verify zigzag order matches VP8 spec
	expectedZigzag := [16]int{
		0, 1, 4, 8,
		5, 2, 3, 6,
		9, 12, 13, 10,
		7, 11, 14, 15,
	}

	for i, v := range ZigzagOrder {
		if v != expectedZigzag[i] {
			t.Errorf("ZigzagOrder[%d] = %d, want %d", i, v, expectedZigzag[i])
		}
	}
}

func TestToFromZigzag(t *testing.T) {
	raster := [16]int16{
		0, 1, 2, 3,
		4, 5, 6, 7,
		8, 9, 10, 11,
		12, 13, 14, 15,
	}

	zigzag := ToZigzag(raster)
	back := FromZigzag(zigzag)

	for i := 0; i < 16; i++ {
		if raster[i] != back[i] {
			t.Errorf("Zigzag roundtrip error at %d: %d != %d", i, raster[i], back[i])
		}
	}
}

func TestZigzagScanOrder(t *testing.T) {
	// Create raster with position as value
	raster := [16]int16{
		0, 1, 2, 3,
		4, 5, 6, 7,
		8, 9, 10, 11,
		12, 13, 14, 15,
	}

	zigzag := ToZigzag(raster)

	// First element should be (0,0)
	if zigzag[0] != 0 {
		t.Errorf("Zigzag[0] = %d, want 0", zigzag[0])
	}
	// Second should be (0,1)
	if zigzag[1] != 1 {
		t.Errorf("Zigzag[1] = %d, want 1", zigzag[1])
	}
	// Third should be (1,0)
	if zigzag[2] != 4 {
		t.Errorf("Zigzag[2] = %d, want 4", zigzag[2])
	}
	// Fourth should be (2,0)
	if zigzag[3] != 8 {
		t.Errorf("Zigzag[3] = %d, want 8", zigzag[3])
	}
}
