package vp8

import (
	"testing"
)

func TestTraceYModeEncoding(t *testing.T) {
	// Test encoding V_PRED
	enc := newBoolEncoder()
	encodeYMode(enc, V_PRED)
	bits := enc.flush()
	t.Logf("V_PRED encoded to: %x", bits)

	// Expected: putBit(145, true), putBit(156, false), putBit(163, true)
	// Which bits get written depends on boolean encoder state

	// Now decode manually to verify
	// Start fresh encoder for each mode
	for _, mode := range []intraMode{DC_PRED, V_PRED, H_PRED, TM_PRED} {
		enc := newBoolEncoder()
		encodeYMode(enc, mode)
		bits := enc.flush()
		t.Logf("Mode %d encoded to: %x", mode, bits)
	}
}
