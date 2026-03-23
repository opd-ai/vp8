package vp8

import (
	"testing"
)

func TestTokenFromValue(t *testing.T) {
	tests := []struct {
		value    int
		expected int
	}{
		{0, DCT_0},
		{1, DCT_1},
		{-1, DCT_1},
		{2, DCT_2},
		{-2, DCT_2},
		{3, DCT_3},
		{4, DCT_4},
		{5, DCT_CAT1},
		{6, DCT_CAT1},
		{7, DCT_CAT2},
		{10, DCT_CAT2},
		{11, DCT_CAT3},
		{18, DCT_CAT3},
		{19, DCT_CAT4},
		{34, DCT_CAT4},
		{35, DCT_CAT5},
		{66, DCT_CAT5},
		{67, DCT_CAT6},
		{2048, DCT_CAT6},
	}

	for _, tt := range tests {
		got := tokenFromValue(tt.value)
		if got != tt.expected {
			t.Errorf("tokenFromValue(%d) = %d, want %d", tt.value, got, tt.expected)
		}
	}
}

func TestGetContext(t *testing.T) {
	tests := []struct {
		prevToken int
		expected  int
	}{
		{DCT_0, 0},
		{DCT_EOB, 0},
		{DCT_1, 1},
		{DCT_2, 2},
		{DCT_3, 2},
		{DCT_4, 2},
		{DCT_CAT1, 2},
		{DCT_CAT6, 2},
	}

	for _, tt := range tests {
		got := getContext(tt.prevToken)
		if got != tt.expected {
			t.Errorf("getContext(%d) = %d, want %d", tt.prevToken, got, tt.expected)
		}
	}
}

func TestCoeffBand(t *testing.T) {
	// Verify band mapping matches RFC 6386
	expectedBands := []int{0, 1, 2, 3, 6, 4, 5, 6, 6, 6, 6, 6, 6, 6, 6, 7}

	for i, expected := range expectedBands {
		if coeffBand[i] != expected {
			t.Errorf("coeffBand[%d] = %d, want %d", i, coeffBand[i], expected)
		}
	}
}

