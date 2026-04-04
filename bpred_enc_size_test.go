package vp8

import (
	"fmt"
	"testing"
)

// TestBPredEncSize traces encoder output size.
func TestBPredEncSize(t *testing.T) {
	// Manually encode some bits and check size
	enc := newBoolEncoder()

	// Encode 100 bits with prob 128 (50/50)
	for i := 0; i < 100; i++ {
		enc.putBit(128, i%2 == 0)
	}

	result := enc.flush()
	fmt.Printf("100 bits (50/50): %d bytes\n", len(result))

	// Encode B_PRED sub-block modes (16 modes, each ~4 bits)
	enc2 := newBoolEncoder()

	// Simulate encoding 16 B_DC_PRED modes
	// B_DC_PRED is prob[0]=false, which is highly probable
	for i := 0; i < 16; i++ {
		enc2.putBit(231, false) // B_DC_PRED in most common context
	}

	result2 := enc2.flush()
	fmt.Printf("16 B_DC_PRED modes: %d bytes\n", len(result2))

	// Encode 16 B_TM_PRED modes (2 bits each)
	enc3 := newBoolEncoder()
	for i := 0; i < 16; i++ {
		enc3.putBit(231, true)  // not DC
		enc3.putBit(180, false) // TM
	}

	result3 := enc3.flush()
	fmt.Printf("16 B_TM_PRED modes: %d bytes\n", len(result3))
}
