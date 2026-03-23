package vp8

import (
	"testing"
)

func TestPredict16x16DC(t *testing.T) {
	var dst [256]byte

	// Test with both above and left available
	above := make([]byte, 16)
	left := make([]byte, 16)
	for i := range above {
		above[i] = 100
	}
	for i := range left {
		left[i] = 100
	}

	Predict16x16(dst[:], above, left, 100, DC_PRED)

	// All pixels should be 100 (average of 32 pixels all equal to 100)
	for i, v := range dst {
		if v != 100 {
			t.Errorf("DC_PRED with uniform input: pixel %d = %d, want 100", i, v)
			break
		}
	}

	// Test with no neighbors (top-left corner)
	Predict16x16(dst[:], nil, nil, 0, DC_PRED)
	for i, v := range dst {
		if v != 128 {
			t.Errorf("DC_PRED with no neighbors: pixel %d = %d, want 128", i, v)
			break
		}
	}
}

func TestPredict16x16V(t *testing.T) {
	var dst [256]byte

	// Create a gradient above row
	above := make([]byte, 16)
	for i := range above {
		above[i] = byte(i * 16)
	}

	Predict16x16(dst[:], above, nil, 0, V_PRED)

	// Each column should have the same value as the above row
	for r := 0; r < 16; r++ {
		for c := 0; c < 16; c++ {
			want := above[c]
			got := dst[r*16+c]
			if got != want {
				t.Errorf("V_PRED: pixel (%d,%d) = %d, want %d", r, c, got, want)
			}
		}
	}
}

func TestPredict16x16H(t *testing.T) {
	var dst [256]byte

	// Create a gradient left column
	left := make([]byte, 16)
	for i := range left {
		left[i] = byte(i * 16)
	}

	Predict16x16(dst[:], nil, left, 0, H_PRED)

	// Each row should have the same value as the left column
	for r := 0; r < 16; r++ {
		for c := 0; c < 16; c++ {
			want := left[r]
			got := dst[r*16+c]
			if got != want {
				t.Errorf("H_PRED: pixel (%d,%d) = %d, want %d", r, c, got, want)
			}
		}
	}
}

func TestPredict16x16TM(t *testing.T) {
	var dst [256]byte

	// Simple test: above=100, left=100, topLeft=100
	// Result should be: 100 + 100 - 100 = 100 for all pixels
	above := make([]byte, 16)
	left := make([]byte, 16)
	for i := range above {
		above[i] = 100
		left[i] = 100
	}

	Predict16x16(dst[:], above, left, 100, TM_PRED)

	for i, v := range dst {
		if v != 100 {
			t.Errorf("TM_PRED uniform: pixel %d = %d, want 100", i, v)
			break
		}
	}

	// Test with gradient: above increases, left increases
	// TM should produce a 2D gradient
	for i := range above {
		above[i] = byte(50 + i*2)
		left[i] = byte(50 + i*2)
	}
	topLeft := byte(50)

	Predict16x16(dst[:], above, left, topLeft, TM_PRED)

	// Check corners
	// (0,0): left[0] + above[0] - topLeft = 50 + 50 - 50 = 50
	if dst[0] != 50 {
		t.Errorf("TM_PRED (0,0) = %d, want 50", dst[0])
	}
	// (15,15): left[15] + above[15] - topLeft = 80 + 80 - 50 = 110
	if dst[255] != 110 {
		t.Errorf("TM_PRED (15,15) = %d, want 110", dst[255])
	}
}

func TestPredict16x16TMClamping(t *testing.T) {
	var dst [256]byte

	// Test clamping: values that would exceed 255 or go below 0
	above := make([]byte, 16)
	left := make([]byte, 16)
	for i := range above {
		above[i] = 250
		left[i] = 250
	}
	topLeft := byte(100)

	Predict16x16(dst[:], above, left, topLeft, TM_PRED)

	// 250 + 250 - 100 = 400 → clamped to 255
	for i, v := range dst {
		if v != 255 {
			t.Errorf("TM_PRED clamping high: pixel %d = %d, want 255", i, v)
			break
		}
	}

	// Test low clamping
	for i := range above {
		above[i] = 10
		left[i] = 10
	}
	topLeft = 200

	Predict16x16(dst[:], above, left, topLeft, TM_PRED)

	// 10 + 10 - 200 = -180 → clamped to 0
	for i, v := range dst {
		if v != 0 {
			t.Errorf("TM_PRED clamping low: pixel %d = %d, want 0", i, v)
			break
		}
	}
}

func TestPredict8x8ChromaDC(t *testing.T) {
	var dst [64]byte

	above := make([]byte, 8)
	left := make([]byte, 8)
	for i := range above {
		above[i] = 80
		left[i] = 80
	}

	Predict8x8Chroma(dst[:], above, left, 80, DC_PRED_CHROMA)

	for i, v := range dst {
		if v != 80 {
			t.Errorf("Chroma DC_PRED: pixel %d = %d, want 80", i, v)
			break
		}
	}
}

