package vp8

import (
	"testing"
)

func TestTraceSecondPartition(t *testing.T) {
	// Manually trace what should be in the second partition
	// for our 2-MB test image (Y=0, Y=255)

	// MB0 (Y=0, V_PRED):
	// - Y2 coeffs: [-177, 0, 0, ...]
	// - Y[0..15] coeffs: all zeros (DC is in Y2)
	// - U/V coeffs: zeros

	// MB1 (Y=255, V_PRED):
	// - Y2 coeffs: [178, 0, 0, ...]
	// - Y[0..15] coeffs: all zeros (DC is in Y2)
	// - U/V coeffs: zeros

	// Create the second partition manually
	residualEnc := newBoolEncoder()
	coeffProbs := DefaultCoeffProbs
	tokenEnc := NewTokenEncoder(residualEnc, &coeffProbs)

	// --- MB0 ---
	// Y2 block with [-177, 0, ...]
	var y2CoeffsMB0 [16]int16
	y2CoeffsMB0[0] = -177
	t.Logf("Encoding MB0 Y2 block: %v", y2CoeffsMB0)
	tokenEnc.EncodeBlock(y2CoeffsMB0, PlaneY2, 0)

	// 16 Y blocks with zeros (but starting from coeff 1 since DC is in Y2)
	var yCoeffZeros [16]int16
	for i := 0; i < 16; i++ {
		tokenEnc.EncodeBlock(yCoeffZeros, PlaneY1WithY2, 1) // firstCoeff=1
	}

	// 4 U blocks with zeros
	for i := 0; i < 4; i++ {
		tokenEnc.EncodeBlock(yCoeffZeros, PlaneUV, 0)
	}

	// 4 V blocks with zeros
	for i := 0; i < 4; i++ {
		tokenEnc.EncodeBlock(yCoeffZeros, PlaneUV, 0)
	}

	// --- MB1 ---
	// Y2 block with [178, 0, ...]
	var y2CoeffsMB1 [16]int16
	y2CoeffsMB1[0] = 178
	t.Logf("Encoding MB1 Y2 block: %v", y2CoeffsMB1)
	tokenEnc.EncodeBlock(y2CoeffsMB1, PlaneY2, 0)

	// 16 Y blocks with zeros
	for i := 0; i < 16; i++ {
		tokenEnc.EncodeBlock(yCoeffZeros, PlaneY1WithY2, 1)
	}

	// 4 U blocks with zeros
	for i := 0; i < 4; i++ {
		tokenEnc.EncodeBlock(yCoeffZeros, PlaneUV, 0)
	}

	// 4 V blocks with zeros
	for i := 0; i < 4; i++ {
		tokenEnc.EncodeBlock(yCoeffZeros, PlaneUV, 0)
	}

	secondPart := residualEnc.flush()
	t.Logf("Second partition (%d bytes): %x", len(secondPart), secondPart)
}
