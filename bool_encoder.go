// Package vp8 provides a pure-Go VP8 I-frame encoder.
package vp8

// boolEncoder is a VP8 boolean arithmetic coder.
// It writes probability-weighted bits to a byte buffer.
type boolEncoder struct {
	buf    []byte
	range_ uint32
	value  uint32
	count  int
}

func newBoolEncoder() *boolEncoder {
	return &boolEncoder{range_: 255, value: 0, count: -24}
}

// putBit encodes a single boolean value with the given probability of being 0.
// prob is in [0, 255] representing P(bit=0) ≈ prob/256.
func (e *boolEncoder) putBit(prob uint8, bit bool) {
	split := 1 + (((e.range_ - 1) * uint32(prob)) >> 8)
	if !bit {
		e.range_ = split
	} else {
		e.value += split
		e.range_ -= split
	}
	for e.range_ < 128 {
		e.range_ <<= 1
		e.count++
		e.value <<= 1
		if e.count == 0 {
			e.buf = append(e.buf, uint8(e.value>>16))
			e.value &= 0xffff
			e.count = -8
		}
	}
}

// putLiteral writes a fixed-probability (128/256 = 0.5) boolean.
func (e *boolEncoder) putLiteral(v uint32, n int) {
	for n > 0 {
		n--
		e.putBit(128, (v>>uint(n))&1 == 1)
	}
}

// flush finalises the boolean encoder output.
func (e *boolEncoder) flush() []byte {
	v := e.value
	for i := 0; i < 32; i++ {
		if e.count >= 0 {
			e.buf = append(e.buf, uint8(v>>16))
			v <<= 8
			v &= 0xffffff
			e.count -= 8
		}
		v <<= 1
		e.count++
	}
	return e.buf
}