func TestTokenEncoder(t *testing.T) {
	// Create a simple test with known coefficients
	boolEnc := newBoolEncoder()
	probs := DefaultCoeffProbs
	te := NewTokenEncoder(boolEnc, &probs)

	// Encode a simple block with some coefficients
	coeffs := [16]int16{10, 5, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	hasNonZero := te.EncodeBlock(coeffs, BlockTypeACY, 0)

	if !hasNonZero {
		t.Error("EncodeBlock should return true for non-zero coefficients")
	}

	// Finalize the encoder
	data := boolEnc.flush()

	// Check that some data was produced
	if len(data) == 0 {
		t.Error("Expected encoded data, got empty")
	}
}

func TestTokenEncoderAllZeros(t *testing.T) {
	boolEnc := newBoolEncoder()
	probs := DefaultCoeffProbs
	te := NewTokenEncoder(boolEnc, &probs)

	// All zero coefficients
	coeffs := [16]int16{}

	hasNonZero := te.EncodeBlock(coeffs, BlockTypeACY, 0)

	if hasNonZero {
		t.Error("EncodeBlock should return false for all-zero coefficients")
	}

	data := boolEnc.flush()
	if len(data) == 0 {
		t.Error("Expected EOB data")
	}
}

func TestTokenEncoderNegativeValues(t *testing.T) {
	boolEnc := newBoolEncoder()
	probs := DefaultCoeffProbs
	te := NewTokenEncoder(boolEnc, &probs)

	// Coefficients with negative values
	coeffs := [16]int16{-10, 5, -2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	hasNonZero := te.EncodeBlock(coeffs, BlockTypeACY, 0)

	if !hasNonZero {
		t.Error("EncodeBlock should return true")
	}

	data := boolEnc.flush()
	if len(data) == 0 {
		t.Error("Expected encoded data")
	}
}

func TestTokenEncoderLargeValue(t *testing.T) {
	boolEnc := newBoolEncoder()
	probs := DefaultCoeffProbs
	te := NewTokenEncoder(boolEnc, &probs)

	// Coefficient with CAT6 value
	coeffs := [16]int16{500, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	hasNonZero := te.EncodeBlock(coeffs, BlockTypeACY, 0)

	if !hasNonZero {
		t.Error("EncodeBlock should return true")
	}

	data := boolEnc.flush()
	if len(data) == 0 {
		t.Error("Expected encoded data")
	}
}

func TestTokenEncoderFirstCoeff(t *testing.T) {
	boolEnc := newBoolEncoder()
	probs := DefaultCoeffProbs
	te := NewTokenEncoder(boolEnc, &probs)

	// Test with firstCoeff=1 (AC only, skip DC)
	coeffs := [16]int16{100, 5, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	hasNonZero := te.EncodeBlock(coeffs, BlockTypeACY, 1)

	if !hasNonZero {
		t.Error("EncodeBlock should return true (AC values present)")
	}

	boolEnc.flush()
}

func TestDefaultCoeffProbs(t *testing.T) {
	// Verify the structure of default probs
	for bt := 0; bt < 4; bt++ {
		for band := 0; band < 8; band++ {
			for ctx := 0; ctx < 3; ctx++ {
				for tok := 0; tok < 11; tok++ {
					p := DefaultCoeffProbs[bt][band][ctx][tok]
					if p < 1 || p > 255 {
						t.Errorf("Invalid probability at [%d][%d][%d][%d]: %d",
							bt, band, ctx, tok, p)
					}
				}
			}
		}
	}
}

func TestBlockTypes(t *testing.T) {
	// Verify block type constants
	if BlockTypeDCY != 0 {
		t.Errorf("BlockTypeDCY = %d, want 0", BlockTypeDCY)
	}
	if BlockTypeACY != 1 {
		t.Errorf("BlockTypeACY = %d, want 1", BlockTypeACY)
	}
	if BlockTypeDCUV != 2 {
		t.Errorf("BlockTypeDCUV = %d, want 2", BlockTypeDCUV)
	}
	if BlockTypeACUV != 3 {
		t.Errorf("BlockTypeACUV = %d, want 3", BlockTypeACUV)
	}
}

func TestCopyCoeffProbs(t *testing.T) {
	src := DefaultCoeffProbs
	dst := CopyCoeffProbs(&src)

	// Verify copy is independent
	src[0][0][0][0] = 100

	if dst[0][0][0][0] == 100 {
		t.Error("CopyCoeffProbs should create independent copy")
	}

	// Verify contents match original default
	if dst[0][0][0][0] != 128 {
		t.Errorf("Copy should have original value 128, got %d", dst[0][0][0][0])
	}
}

func TestEncodeNoCoeffProbUpdates(t *testing.T) {
	enc := newBoolEncoder()
	EncodeNoCoeffProbUpdates(enc)
	data := enc.flush()

	// Should produce some encoded data
	if len(data) == 0 {
		t.Error("EncodeNoCoeffProbUpdates should produce output")
	}
}

func TestEncodeCoeffProbUpdates(t *testing.T) {
	enc := newBoolEncoder()
	current := CopyCoeffProbs(&DefaultCoeffProbs)
	newProbs := CopyCoeffProbs(&DefaultCoeffProbs)

	// Change one probability
	newProbs[0][1][0][0] = 200

	hasUpdates := EncodeCoeffProbUpdates(enc, &current, &newProbs)

	if !hasUpdates {
		t.Error("Should report updates when probability changed")
	}

	// Current should be updated
	if current[0][1][0][0] != 200 {
		t.Errorf("Current prob should be updated to 200, got %d", current[0][1][0][0])
	}

	data := enc.flush()
	if len(data) == 0 {
		t.Error("Should produce output")
	}
}

func TestEncodeCoeffProbUpdatesNoChange(t *testing.T) {
	enc := newBoolEncoder()
	current := CopyCoeffProbs(&DefaultCoeffProbs)
	newProbs := CopyCoeffProbs(&DefaultCoeffProbs)

	// No changes
	hasUpdates := EncodeCoeffProbUpdates(enc, &current, &newProbs)

	if hasUpdates {
		t.Error("Should report no updates when probabilities unchanged")
	}
}

func TestCoeffProbUpdateProbs(t *testing.T) {
	// Verify update probability table structure
	for bt := 0; bt < 4; bt++ {
		for band := 0; band < 8; band++ {
			for ctx := 0; ctx < 3; ctx++ {
				for tok := 0; tok < 11; tok++ {
					p := CoeffProbUpdateProbs[bt][band][ctx][tok]
					if p < 1 {
						t.Errorf("Invalid update probability at [%d][%d][%d][%d]: %d",
							bt, band, ctx, tok, p)
					}
				}
			}
		}
	}
}
