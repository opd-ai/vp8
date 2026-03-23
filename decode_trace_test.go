package vp8

import (
	"testing"
)

// mockPartition is a minimal bool decoder for tracing
type mockPartition struct {
	data   []byte
	bitPos int
	range_ uint32
	value  uint32
}

func newMockPartition(data []byte) *mockPartition {
	p := &mockPartition{
		data:   data,
		range_: 255,
		value:  0,
	}
	// Initialize value from first bytes
	if len(data) >= 2 {
		p.value = uint32(data[0])<<8 | uint32(data[1])
	}
	p.bitPos = 16
	return p
}

func (p *mockPartition) readBit(prob uint8) bool {
	split := 1 + (((p.range_ - 1) * uint32(prob)) >> 8)

	if p.value < (split << 8) {
		p.range_ = split
		result := false
		p.normalize()
		return result
	} else {
		p.range_ -= split
		p.value -= split << 8
		result := true
		p.normalize()
		return result
	}
}

func (p *mockPartition) normalize() {
	for p.range_ < 128 {
		p.range_ <<= 1
		p.value <<= 1

		bytePos := p.bitPos / 8
		bitInByte := uint(p.bitPos % 8)
		if bytePos < len(p.data) {
			p.value |= uint32((p.data[bytePos] >> (7 - bitInByte)) & 1)
		}
		p.bitPos++
	}
}

func TestDecodeTrace(t *testing.T) {
	// Second partition: fefd28bafd778000
	data := []byte{0xfe, 0xfd, 0x28, 0xba, 0xfd, 0x77, 0x80, 0x00}
	p := newMockPartition(data)

	// Decode MB0 Y2 block
	// First read p[0] (EOB vs has-coeff)
	probs := &DefaultCoeffProbs[PlaneY2][0][0] // band 0, context 0
	t.Logf("Y2 band0 ctx0 probs: %v", probs)

	hasCoeff := p.readBit(probs[0])
	t.Logf("MB0 Y2 p[0]=%v (EOB=%v), prob=%d", hasCoeff, !hasCoeff, probs[0])

	if hasCoeff {
		// p[1]: zero vs non-zero
		nonZero := p.readBit(probs[1])
		t.Logf("MB0 Y2 p[1]=%v (non-zero=%v), prob=%d", nonZero, nonZero, probs[1])

		if nonZero {
			// p[2]: one vs more
			moreThanOne := p.readBit(probs[2])
			t.Logf("MB0 Y2 p[2]=%v (>1=%v), prob=%d", moreThanOne, moreThanOne, probs[2])

			if moreThanOne {
				// p[3]: 2/3/4 vs cats
				cats := p.readBit(probs[3])
				t.Logf("MB0 Y2 p[3]=%v (cats=%v), prob=%d", cats, cats, probs[3])

				if cats {
					// p[6]: cat1/2 vs cat3+
					cat3plus := p.readBit(probs[6])
					t.Logf("MB0 Y2 p[6]=%v (cat3+=%v), prob=%d", cat3plus, cat3plus, probs[6])

					if cat3plus {
						// p[8]: cat3/4 vs cat5/6
						cat56 := p.readBit(probs[8])
						t.Logf("MB0 Y2 p[8]=%v (cat5/6=%v), prob=%d", cat56, cat56, probs[8])

						if cat56 {
							// p[10]: cat5 vs cat6
							cat6 := p.readBit(probs[10])
							t.Logf("MB0 Y2 p[10]=%v (cat6=%v), prob=%d", cat6, cat6, probs[10])
						}
					}
				}
			}
		}
	}
}
