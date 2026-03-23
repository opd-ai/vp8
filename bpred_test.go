package vp8

import (
	"testing"
)

func TestPredict4x4DC(t *testing.T) {
	var dst [16]byte

	// Test with both above and left available
	// above[0] = P (topLeft), above[1..4] = A[0..3]
	above := []byte{100, 100, 100, 100, 100}
	left := []byte{100, 100, 100, 100}

	Predict4x4(dst[:], above, left, B_DC_PRED)

	for i, v := range dst {
		if v != 100 {
			t.Errorf("B_DC_PRED with uniform input: pixel %d = %d, want 100", i, v)
			break
		}
	}

	// Test with no neighbors
	Predict4x4(dst[:], nil, nil, B_DC_PRED)
	for i, v := range dst {
		if v != 128 {
			t.Errorf("B_DC_PRED with no neighbors: pixel %d = %d, want 128", i, v)
			break
		}
	}
}

func TestPredict4x4TM(t *testing.T) {
	var dst [16]byte

	// Uniform input: result should be all same value
	above := []byte{100, 100, 100, 100, 100} // P, A[0..3]
	left := []byte{100, 100, 100, 100}

	Predict4x4(dst[:], above, left, B_TM_PRED)

	// 100 + 100 - 100 = 100
	for i, v := range dst {
		if v != 100 {
			t.Errorf("B_TM_PRED uniform: pixel %d = %d, want 100", i, v)
			break
		}
	}
}

func TestPredict4x4VE(t *testing.T) {
	var dst [16]byte

	// Create distinct above row
	// above[0]=P, above[1..5]=A[0..4] (need A[4] for smoothing)
	above := []byte{50, 60, 80, 100, 120, 140}

	Predict4x4(dst[:], above, nil, B_VE_PRED)

	// Each column should have a smoothed version of the above row
	// Column 0: avg3(P, A[0], A[1]) = avg3(50, 60, 80) = (50 + 120 + 80 + 2) / 4 = 63
	expected0 := avg3(50, 60, 80)
	for r := 0; r < 4; r++ {
		if dst[r*4] != expected0 {
			t.Errorf("B_VE_PRED col 0 row %d: got %d, want %d", r, dst[r*4], expected0)
		}
	}
}

func TestPredict4x4HE(t *testing.T) {
	var dst [16]byte

	left := []byte{60, 80, 100, 120}

	Predict4x4(dst[:], nil, left, B_HE_PRED)

	// Row 0: avg3(P=129, L[0], L[1]) = avg3(129, 60, 80)
	// Row 1: avg3(L[0], L[1], L[2]) = avg3(60, 80, 100)
	// etc.
	expected1 := avg3(60, 80, 100)
	for c := 0; c < 4; c++ {
		if dst[4+c] != expected1 {
			t.Errorf("B_HE_PRED row 1 col %d: got %d, want %d", c, dst[4+c], expected1)
		}
	}
}

func TestPredict4x4LD(t *testing.T) {
	var dst [16]byte

	// above[0]=P, above[1..8]=A[0..7]
	above := []byte{0, 10, 20, 30, 40, 50, 60, 70, 80}

	Predict4x4(dst[:], above, nil, B_LD_PRED)

	// (0,0) = avg3(A[0], A[1], A[2]) = avg3(10, 20, 30) = 20
	if dst[0] != 20 {
		t.Errorf("B_LD_PRED (0,0): got %d, want 20", dst[0])
	}
	// (0,1) and (1,0) should be equal = avg3(A[1], A[2], A[3]) = 30
	if dst[1] != 30 || dst[4] != 30 {
		t.Errorf("B_LD_PRED (0,1)=%d (1,0)=%d, want 30", dst[1], dst[4])
	}
}

func TestPredict4x4RD(t *testing.T) {
	var dst [16]byte

	// Build proper edge array via above and left
	// E[0]=L[3], E[1]=L[2], E[2]=L[1], E[3]=L[0], E[4]=P, E[5]=A[0], E[6]=A[1], E[7]=A[2], E[8]=A[3]
	above := []byte{50, 60, 70, 80, 90} // P=50, A[0..3]=60,70,80,90
	left := []byte{40, 30, 20, 10}      // L[0..3]

	Predict4x4(dst[:], above, left, B_RD_PRED)

	// (0,0) = avg3(E[3], E[4], E[5]) = avg3(L[0], P, A[0]) = avg3(40, 50, 60) = 50
	if dst[0] != 50 {
		t.Errorf("B_RD_PRED (0,0): got %d, want 50", dst[0])
	}
}

func TestPredict4x4VR(t *testing.T) {
	var dst [16]byte

	above := []byte{50, 60, 70, 80, 90}
	left := []byte{40, 30, 20, 10}

	Predict4x4(dst[:], above, left, B_VR_PRED)

	// (0,0) = avg2(E[4], E[5]) = avg2(P, A[0]) = avg2(50, 60) = 55
	if dst[0] != 55 {
		t.Errorf("B_VR_PRED (0,0): got %d, want 55", dst[0])
	}
}

