package vp8

import (
	"testing"
)

// TestTokenEncodingDebug tests individual coefficient encoding/decoding
func TestTokenEncodingDebug(t *testing.T) {
	// Test encoding value 178 (cat6: 67-2048)
	// cat6 base = 67, so extra = 178 - 67 = 111
	// cat6 extra bits: 11 bits with probs [254, 254, 243, 230, 196, 177, 153, 140, 133, 130, 129]

	absVal := 178
	token := tokenFromValue(absVal)
	t.Logf("Value %d maps to token %d (expected DCT_CAT6=%d)", absVal, token, DCT_CAT6)

	// Create a mock encoder
	boolEnc := newBoolEncoder()

	probs := DefaultCoeffProbs
	te := NewTokenEncoder(boolEnc, &probs)

	// Use type 1 (Y2), band 0, context 0
	p := &te.coeffProbs[1][0][0]

	t.Logf("Probability table for Y2/band0/ctx0:")
	for i := 0; i < 11; i++ {
		t.Logf("  p[%d] = %d", i, p[i])
	}

	// Encode the value
	te.encodeCoeffValue(p, int16(absVal))

	// Get the encoded data
	data := boolEnc.flush()
	t.Logf("Encoded data: %x", data)
}
