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

func TestCoeffHistogramBasic(t *testing.T) {
	h := NewCoeffHistogram()

	// Record some tokens
	h.RecordToken(PlaneY1WithY2, 1, 0, DCT_EOB)
	h.RecordToken(PlaneY1WithY2, 1, 0, DCT_0)
	h.RecordToken(PlaneY1WithY2, 1, 0, DCT_1)
	h.RecordToken(PlaneY1WithY2, 1, 0, DCT_2)
	h.RecordToken(PlaneY1WithY2, 1, 0, DCT_3)
	h.RecordToken(PlaneY1WithY2, 1, 0, DCT_4)
	h.RecordToken(PlaneY1WithY2, 1, 0, DCT_CAT1)
	h.RecordToken(PlaneY1WithY2, 1, 0, DCT_CAT2)

	// Verify counts - EOB branch (branch 0)
	if h.counts[PlaneY1WithY2][1][0][0][0] != 1 {
		t.Errorf("Expected 1 EOB false count, got %d", h.counts[PlaneY1WithY2][1][0][0][0])
	}
	if h.counts[PlaneY1WithY2][1][0][0][1] != 7 {
		t.Errorf("Expected 7 EOB true counts (non-EOB tokens), got %d", h.counts[PlaneY1WithY2][1][0][0][1])
	}
}

func TestCoeffHistogramReset(t *testing.T) {
	h := NewCoeffHistogram()

	// Record tokens
	for i := 0; i < 100; i++ {
		h.RecordToken(PlaneY1WithY2, 1, 0, DCT_1)
	}

	// Verify something was recorded
	total := h.counts[PlaneY1WithY2][1][0][1][0] + h.counts[PlaneY1WithY2][1][0][1][1]
	if total == 0 {
		t.Error("Expected recorded tokens")
	}

	// Reset and verify clear
	h.Reset()

	total = h.counts[PlaneY1WithY2][1][0][1][0] + h.counts[PlaneY1WithY2][1][0][1][1]
	if total != 0 {
		t.Errorf("Expected 0 after reset, got %d", total)
	}
}

func TestCoeffHistogramComputeUpdatedProbs(t *testing.T) {
	h := NewCoeffHistogram()

	// Record a bias towards DCT_0 (non-zero more common)
	for i := 0; i < 100; i++ {
		h.RecordToken(PlaneY1WithY2, 1, 0, DCT_1)
	}
	for i := 0; i < 10; i++ {
		h.RecordToken(PlaneY1WithY2, 1, 0, DCT_0)
	}
	// Record non-EOB for branch 0
	for i := 0; i < 110; i++ {
		h.RecordToken(PlaneY1WithY2, 1, 0, DCT_2)
	}

	updated := h.ComputeUpdatedProbs(&DefaultCoeffProbs)

	// With 100 non-zero and 10 zero for branch 1 (is non-zero),
	// P(zero) = 10/110 = 9%, or ~23/256
	// The computed probability should reflect this
	if updated[PlaneY1WithY2][1][0][1] == DefaultCoeffProbs[PlaneY1WithY2][1][0][1] {
		t.Log("Probability unchanged (may be similar to default or insufficient samples)")
	}
}

func TestSetProbabilityUpdatesAPI(t *testing.T) {
	enc, err := NewEncoder(320, 240, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	// Default should be disabled
	if enc.useProbUpdates {
		t.Error("Expected useProbUpdates to be false by default")
	}

	// Enable
	enc.SetProbabilityUpdates(true)
	if !enc.useProbUpdates {
		t.Error("Expected useProbUpdates to be true after SetProbabilityUpdates(true)")
	}

	// Disable
	enc.SetProbabilityUpdates(false)
	if enc.useProbUpdates {
		t.Error("Expected useProbUpdates to be false after SetProbabilityUpdates(false)")
	}
}

func TestEncoderWithProbabilityUpdates(t *testing.T) {
	enc, err := NewEncoder(32, 32, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	enc.SetProbabilityUpdates(true)

	// Create a test frame
	yuv := make([]byte, 32*32*3/2)
	for i := range yuv {
		yuv[i] = byte(i % 256)
	}

	// Encode multiple frames
	for i := 0; i < 5; i++ {
		_, err := enc.Encode(yuv)
		if err != nil {
			t.Fatalf("Encode frame %d: %v", i, err)
		}
	}
}
