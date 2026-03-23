package vp8

// correctPartition implements a bool decoder that matches x/image/vp8 exactly
type correctPartition struct {
	buf     []byte
	r       int
	rangeM1 uint32
	bits    uint32
	nBits   uint8
}

// Lookup tables from x/image/vp8
var lutShift = [127]uint8{
	7, 6, 6, 5, 5, 5, 5, 4, 4, 4, 4, 4, 4, 4, 4,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
}

var lutRangeM1 = [127]uint8{
	127, 127, 191, 127, 159, 191, 223, 127, 143, 159, 175, 191, 207, 223, 239,
	127, 135, 143, 151, 159, 167, 175, 183, 191, 199, 207, 215, 223, 231, 239, 247,
	127, 131, 135, 139, 143, 147, 151, 155, 159, 163, 167, 171, 175, 179, 183, 187,
	191, 195, 199, 203, 207, 211, 215, 219, 223, 227, 231, 235, 239, 243, 247, 251,
	127, 129, 131, 133, 135, 137, 139, 141, 143, 145, 147, 149, 151, 153, 155, 157,
	159, 161, 163, 165, 167, 169, 171, 173, 175, 177, 179, 181, 183, 185, 187, 189,
	191, 193, 195, 197, 199, 201, 203, 205, 207, 209, 211, 213, 215, 217, 219, 221,
	223, 225, 227, 229, 231, 233, 235, 237, 239, 241, 243, 245, 247, 249, 251, 253,
}

func newCorrectPartition(data []byte) *correctPartition {
	p := &correctPartition{
		buf:     data,
		rangeM1: 254,
	}
	// Read first byte
	if len(data) > 0 {
		p.bits = uint32(data[0]) << 8
		p.nBits = 8
		p.r = 1
	}
	return p
}

func (p *correctPartition) readBit(prob uint8) bool {
	// x/image/vp8 only normalizes when rangeM1 < 127
	// If rangeM1 >= 127, no normalization is needed

	split := (p.rangeM1 * uint32(prob)) >> 8

	if p.bits >= (split+1)<<8 {
		// Take the 1 branch
		p.rangeM1 -= split + 1
		p.bits -= (split + 1) << 8
		// Normalize only if needed
		if p.rangeM1 < 127 {
			s := lutShift[p.rangeM1]
			p.rangeM1 = uint32(lutRangeM1[p.rangeM1])
			p.bits <<= s
			p.nBits -= s
			if int8(p.nBits) < 0 {
				if p.r < len(p.buf) {
					p.bits |= uint32(p.buf[p.r]) << (8 - p.nBits)
					p.r++
				}
				p.nBits += 8
			}
		}
		return true
	}

	// Take the 0 branch
	p.rangeM1 = split
	// Normalize only if needed
	if p.rangeM1 < 127 {
		s := lutShift[p.rangeM1]
		p.rangeM1 = uint32(lutRangeM1[p.rangeM1])
		p.bits <<= s
		p.nBits -= s
		if int8(p.nBits) < 0 {
			if p.r < len(p.buf) {
				p.bits |= uint32(p.buf[p.r]) << (8 - p.nBits)
				p.r++
			}
			p.nBits += 8
		}
	}
	return false
}
