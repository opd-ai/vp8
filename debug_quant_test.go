package vp8

import (
	"testing"
)

func TestQuantFactorsForDefault(t *testing.T) {
	qf := GetQuantFactorsSimple(24)
	t.Logf("Y1DC: %d", qf.Y1DC)
	t.Logf("Y1AC: %d", qf.Y1AC)
	t.Logf("Y2DC: %d", qf.Y2DC)
	t.Logf("Y2AC: %d", qf.Y2AC)
	t.Logf("UVDC: %d", qf.UVDC)
	t.Logf("UVAC: %d", qf.UVAC)

	// Now test the full coefficient chain for Y=255 with V_PRED
	// Residual = 255 - 127 = 128 per pixel

	// DCT of flat 128 block → DC=1024
	var residual [16]int16
	for i := 0; i < 16; i++ {
		residual[i] = 128
	}
	dctOut := ForwardDCT4x4(residual[:])
	t.Logf("DCT output (flat 128): DC=%d", dctOut[0])

	// With 16 blocks each having DC=1024, WHT input is [1024 x 16]
	var whtIn [16]int16
	for i := 0; i < 16; i++ {
		whtIn[i] = 1024
	}
	whtOut := ForwardWHT4x4(whtIn)
	t.Logf("WHT output: [0]=%d", whtOut[0])

	// Quantize Y2[0] with Y2DC factor
	quantized := (whtOut[0] + qf.Y2DC/2) / qf.Y2DC
	t.Logf("Quantized Y2[0]: %d", quantized)

	// This quantized value is what gets encoded
	t.Logf("Token for %d: %d", quantized, tokenFromValue(int(quantized)))
}
