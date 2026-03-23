package vp8

import (
	"testing"
)

func TestClampQI(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{-10, 0},
		{0, 0},
		{64, 64},
		{127, 127},
		{200, 127},
	}
	for _, tt := range tests {
		got := clampQI(tt.input)
		if got != tt.want {
			t.Errorf("clampQI(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestDCQLookupTable(t *testing.T) {
	// Test table boundaries and key values from RFC 6386 §14.1
	tests := []struct {
		qi   int
		want int16
	}{
		{0, 4},     // First entry
		{1, 5},     // Second entry
		{7, 10},    // Value 10 appears twice
		{8, 11},    // Next after duplicate
		{127, 157}, // Last entry
		{96, 91},   // Jump point where values start increasing faster
	}
	for _, tt := range tests {
		got := dcQLookup[tt.qi]
		if got != tt.want {
			t.Errorf("dcQLookup[%d] = %d, want %d", tt.qi, got, tt.want)
		}
	}
}

func TestACQLookupTable(t *testing.T) {
	// Test table boundaries and key values from RFC 6386 §14.1
	tests := []struct {
		qi   int
		want int16
	}{
		{0, 4},     // First entry
		{53, 57},   // Linear portion ends here
		{54, 58},   // Still linear
		{55, 60},   // Gap starts (skips 59)
		{56, 62},   // Continues with larger steps
		{127, 284}, // Last entry
	}
	for _, tt := range tests {
		got := acQLookup[tt.qi]
		if got != tt.want {
			t.Errorf("acQLookup[%d] = %d, want %d", tt.qi, got, tt.want)
		}
	}
}

func TestGetQuantFactorsSimple(t *testing.T) {
	// Test with qi=0 (minimum quantization = best quality)
	qf := GetQuantFactorsSimple(0)
	if qf.Y1DC != 4 {
		t.Errorf("Y1DC at qi=0: got %d, want 4", qf.Y1DC)
	}
	if qf.Y1AC != 4 {
		t.Errorf("Y1AC at qi=0: got %d, want 4", qf.Y1AC)
	}
	// Y2DC should be scaled by 2 with minimum 8
	if qf.Y2DC != 8 {
		t.Errorf("Y2DC at qi=0: got %d, want 8 (min clamp)", qf.Y2DC)
	}
	// Y2AC should be scaled by 155/100 with minimum 8
	if qf.Y2AC != 8 {
		t.Errorf("Y2AC at qi=0: got %d, want 8 (min clamp)", qf.Y2AC)
	}
	if qf.UVDC != 4 {
		t.Errorf("UVDC at qi=0: got %d, want 4", qf.UVDC)
	}
	if qf.UVAC != 4 {
		t.Errorf("UVAC at qi=0: got %d, want 4", qf.UVAC)
	}

	// Test with qi=127 (maximum quantization = lowest quality)
	qf = GetQuantFactorsSimple(127)
	if qf.Y1DC != 157 {
		t.Errorf("Y1DC at qi=127: got %d, want 157", qf.Y1DC)
	}
	if qf.Y1AC != 284 {
		t.Errorf("Y1AC at qi=127: got %d, want 284", qf.Y1AC)
	}
	// Y2DC = 157 * 2 = 314
	if qf.Y2DC != 314 {
		t.Errorf("Y2DC at qi=127: got %d, want 314", qf.Y2DC)
	}
	// Y2AC = 284 * 155 / 100 = 440
	if qf.Y2AC != 440 {
		t.Errorf("Y2AC at qi=127: got %d, want 440", qf.Y2AC)
	}
	// UVDC is clamped to max 132
	if qf.UVDC != 132 {
		t.Errorf("UVDC at qi=127: got %d, want 132 (max clamp)", qf.UVDC)
	}
	if qf.UVAC != 284 {
		t.Errorf("UVAC at qi=127: got %d, want 284", qf.UVAC)
	}
}

func TestGetQuantFactorsWithDeltas(t *testing.T) {
	// Test with deltas
	qf := GetQuantFactors(64, 10, -5, 5, -10, 10)

	// Y1DC uses qi=74 (64+10)
	if qf.Y1DC != dcQLookup[74] {
		t.Errorf("Y1DC with delta: got %d, want %d", qf.Y1DC, dcQLookup[74])
	}
	// Y1AC uses qi=64 (no delta)
	if qf.Y1AC != acQLookup[64] {
		t.Errorf("Y1AC with delta: got %d, want %d", qf.Y1AC, acQLookup[64])
	}
	// Y2DC uses qi=59 (64-5), scaled by 2
	expectedY2DC := dcQLookup[59] * 2
	if qf.Y2DC != expectedY2DC {
		t.Errorf("Y2DC with delta: got %d, want %d", qf.Y2DC, expectedY2DC)
	}
	// UVDC uses qi=54 (64-10)
	if qf.UVDC != dcQLookup[54] {
		t.Errorf("UVDC with delta: got %d, want %d", qf.UVDC, dcQLookup[54])
	}
}

func TestQuantIndexToQpBackwardCompat(t *testing.T) {
	// Test that quantIndexToQp now uses the AC lookup table
	// and handles edge cases properly
	tests := []struct {
		qi      int
		wantMin int16
	}{
		{0, 4},   // Minimum value
		{24, 24}, // Typical default qi → should be 24 from AC table
		{127, 1}, // Maximum qi → should return valid value
	}
	for _, tt := range tests {
		got := quantIndexToQp(tt.qi)
		if got < tt.wantMin {
			t.Errorf("quantIndexToQp(%d) = %d, want >= %d", tt.qi, got, tt.wantMin)
		}
	}

	// Verify it returns AC table values for valid range
	for qi := 0; qi <= 127; qi++ {
		got := quantIndexToQp(qi)
		want := acQLookup[qi]
		if got != want {
			t.Errorf("quantIndexToQp(%d) = %d, want %d (from AC table)", qi, got, want)
		}
	}
}

func TestQuantFactorsY2MinClamp(t *testing.T) {
	// Y2 DC and AC have minimum values of 8
	for qi := 0; qi < 10; qi++ {
		qf := GetQuantFactorsSimple(qi)
		if qf.Y2DC < 8 {
			t.Errorf("Y2DC at qi=%d: got %d, want >= 8", qi, qf.Y2DC)
		}
		if qf.Y2AC < 8 {
			t.Errorf("Y2AC at qi=%d: got %d, want >= 8", qi, qf.Y2AC)
		}
	}
}

func TestQuantFactorsUVDCMaxClamp(t *testing.T) {
	// UV DC is clamped to maximum 132
	for qi := 100; qi <= 127; qi++ {
		qf := GetQuantFactorsSimple(qi)
		if qf.UVDC > 132 {
			t.Errorf("UVDC at qi=%d: got %d, want <= 132", qi, qf.UVDC)
		}
	}
}
