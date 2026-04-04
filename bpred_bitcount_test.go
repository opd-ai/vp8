package vp8

import (
	"fmt"
	"testing"
)

// TestBPredBitCount counts approximate bits for mode encoding.
func TestBPredBitCount(t *testing.T) {
	// Calculate expected bits for 16x32 (2 MBs)
	// MB 0: B_PRED
	// - skip bit: ~1 bit
	// - y_mode B_PRED: 1 bit (prob 145)
	// - 16 sub-block modes: each ~3-6 bits depending on context
	//   Let's say avg 4 bits each = 64 bits
	// - uv_mode DC: ~2 bits
	// Total MB0: ~68 bits

	// MB 1: V_PRED, skip=true
	// - skip bit: ~1 bit
	// - y_mode 16x16: 1 bit (prob 145), then 1 bit (prob 156), then 1 bit (prob 163) for V = ~3 bits
	// - uv_mode DC: ~2 bits
	// Total MB1: ~6 bits

	// Frame header:
	// - color_space, clamping_type: 2 bits
	// - segmentation_enabled: 1 bit
	// - loop filter: ~10-15 bits
	// - quantizer: ~10-15 bits
	// - token prob updates: ~40-50 bits
	// - mb_no_skip_coeff + prob_skip_false: 9 bits
	// Total header: ~70-90 bits

	// Total expected: 70 + 68 + 6 = 144 bits = 18 bytes
	// But we only have 7 bytes (56 bits) of mode data!

	fmt.Println("Expected partition 1 size: ~18-20 bytes")
	fmt.Println("Actual partition 1 size: 14 bytes (including 7-byte keyframe header)")
	fmt.Println("Mode data only: 7 bytes = 56 bits")
	fmt.Println("This is way too small!")
}
