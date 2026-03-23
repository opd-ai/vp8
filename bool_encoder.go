// Package vp8 provides a pure-Go VP8 I-frame encoder.
package vp8

// vp8Norm is the normalization lookup table from libvpx (vpx_dsp/prob.c).
// Given a range value, it returns the number of bits to shift left.
var vp8Norm = [256]uint8{
	0, 7, 6, 6, 5, 5, 5, 5, 4, 4, 4, 4, 4, 4, 4, 4,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

// boolEncoder is a VP8 boolean arithmetic coder.
// It writes probability-weighted bits to a byte buffer.
// Implementation matches libvpx's boolhuff.h.
type boolEncoder struct {
	buf      []byte
	lowvalue uint32
	range_   uint32
	count    int
}

func newBoolEncoder() *boolEncoder {
	return &boolEncoder{range_: 255, lowvalue: 0, count: -24}
}

// putBit encodes a single boolean value with the given probability of being 0.
// prob is in [0, 255] representing P(bit=0) ≈ prob/256.
// Implementation follows libvpx's vp8_encode_bool exactly.
func (e *boolEncoder) putBit(prob uint8, bit bool) {
	split := 1 + (((e.range_ - 1) * uint32(prob)) >> 8)

	newRange := split
	if bit {
		e.lowvalue += split
		newRange = e.range_ - split
	}

	shift := int(vp8Norm[newRange])
	newRange <<= shift
	e.count += shift

	if e.count >= 0 {
		offset := shift - e.count

		// Check for carry propagation
		if (e.lowvalue<<(offset-1))&0x80000000 != 0 {
			// Carry propagation needed
			for i := len(e.buf) - 1; i >= 0 && e.buf[i] == 0xff; i-- {
				e.buf[i] = 0
			}
			if len(e.buf) > 0 {
				// Find first non-0xff byte and increment it
				for i := len(e.buf) - 1; i >= 0; i-- {
					if e.buf[i] != 0xff {
						e.buf[i]++
						break
					}
				}
			}
		}

		// Output byte
		e.buf = append(e.buf, uint8(e.lowvalue>>(24-offset)))

		// Update state for after output
		e.lowvalue = uint32((uint64(e.lowvalue) << offset) & 0xffffff)
		shift = e.count
		e.count -= 8
	}

	e.lowvalue <<= shift
	e.range_ = newRange
}

// putLiteral writes a fixed-probability (128/256 = 0.5) boolean.
func (e *boolEncoder) putLiteral(v uint32, n int) {
	for n > 0 {
		n--
		e.putBit(128, (v>>uint(n))&1 == 1)
	}
}

// flush finalises the boolean encoder output by writing trailing bits.
// Implementation follows libvpx's vp8_stop_encode.
func (e *boolEncoder) flush() []byte {
	// libvpx encodes 32 zeros with prob 128 to flush
	for i := 0; i < 32; i++ {
		e.putBit(128, false)
	}
	return e.buf
}
