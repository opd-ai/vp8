package vp8

import (
	"testing"
)

func TestTraceModeSelection(t *testing.T) {
	// MB1 source: all 255
	var src [256]byte
	for i := range src {
		src[i] = 255
	}

	// MB1's left neighbor (MB0) is all 0
	var left [16]byte
	for i := range left {
		left[i] = 0
	}

	// MB1's above row is frame boundary (first row of image)
	// Frame boundary uses 127
	var above [16]byte
	for i := range above {
		above[i] = 127 // or should it be 0?
	}

	topLeft := byte(127) // frame boundary corner

	mode, sad := SelectBest16x16Mode(src[:], above[:], left[:], topLeft)
	t.Logf("Selected mode: %d (DC_PRED=0, V_PRED=1, H_PRED=2, TM_PRED=3), SAD: %d", mode, sad)

	// What prediction does each mode produce?
	var pred [256]byte

	Predict16x16(pred[:], above[:], left[:], topLeft, DC_PRED)
	t.Logf("DC_PRED prediction: %d (uniform)", pred[0])

	Predict16x16(pred[:], above[:], left[:], topLeft, V_PRED)
	t.Logf("V_PRED prediction: %d (uniform)", pred[0])

	Predict16x16(pred[:], above[:], left[:], topLeft, H_PRED)
	t.Logf("H_PRED prediction: %d (uniform)", pred[0])

	Predict16x16(pred[:], above[:], left[:], topLeft, TM_PRED)
	t.Logf("TM_PRED prediction: %d (uniform)", pred[0])
}
