package vp8

import (
	"testing"
)

// fullMockPartition is a complete bool decoder for tracing
type fullMockPartition struct {
	data    []byte
	bytePos int
	bitPos  int // bit within current byte (0-7)
	range_  uint32
	value   uint32
}

func newFullMockPartition(data []byte) *fullMockPartition {
	p := &fullMockPartition{
		data:   data,
		range_: 255,
	}
	// Load first 2 bytes
	if len(data) >= 2 {
		p.value = uint32(data[0])<<8 | uint32(data[1])
		p.bytePos = 2
	}
	return p
}

func (p *fullMockPartition) readBit(prob uint8) bool {
	split := 1 + (((p.range_ - 1) * uint32(prob)) >> 8)
	bigsplit := split << 8

	var bit bool
	if p.value >= bigsplit {
		bit = true
		p.range_ -= split
		p.value -= bigsplit
	} else {
		bit = false
		p.range_ = split
	}

	// Renormalize
	for p.range_ < 128 {
		p.value <<= 1
		p.range_ <<= 1
		// Read next bit
		if p.bytePos < len(p.data) {
			p.value |= uint32((p.data[p.bytePos] >> uint(7-p.bitPos)) & 1)
			p.bitPos++
			if p.bitPos == 8 {
				p.bitPos = 0
				p.bytePos++
			}
		}
	}
	return bit
}

func (p *fullMockPartition) readUint(prob uint8, n int) uint32 {
	var v uint32
	for i := 0; i < n; i++ {
		if p.readBit(prob) {
			v |= 1 << uint(n-1-i)
		}
	}
	return v
}

func TestDecodeFullCoeff(t *testing.T) {
	// Second partition: fefd28bafd778000
	data := []byte{0xfe, 0xfd, 0x28, 0xba, 0xfd, 0x77, 0x80, 0x00}
	p := newFullMockPartition(data)

	probs := DefaultCoeffProbs[PlaneY2][0][0]
	t.Logf("Y2 band0 ctx0 probs: %v", probs)

	// Read MB0 Y2[0]
	hasCoeff := p.readBit(probs[0])
	t.Logf("p[0]=%v (has_coeff)", hasCoeff)

	nonZero := p.readBit(probs[1])
	t.Logf("p[1]=%v (non_zero)", nonZero)

	moreThanOne := p.readBit(probs[2])
	t.Logf("p[2]=%v (>1)", moreThanOne)

	cats := p.readBit(probs[3])
	t.Logf("p[3]=%v (cats)", cats)

	cat3plus := p.readBit(probs[6])
	t.Logf("p[6]=%v (cat3+)", cat3plus)

	cat56 := p.readBit(probs[8])
	t.Logf("p[8]=%v (cat5/6)", cat56)

	cat6 := p.readBit(probs[10])
	t.Logf("p[10]=%v (cat6)", cat6)

	// Read cat6 extra bits using cat6 probability table
	cat6Probs := []uint8{254, 254, 243, 230, 196, 177, 153, 140, 133, 130, 129}
	extra := uint32(0)
	for i, prob := range cat6Probs {
		bit := p.readBit(prob)
		if bit {
			extra |= 1 << uint(10-i)
		}
		t.Logf("cat6 extra bit %d: %v (prob=%d)", i, bit, prob)
	}

	value := 67 + int(extra)
	t.Logf("Extra bits: %d (0b%011b)", extra, extra)
	t.Logf("Value before sign: %d", value)

	// Read sign bit
	sign := p.readBit(128)
	t.Logf("Sign bit: %v", sign)

	if sign {
		value = -value
	}
	t.Logf("Final decoded value: %d (expected: -177)", value)
}