func TestPredict8x8ChromaV(t *testing.T) {
	var dst [64]byte

	above := make([]byte, 8)
	for i := range above {
		above[i] = byte(i * 30)
	}

	Predict8x8Chroma(dst[:], above, nil, 0, V_PRED_CHROMA)

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			want := above[c]
			got := dst[r*8+c]
			if got != want {
				t.Errorf("Chroma V_PRED: pixel (%d,%d) = %d, want %d", r, c, got, want)
			}
		}
	}
}

func TestPredict8x8ChromaH(t *testing.T) {
	var dst [64]byte

	left := make([]byte, 8)
	for i := range left {
		left[i] = byte(i * 30)
	}

	Predict8x8Chroma(dst[:], nil, left, 0, H_PRED_CHROMA)

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			want := left[r]
			got := dst[r*8+c]
			if got != want {
				t.Errorf("Chroma H_PRED: pixel (%d,%d) = %d, want %d", r, c, got, want)
			}
		}
	}
}

func TestPredict8x8ChromaTM(t *testing.T) {
	var dst [64]byte

	above := make([]byte, 8)
	left := make([]byte, 8)
	for i := range above {
		above[i] = 100
		left[i] = 100
	}

	Predict8x8Chroma(dst[:], above, left, 100, TM_PRED_CHROMA)

	for i, v := range dst {
		if v != 100 {
			t.Errorf("Chroma TM_PRED: pixel %d = %d, want 100", i, v)
			break
		}
	}
}

func TestSelectBest16x16Mode(t *testing.T) {
	// Create a source that's uniform - DC_PRED should win
	src := make([]byte, 256)
	for i := range src {
		src[i] = 100
	}
	above := make([]byte, 16)
	left := make([]byte, 16)
	for i := range above {
		above[i] = 100
		left[i] = 100
	}

	mode, sad := SelectBest16x16Mode(src, above, left, 100)
	if sad != 0 {
		t.Errorf("SelectBest16x16Mode with perfect match: SAD = %d, want 0", sad)
	}
	// DC_PRED should be perfect match
	if mode != DC_PRED {
		t.Logf("Mode selected: %v (SAD=0 expected for DC_PRED)", mode)
	}

	// Create a vertically striped source - V_PRED should be best
	for r := 0; r < 16; r++ {
		for c := 0; c < 16; c++ {
			src[r*16+c] = byte(c * 16)
		}
	}
	for i := range above {
		above[i] = byte(i * 16)
	}

	mode, sad = SelectBest16x16Mode(src, above, nil, 0)
	if mode != V_PRED {
		t.Errorf("SelectBest16x16Mode with vertical stripes: mode = %v, want V_PRED", mode)
	}
	if sad != 0 {
		t.Errorf("SelectBest16x16Mode with vertical stripes: SAD = %d, want 0", sad)
	}
}

func TestSelectBest8x8ChromaMode(t *testing.T) {
	// Uniform source
	src := make([]byte, 64)
	above := make([]byte, 8)
	left := make([]byte, 8)
	for i := range src {
		src[i] = 100
	}
	for i := range above {
		above[i] = 100
		left[i] = 100
	}

	mode, sad := SelectBest8x8ChromaMode(src, above, left, 100)
	if sad != 0 {
		t.Errorf("SelectBest8x8ChromaMode with perfect match: SAD = %d, want 0", sad)
	}
	_ = mode // Any mode with SAD=0 is acceptable
}

func TestComputeSAD16x16(t *testing.T) {
	a := make([]byte, 256)
	b := make([]byte, 256)

	// Identical blocks
	for i := range a {
		a[i] = 100
		b[i] = 100
	}
	if sad := computeSAD16x16(a, b); sad != 0 {
		t.Errorf("SAD of identical blocks: got %d, want 0", sad)
	}

	// All differ by 1
	for i := range b {
		b[i] = 101
	}
	if sad := computeSAD16x16(a, b); sad != 256 {
		t.Errorf("SAD with all diff=1: got %d, want 256", sad)
	}

	// Half differ by 2
	for i := 0; i < 128; i++ {
		b[i] = 98
	}
	for i := 128; i < 256; i++ {
		b[i] = 100
	}
	if sad := computeSAD16x16(a, b); sad != 256 {
		t.Errorf("SAD with half diff=2: got %d, want 256", sad)
	}
}

func TestClamp8(t *testing.T) {
	tests := []struct {
		input int
		want  byte
	}{
		{-100, 0},
		{0, 0},
		{128, 128},
		{255, 255},
		{300, 255},
	}
	for _, tt := range tests {
		got := clamp8(tt.input)
		if got != tt.want {
			t.Errorf("clamp8(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
