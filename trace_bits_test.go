package vp8

import (
	"testing"
)

func TestTraceBits(t *testing.T) {
	// Encode value 178 (cat6) and trace the bits
	boolEnc := newBoolEncoder()

	probs := DefaultCoeffProbs
	te := NewTokenEncoder(boolEnc, &probs)

	// Type 1 = Y2, band 0, context 0
	p := &te.coeffProbs[1][0][0]

	// First write p[0]=true (not EOB, has coefficients)
	boolEnc.putBit(p[0], true)

	// Now encode the coefficient value 178
	te.encodeCoeffValue(p, 178)

	data := boolEnc.flush()
	t.Logf("Encoded data: %x", data)

	// Now let's trace what the decoder should read:
	// p[0]=true (not EOB)
	// p[1]=true (non-zero)
	// p[2]=true (more than 1)
	// p[3]=true (cats)
	// p[6]=true (cat3+)
	// p[8]=true (cat5/6)
	// p[10]=true (cat6)
	// Then cat6 extra bits for 178-67=111 = 0b00001101111 (11 bits)
	// Then sign bit (positive)

	// cat6 probs: [254, 254, 243, 230, 196, 177, 153, 140, 133, 130, 129]
	// 111 in binary: 00001101111
	// extra bits from MSB to LSB: 0,0,0,0,1,1,0,1,1,1,1

	t.Logf("Expected bits for value 178:")
	t.Logf("  p[0]=true (not EOB)")
	t.Logf("  p[1]=true (non-zero), prob=%d", p[1])
	t.Logf("  p[2]=true (>1), prob=%d", p[2])
	t.Logf("  p[3]=true (cats), prob=%d", p[3])
	t.Logf("  p[6]=true (cat3+), prob=%d", p[6])
	t.Logf("  p[8]=true (cat5/6), prob=%d", p[8])
	t.Logf("  p[10]=true (cat6), prob=%d", p[10])
	t.Logf("  cat6 extra bits: 111 = 0b00001101111")
	t.Logf("  sign=false (positive)")
}