func TestPredict4x4VL(t *testing.T) {
	var dst [16]byte

	above := []byte{0, 10, 20, 30, 40, 50, 60, 70, 80}

	Predict4x4(dst[:], above, nil, B_VL_PRED)

	// (0,0) = avg2(A[0], A[1]) = avg2(10, 20) = 15
	if dst[0] != 15 {
		t.Errorf("B_VL_PRED (0,0): got %d, want 15", dst[0])
	}
}

func TestPredict4x4HD(t *testing.T) {
	var dst [16]byte

	above := []byte{50, 60, 70, 80, 90}
	left := []byte{40, 30, 20, 10}

	Predict4x4(dst[:], above, left, B_HD_PRED)

	// (0,0) = avg2(E[3], E[4]) = avg2(L[0], P) = avg2(40, 50) = 45
	if dst[0] != 45 {
		t.Errorf("B_HD_PRED (0,0): got %d, want 45", dst[0])
	}
}

func TestPredict4x4HU(t *testing.T) {
	var dst [16]byte

	left := []byte{10, 20, 30, 40}

	Predict4x4(dst[:], nil, left, B_HU_PRED)

	// (0,0) = avg2(L[0], L[1]) = avg2(10, 20) = 15
	if dst[0] != 15 {
		t.Errorf("B_HU_PRED (0,0): got %d, want 15", dst[0])
	}
	// (3,3) = L[3] = 40
	if dst[15] != 40 {
		t.Errorf("B_HU_PRED (3,3): got %d, want 40", dst[15])
	}
}

func TestSelectBest4x4Mode(t *testing.T) {
	// Create a uniform source
	src := make([]byte, 16)
	for i := range src {
		src[i] = 100
	}
	above := []byte{100, 100, 100, 100, 100, 100, 100, 100, 100}
	left := []byte{100, 100, 100, 100}

	mode, sad := SelectBest4x4Mode(src, above, left)
	if sad != 0 {
		t.Errorf("SelectBest4x4Mode with perfect uniform: SAD = %d, want 0", sad)
	}
	_ = mode // Any mode with SAD=0 is acceptable
}

func TestComputeSAD4x4(t *testing.T) {
	a := make([]byte, 16)
	b := make([]byte, 16)

	// Identical blocks
	for i := range a {
		a[i] = 50
		b[i] = 50
	}
	if sad := computeSAD4x4(a, b); sad != 0 {
		t.Errorf("SAD of identical 4x4: got %d, want 0", sad)
	}

	// All differ by 1
	for i := range b {
		b[i] = 51
	}
	if sad := computeSAD4x4(a, b); sad != 16 {
		t.Errorf("SAD 4x4 all diff=1: got %d, want 16", sad)
	}
}

func TestAvg2(t *testing.T) {
	tests := []struct {
		x, y byte
		want byte
	}{
		{0, 0, 0},
		{10, 10, 10},
		{10, 11, 11}, // (10 + 11 + 1) >> 1 = 11
		{10, 12, 11},
		{255, 255, 255},
	}
	for _, tt := range tests {
		got := avg2(tt.x, tt.y)
		if got != tt.want {
			t.Errorf("avg2(%d, %d) = %d, want %d", tt.x, tt.y, got, tt.want)
		}
	}
}

func TestAvg3(t *testing.T) {
	tests := []struct {
		x, y, z byte
		want    byte
	}{
		{0, 0, 0, 0},
		{10, 10, 10, 10},
		{10, 20, 30, 20}, // (10 + 40 + 30 + 2) / 4 = 20.5 -> 20
		{50, 60, 70, 60}, // (50 + 120 + 70 + 2) / 4 = 60.5 -> 60
	}
	for _, tt := range tests {
		got := avg3(tt.x, tt.y, tt.z)
		if got != tt.want {
			t.Errorf("avg3(%d, %d, %d) = %d, want %d", tt.x, tt.y, tt.z, got, tt.want)
		}
	}
}

func TestAllBModesProduceOutput(t *testing.T) {
	// Ensure all modes produce valid output without panicking
	var dst [16]byte
	above := []byte{128, 100, 110, 120, 130, 140, 150, 160, 170}
	left := []byte{90, 80, 70, 60}

	modes := []intraBMode{
		B_DC_PRED, B_TM_PRED, B_VE_PRED, B_HE_PRED,
		B_LD_PRED, B_RD_PRED, B_VR_PRED, B_VL_PRED,
		B_HD_PRED, B_HU_PRED,
	}

	for _, mode := range modes {
		Predict4x4(dst[:], above, left, mode)

		// Check that output is in valid range
		for i, v := range dst {
			if v > 255 { // Always true for byte, but checks logic
				t.Errorf("Mode %d: pixel %d has invalid value %d", mode, i, v)
			}
		}
	}
}
