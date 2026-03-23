package vp8

import (
	"testing"
)

// boolDecoder is a VP8 boolean arithmetic decoder
type boolDecoder struct {
	data    []byte
	bytePos int
	bitPos  int // bit within current byte (0-7)
	range_  uint32
	value   uint32
}

func newBoolDecoder(data []byte) *boolDecoder {
	p := &boolDecoder{
		data:   data,
		range_: 255,
	}
	if len(data) >= 2 {
		p.value = uint32(data[0])<<8 | uint32(data[1])
		p.bytePos = 2
	}
	return p
}

func (p *boolDecoder) readBit(prob uint8) bool {
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

	for p.range_ < 128 {
		p.value <<= 1
		p.range_ <<= 1
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

// decodeCoeff decodes a single coefficient using the VP8 token tree
func decodeCoeff(p *boolDecoder, probs [11]uint8) (int, bool) {
	// p[0]: EOB vs has-coeff
	if !p.readBit(probs[0]) {
		return 0, false // EOB
	}

	// p[1]: zero vs non-zero
	if !p.readBit(probs[1]) {
		return 0, true // zero coefficient
	}

	// p[2]: one vs more
	var v int
	if !p.readBit(probs[2]) {
		v = 1
	} else {
		// p[3]: 2/3/4 vs cats
		if !p.readBit(probs[3]) {
			// 2, 3, or 4
			if !p.readBit(probs[4]) {
				v = 2
			} else {
				if !p.readBit(probs[5]) {
					v = 3
				} else {
					v = 4
				}
			}
		} else {
			// categories
			if !p.readBit(probs[6]) {
				// cat1 or cat2
				if !p.readBit(probs[7]) {
					// cat1: 5-6
					v = 5 + btoi(p.readBit159())
				} else {
					// cat2: 7-10
					v = 7 + 2*btoi(p.readBit165()) + btoi(p.readBit145())
				}
			} else {
				// cat3-6
				b1 := p.readBitProb(probs[8])
				var b0 bool
				if b1 {
					b0 = p.readBitProb(probs[10])
				} else {
					b0 = p.readBitProb(probs[9])
				}
				cat := 2*btoi(b1) + btoi(b0)

				catProbs := [][]uint8{
					{173, 148, 140},           // cat3
					{176, 155, 140, 135},      // cat4
					{180, 157, 141, 134, 130}, // cat5
					{254, 254, 243, 230, 196, 177, 153, 140, 133, 130, 129}, // cat6
				}

				extra := 0
				for _, prob := range catProbs[cat] {
					extra = extra*2 + btoi(p.readBitProb(prob))
				}
				v = 3 + (8 << uint(cat)) + extra
			}
		}
	}

	// Sign bit
	if p.readBit(128) {
		v = -v
	}
	return v, true
}

func (p *boolDecoder) readBit159() bool            { return p.readBit(159) }
func (p *boolDecoder) readBit165() bool            { return p.readBit(165) }
func (p *boolDecoder) readBit145() bool            { return p.readBit(145) }
func (p *boolDecoder) readBitProb(prob uint8) bool { return p.readBit(prob) }

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestDecodeBothMBs(t *testing.T) {
	// Second partition: fefd28bafd778000
	data := []byte{0xfe, 0xfd, 0x28, 0xba, 0xfd, 0x77, 0x80, 0x00}
	p := newBoolDecoder(data)

	// Decode MB0 Y2 block
	probs := DefaultCoeffProbs[PlaneY2][0][0]
	v, hasMore := decodeCoeff(p, probs)
	t.Logf("MB0 Y2[0]: %d, hasMore=%v", v, hasMore)

	// After a non-zero coeff, check for EOB
	if hasMore {
		probs2 := DefaultCoeffProbs[PlaneY2][1][2] // band 1, ctx 2 (prev was >1)
		eob := !p.readBit(probs2[0])
		t.Logf("MB0 Y2 EOB after [0]: %v", eob)
	}

	// Now decode 16 Y blocks for MB0 (all should be EOB since firstCoeff=1 and all zeros)
	for i := 0; i < 16; i++ {
		yProbs := DefaultCoeffProbs[PlaneY1WithY2][1][0] // band 1 (coeff 1), ctx 0
		eob := !p.readBit(yProbs[0])
		if !eob {
			t.Logf("MB0 Y[%d] unexpected non-EOB!", i)
		}
	}
	t.Logf("MB0 Y blocks: all EOB")

	// 4 U blocks
	for i := 0; i < 4; i++ {
		uvProbs := DefaultCoeffProbs[PlaneUV][0][0]
		eob := !p.readBit(uvProbs[0])
		if !eob {
			t.Logf("MB0 U[%d] unexpected non-EOB!", i)
		}
	}
	t.Logf("MB0 U blocks: all EOB")

	// 4 V blocks
	for i := 0; i < 4; i++ {
		uvProbs := DefaultCoeffProbs[PlaneUV][0][0]
		eob := !p.readBit(uvProbs[0])
		if !eob {
			t.Logf("MB0 V[%d] unexpected non-EOB!", i)
		}
	}
	t.Logf("MB0 V blocks: all EOB")

	// Now decode MB1 Y2 block
	t.Logf("--- MB1 ---")
	probs = DefaultCoeffProbs[PlaneY2][0][0]
	v, hasMore = decodeCoeff(p, probs)
	t.Logf("MB1 Y2[0]: %d, hasMore=%v (expected: 178)", v, hasMore)
}
