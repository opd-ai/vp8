package vp8

// This file implements VP8 coefficient token entropy coding.
// Reference: RFC 6386 §13

// Token types for DCT coefficient coding
const (
	DCT_0    = iota // 0 - End of block marker
	DCT_1           // 1
	DCT_2           // 2
	DCT_3           // 3
	DCT_4           // 4
	DCT_CAT1        // 5-6
	DCT_CAT2        // 7-10
	DCT_CAT3        // 11-18
	DCT_CAT4        // 19-34
	DCT_CAT5        // 35-66
	DCT_CAT6        // 67-2048+
	DCT_EOB         // End of block (no more non-zero coefficients)

	NumDCTTokens = 12
)

// Block types for coefficient probability selection (RFC 6386 §13.3)
// These correspond to the "plane" enumeration in the decoder:
//
//	0 = Y1 with Y2 (Y blocks when Y2 DC block exists)
//	1 = Y2 (the WHT-transformed DC values)
//	2 = UV (chroma blocks, both U and V)
//	3 = Y1 sans Y2 (Y blocks in B_PRED mode, no Y2 block)
const (
	PlaneY1WithY2 = 0 // Y blocks when Y2 exists
	PlaneY2       = 1 // Y2 (WHT of Y DC values)
	PlaneUV       = 2 // U/V chroma blocks
	PlaneY1SansY2 = 3 // Y blocks in B_PRED mode (no Y2)
)

// Legacy constants for backward compatibility (deprecated, use Plane* instead)
const (
	BlockTypeDCY  = PlaneY1WithY2 // Deprecated: use PlaneY1WithY2
	BlockTypeACY  = PlaneY2       // Deprecated: use PlaneY2
	BlockTypeDCUV = PlaneUV       // Deprecated: use PlaneUV
	BlockTypeACUV = PlaneY1SansY2 // Deprecated: use PlaneY1SansY2
)

// Coefficient bands for probability selection (RFC 6386 §13.3)
// Maps coefficient position (0-15) to band (0-7)
var coeffBand = [16]int{
	0, 1, 2, 3, 6, 4, 5, 6, 6, 6, 6, 6, 6, 6, 6, 7,
}

// DCT value category parameters (extra bits, base value)
var dctCatParams = [6]struct {
	bits int
	base int
}{
	{1, 5},   // CAT1: 5-6
	{2, 7},   // CAT2: 7-10
	{3, 11},  // CAT3: 11-18
	{4, 19},  // CAT4: 19-34
	{5, 35},  // CAT5: 35-66
	{11, 67}, // CAT6: 67-2048+
}

// Category probability tables for extra bits (RFC 6386 §13.2)
var catProbs = [6][]uint8{
	{159},                     // CAT1
	{165, 145},                // CAT2
	{173, 148, 140},           // CAT3
	{176, 155, 140, 135},      // CAT4
	{180, 157, 141, 134, 130}, // CAT5
	{254, 254, 243, 230, 196, 177, 153, 140, 133, 130, 129}, // CAT6
}

// DefaultCoeffProbs contains the default coefficient probability tables.
// Indexed as [block_type][band][context][token].
// From RFC 6386 §13.5.
var DefaultCoeffProbs = [4][8][3][11]uint8{
	// Block type 0: Y DC (after WHT) - uses band 0 only
	{
		{
			{128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128},
			{128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128},
			{128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128},
		},
		{
			{253, 136, 254, 255, 228, 219, 128, 128, 128, 128, 128},
			{189, 129, 242, 255, 227, 213, 255, 219, 128, 128, 128},
			{106, 126, 227, 252, 214, 209, 255, 255, 128, 128, 128},
		},
		{
			{1, 98, 248, 255, 236, 226, 255, 255, 128, 128, 128},
			{181, 133, 238, 254, 221, 234, 255, 154, 128, 128, 128},
			{78, 134, 202, 247, 198, 180, 255, 219, 128, 128, 128},
		},
		{
			{1, 185, 249, 255, 243, 255, 128, 128, 128, 128, 128},
			{184, 150, 247, 255, 236, 224, 128, 128, 128, 128, 128},
			{77, 110, 216, 255, 236, 230, 128, 128, 128, 128, 128},
		},
		{
			{1, 101, 251, 255, 241, 255, 128, 128, 128, 128, 128},
			{170, 139, 241, 252, 236, 209, 255, 255, 128, 128, 128},
			{37, 116, 196, 243, 228, 255, 255, 255, 128, 128, 128},
		},
		{
			{1, 204, 254, 255, 245, 255, 128, 128, 128, 128, 128},
			{207, 160, 250, 255, 238, 128, 128, 128, 128, 128, 128},
			{102, 103, 231, 255, 211, 171, 128, 128, 128, 128, 128},
		},
		{
			{1, 152, 252, 255, 240, 255, 128, 128, 128, 128, 128},
			{177, 135, 243, 255, 234, 225, 128, 128, 128, 128, 128},
			{80, 129, 211, 255, 194, 224, 128, 128, 128, 128, 128},
		},
		{
			{1, 1, 255, 128, 128, 128, 128, 128, 128, 128, 128},
			{246, 1, 255, 128, 128, 128, 128, 128, 128, 128, 128},
			{255, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128},
		},
	},
	// Block type 1: Y AC (and DC when not using WHT)
	{
		{
			{198, 35, 237, 223, 193, 187, 162, 160, 145, 155, 62},
			{131, 45, 198, 221, 172, 176, 220, 157, 252, 221, 1},
			{68, 47, 146, 208, 149, 167, 221, 162, 255, 223, 128},
		},
		{
			{1, 149, 241, 255, 221, 224, 255, 255, 128, 128, 128},
			{184, 141, 234, 253, 222, 220, 255, 199, 128, 128, 128},
			{81, 99, 181, 242, 176, 190, 249, 202, 255, 255, 128},
		},
		{
			{1, 129, 232, 253, 214, 197, 242, 196, 255, 255, 128},
			{99, 121, 210, 250, 201, 198, 255, 202, 128, 128, 128},
			{23, 91, 163, 242, 170, 187, 247, 210, 255, 255, 128},
		},
		{
			{1, 200, 246, 255, 234, 255, 128, 128, 128, 128, 128},
			{109, 178, 241, 255, 231, 245, 255, 255, 128, 128, 128},
			{44, 130, 201, 253, 205, 192, 255, 255, 128, 128, 128},
		},
		{
			{1, 132, 239, 251, 219, 209, 255, 165, 128, 128, 128},
			{94, 136, 225, 251, 218, 190, 255, 255, 128, 128, 128},
			{22, 100, 174, 245, 186, 161, 255, 199, 128, 128, 128},
		},
		{
			{1, 182, 249, 255, 232, 235, 128, 128, 128, 128, 128},
			{124, 143, 241, 255, 227, 234, 128, 128, 128, 128, 128},
			{35, 77, 181, 251, 193, 211, 255, 205, 128, 128, 128},
		},
		{
			{1, 157, 247, 255, 236, 231, 255, 255, 128, 128, 128},
			{121, 141, 235, 255, 225, 227, 255, 255, 128, 128, 128},
			{45, 99, 188, 251, 195, 217, 255, 224, 128, 128, 128},
		},
		{
			{1, 1, 251, 255, 213, 255, 128, 128, 128, 128, 128},
			{203, 1, 248, 255, 255, 128, 128, 128, 128, 128, 128},
			{137, 1, 177, 255, 224, 255, 128, 128, 128, 128, 128},
		},
	},
	// Block type 2: UV DC
	{
		{
			{253, 9, 248, 251, 207, 208, 255, 192, 128, 128, 128},
			{175, 13, 224, 243, 193, 185, 249, 198, 255, 255, 128},
			{73, 17, 171, 221, 161, 179, 236, 167, 255, 234, 128},
		},
		{
			{1, 95, 247, 253, 212, 183, 255, 255, 128, 128, 128},
			{239, 90, 244, 250, 211, 209, 255, 255, 128, 128, 128},
			{155, 77, 195, 248, 188, 195, 255, 255, 128, 128, 128},
		},
		{
			{1, 24, 239, 251, 218, 219, 255, 205, 128, 128, 128},
			{201, 51, 219, 255, 196, 186, 128, 128, 128, 128, 128},
			{69, 46, 190, 239, 201, 218, 255, 228, 128, 128, 128},
		},
		{
			{1, 191, 251, 255, 255, 128, 128, 128, 128, 128, 128},
			{223, 165, 249, 255, 213, 255, 128, 128, 128, 128, 128},
			{141, 124, 248, 255, 255, 128, 128, 128, 128, 128, 128},
		},
		{
			{1, 16, 248, 255, 255, 128, 128, 128, 128, 128, 128},
			{190, 36, 230, 255, 236, 255, 128, 128, 128, 128, 128},
			{149, 1, 255, 128, 128, 128, 128, 128, 128, 128, 128},
		},
		{
			{1, 226, 255, 128, 128, 128, 128, 128, 128, 128, 128},
			{247, 192, 255, 128, 128, 128, 128, 128, 128, 128, 128},
			{240, 128, 255, 128, 128, 128, 128, 128, 128, 128, 128},
		},
		{
			{1, 134, 252, 255, 255, 128, 128, 128, 128, 128, 128},
			{213, 62, 250, 255, 255, 128, 128, 128, 128, 128, 128},
			{55, 93, 255, 128, 128, 128, 128, 128, 128, 128, 128},
		},
		{
			{128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128},
			{128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128},
			{128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128},
		},
	},
	// Block type 3: UV AC
	{
		{
			{202, 24, 213, 235, 186, 191, 220, 160, 240, 175, 255},
			{126, 38, 182, 232, 169, 184, 228, 174, 255, 187, 128},
			{61, 46, 138, 219, 151, 178, 240, 170, 255, 216, 128},
		},
		{
			{1, 112, 230, 250, 199, 191, 247, 159, 255, 255, 128},
			{166, 109, 228, 252, 211, 215, 255, 174, 128, 128, 128},
			{39, 77, 162, 232, 172, 180, 245, 178, 255, 255, 128},
		},
		{
			{1, 52, 220, 246, 198, 199, 249, 220, 255, 255, 128},
			{124, 74, 191, 243, 183, 193, 250, 221, 255, 255, 128},
			{24, 71, 130, 219, 154, 170, 243, 182, 255, 255, 128},
		},
		{
			{1, 182, 225, 249, 219, 240, 255, 224, 128, 128, 128},
			{149, 150, 226, 252, 216, 205, 255, 171, 128, 128, 128},
			{28, 108, 170, 242, 183, 194, 254, 223, 255, 255, 128},
		},
		{
			{1, 81, 230, 252, 204, 203, 255, 192, 128, 128, 128},
			{123, 102, 209, 247, 188, 196, 255, 233, 128, 128, 128},
			{20, 95, 153, 243, 164, 173, 255, 203, 128, 128, 128},
		},
		{
			{1, 222, 248, 255, 216, 213, 128, 128, 128, 128, 128},
			{168, 175, 246, 252, 235, 205, 255, 255, 128, 128, 128},
			{47, 116, 215, 255, 211, 212, 255, 255, 128, 128, 128},
		},
		{
			{1, 121, 236, 253, 212, 214, 255, 255, 128, 128, 128},
			{141, 84, 213, 252, 201, 202, 255, 219, 128, 128, 128},
			{42, 80, 160, 240, 162, 185, 255, 205, 128, 128, 128},
		},
		{
			{1, 1, 255, 128, 128, 128, 128, 128, 128, 128, 128},
			{244, 1, 255, 128, 128, 128, 128, 128, 128, 128, 128},
			{238, 1, 255, 128, 128, 128, 128, 128, 128, 128, 128},
		},
	},
}

// TokenEncoder encodes DCT coefficient tokens into the boolean encoder.
type TokenEncoder struct {
	boolEnc    *boolEncoder
	coeffProbs *[4][8][3][11]uint8
}

// NewTokenEncoder creates a new token encoder.
func NewTokenEncoder(boolEnc *boolEncoder, probs *[4][8][3][11]uint8) *TokenEncoder {
	return &TokenEncoder{
		boolEnc:    boolEnc,
		coeffProbs: probs,
	}
}

// getContext returns the context (0, 1, or 2) based on the previous token.
// Context 0: previous was 0 or EOB
// Context 1: previous was 1
// Context 2: previous was > 1
func getContext(prevToken int) int {
	if prevToken == DCT_0 || prevToken == DCT_EOB {
		return 0
	}
	if prevToken == DCT_1 {
		return 1
	}
	return 2
}

// tokenFromValue converts an absolute coefficient value to a token type.
func tokenFromValue(v int) int {
	if v < 0 {
		v = -v
	}
	switch {
	case v == 0:
		return DCT_0
	case v == 1:
		return DCT_1
	case v == 2:
		return DCT_2
	case v == 3:
		return DCT_3
	case v == 4:
		return DCT_4
	case v <= 6:
		return DCT_CAT1
	case v <= 10:
		return DCT_CAT2
	case v <= 18:
		return DCT_CAT3
	case v <= 34:
		return DCT_CAT4
	case v <= 66:
		return DCT_CAT5
	default:
		return DCT_CAT6
	}
}

// encodeCoeffValue encodes a coefficient value (0 or non-zero) using the VP8 tree.
// This encodes just the value portion, without the EOB/continue decision.
// probs: probability table for [is_zero, is_one, ...]
// value: the coefficient value
// Returns the token type for context tracking.
func (te *TokenEncoder) encodeCoeffValue(probs *[11]uint8, value int16) int {
	absVal := int(value)
	if absVal < 0 {
		absVal = -absVal
	}

	token := tokenFromValue(int(value))

	// Encode the value tree (starting from p[1], not p[0])
	// p[1] = is non-zero
	if token == DCT_0 {
		te.boolEnc.putBit(probs[1], false) // 0 = zero
		return token
	}
	te.boolEnc.putBit(probs[1], true) // 1 = non-zero

	// p[2] = more than one
	if token == DCT_1 {
		te.boolEnc.putBit(probs[2], false) // 0 = one
	} else {
		te.boolEnc.putBit(probs[2], true) // 1 = more than one
		te.encodeValueTree(probs, token, absVal)
	}

	// Encode sign if non-zero
	if value < 0 {
		te.boolEnc.putBit(128, true) // negative
	} else {
		te.boolEnc.putBit(128, false) // positive
	}

	return token
}

// encodeValueTree encodes values >= 2 using the tree structure.
// This matches the decoder's tree in x/image/vp8 parseResiduals4:
//
//	p[3]: 2/3/4 vs cat1+ (values 5+)
//	  false: 2/3/4
//	    p[4]: 2 vs 3/4
//	      false: 2
//	      true: 3 or 4
//	        p[5]: extra bit for 3 vs 4
//	  true: cat1+ (values 5+)
//	    p[6]: cat1/cat2 vs cat3+
//	      false: cat1 or cat2
//	        p[7]: cat1 vs cat2
//	      true: cat3/4/5/6
//	        p[8]: cat3/4 vs cat5/6
//	        p[9] or p[10]: specific category
func (te *TokenEncoder) encodeValueTree(probs *[11]uint8, token, absVal int) {
	// p[3]: 2/3/4 vs cat1+
	if token <= DCT_4 {
		te.boolEnc.putBit(probs[3], false)
		// p[4]: 2 vs 3/4
		if token == DCT_2 {
			te.boolEnc.putBit(probs[4], false)
		} else {
			te.boolEnc.putBit(probs[4], true)
			// p[5]: 3 vs 4 (extra bit, 0=3, 1=4)
			te.boolEnc.putBit(probs[5], token == DCT_4)
		}
		return
	}
	te.boolEnc.putBit(probs[3], true) // cat1+

	// p[6]: cat1/cat2 vs cat3+
	if token <= DCT_CAT2 {
		te.boolEnc.putBit(probs[6], false)
		// p[7]: cat1 vs cat2
		if token == DCT_CAT1 {
			te.boolEnc.putBit(probs[7], false)
			te.encodeCatExtra(0, absVal-5)
		} else {
			te.boolEnc.putBit(probs[7], true)
			te.encodeCatExtra(1, absVal-7)
		}
		return
	}
	te.boolEnc.putBit(probs[6], true) // cat3+

	// p[8]: cat3/4 vs cat5/6
	if token <= DCT_CAT4 {
		te.boolEnc.putBit(probs[8], false)
		// p[9]: cat3 vs cat4
		if token == DCT_CAT3 {
			te.boolEnc.putBit(probs[9], false)
			te.encodeCatExtra(2, absVal-11)
		} else {
			te.boolEnc.putBit(probs[9], true)
			te.encodeCatExtra(3, absVal-19)
		}
		return
	}
	te.boolEnc.putBit(probs[8], true) // cat5/6

	// p[10]: cat5 vs cat6
	if token == DCT_CAT5 {
		te.boolEnc.putBit(probs[10], false)
		te.encodeCatExtra(4, absVal-35)
	} else {
		te.boolEnc.putBit(probs[10], true)
		te.encodeCatExtra(5, absVal-67)
	}
}

// EncodeToken encodes a single coefficient token.
// blockType: 0=Y_DC, 1=Y_AC, 2=UV_DC, 3=UV_AC
// coeffIdx: coefficient position in zigzag order (0-15)
// context: based on previous token
// Returns the token type for context tracking.
func (te *TokenEncoder) EncodeToken(blockType, coeffIdx, context int, value int16) int {
	band := coeffBand[coeffIdx]
	probs := &te.coeffProbs[blockType][band][context]

	absVal := int(value)
	if absVal < 0 {
		absVal = -absVal
	}

	token := tokenFromValue(int(value))

	// Encode the token using the probability tree
	te.encodeTokenTree(probs, token, absVal)

	// Encode sign if non-zero
	if absVal != 0 {
		if value < 0 {
			te.boolEnc.putBit(128, true) // negative
		} else {
			te.boolEnc.putBit(128, false) // positive
		}
	}

	return token
}

// encodeTokenTree encodes the token using the binary probability tree.
// The tree structure from RFC 6386 §13.2:
//
//	                    +-------------- DCT_EOB (11)
//	                    |
//	          +---------+
//	          |         |
//	          |         +-------------- DCT_0 (0)
//	+---------+
//	|         |         +-------------- DCT_1 (1)
//	|         |         |
//	|         +---------+
//	|                   |         +---- DCT_2 (2)
//	|                   |         |
//	|                   +---------+
//	|                             |
//	|                             +---- DCT_3 (3)
//
// encodeTokenTree encodes the token using the binary probability tree.
// This matches the decoder's tree in x/image/vp8 parseResiduals4.
// The tree structure handles DCT_EOB and DCT_0 first, then for non-zero values:
//
//	p[0]: EOB vs not-EOB
//	p[1]: zero vs non-zero
//	p[2]: one vs more-than-one
//	p[3]: 2/3/4 vs cat1+ (values 5+)
//	p[4]: 2 vs 3/4
//	p[5]: 3 vs 4 (extra bit)
//	p[6]: cat1/cat2 vs cat3+
//	p[7]: cat1 vs cat2
//	p[8]: cat3/4 vs cat5/6
//	p[9]: cat3 vs cat4
//	p[10]: cat5 vs cat6
func (te *TokenEncoder) encodeTokenTree(probs *[11]uint8, token, absVal int) {
	// Decision 0: Is this DCT_EOB or something else?
	if token == DCT_EOB {
		te.boolEnc.putBit(probs[0], false) // 0 = EOB
		return
	}
	te.boolEnc.putBit(probs[0], true) // 1 = not EOB

	// Decision 1: Is this DCT_0?
	if token == DCT_0 {
		te.boolEnc.putBit(probs[1], false) // 0 = zero
		return
	}
	te.boolEnc.putBit(probs[1], true) // 1 = non-zero

	// Decision 2: Is this DCT_1?
	if token == DCT_1 {
		te.boolEnc.putBit(probs[2], false) // 0 = one
		return
	}
	te.boolEnc.putBit(probs[2], true) // 1 = more than one

	// Decision 3: 2/3/4 vs cat1+ (values 5+)
	if token <= DCT_4 {
		te.boolEnc.putBit(probs[3], false)
		// p[4]: 2 vs 3/4
		if token == DCT_2 {
			te.boolEnc.putBit(probs[4], false)
		} else {
			te.boolEnc.putBit(probs[4], true)
			// p[5]: 3 vs 4 (extra bit)
			te.boolEnc.putBit(probs[5], token == DCT_4)
		}
		return
	}
	te.boolEnc.putBit(probs[3], true) // cat1+

	// Decision 4: cat1/cat2 vs cat3+
	if token <= DCT_CAT2 {
		te.boolEnc.putBit(probs[6], false)
		// p[7]: cat1 vs cat2
		if token == DCT_CAT1 {
			te.boolEnc.putBit(probs[7], false)
			te.encodeCatExtra(0, absVal-5)
		} else {
			te.boolEnc.putBit(probs[7], true)
			te.encodeCatExtra(1, absVal-7)
		}
		return
	}
	te.boolEnc.putBit(probs[6], true) // cat3+

	// Decision 5: cat3/4 vs cat5/6
	if token <= DCT_CAT4 {
		te.boolEnc.putBit(probs[8], false)
		// p[9]: cat3 vs cat4
		if token == DCT_CAT3 {
			te.boolEnc.putBit(probs[9], false)
			te.encodeCatExtra(2, absVal-11)
		} else {
			te.boolEnc.putBit(probs[9], true)
			te.encodeCatExtra(3, absVal-19)
		}
		return
	}
	te.boolEnc.putBit(probs[8], true)

	// p[10]: cat5 vs cat6
	if token == DCT_CAT5 {
		te.boolEnc.putBit(probs[10], false)
		te.encodeCatExtra(4, absVal-35)
	} else {
		te.boolEnc.putBit(probs[10], true)
		te.encodeCatExtra(5, absVal-67)
	}
}

// encodeCatExtra encodes the extra bits for category tokens.
func (te *TokenEncoder) encodeCatExtra(cat, extra int) {
	probs := catProbs[cat]
	for i, p := range probs {
		bit := (extra >> (len(probs) - 1 - i)) & 1
		te.boolEnc.putBit(p, bit == 1)
	}
}

// EncodeEOB encodes an end-of-block token.
func (te *TokenEncoder) EncodeEOB(blockType, coeffIdx, context int) {
	band := coeffBand[coeffIdx]
	probs := &te.coeffProbs[blockType][band][context]
	te.boolEnc.putBit(probs[0], false) // EOB
}

// EncodeBlock encodes all coefficients in a 4x4 block.
// coeffs: 16 DCT coefficients in zigzag order
// blockType: 0=Y_DC, 1=Y_AC, 2=UV_DC, 3=UV_AC
// firstCoeff: starting coefficient index (0 for DC, 1 for AC-only blocks)
// Returns true if any non-zero coefficients were encoded.
//
// VP8 coefficient encoding structure (per RFC 6386 §13.2):
// 1. For first coefficient: read p[0] for EOB/has-coeff
// 2. For each coefficient position: read p[1] for is-zero, then value bits if non-zero
// 3. After each coefficient except the last: read p[0] for continue/EOB
func (te *TokenEncoder) EncodeBlock(coeffs [16]int16, blockType, firstCoeff int) bool {
	// Find the last non-zero coefficient
	lastNonZero := -1
	for i := 15; i >= firstCoeff; i-- {
		if coeffs[i] != 0 {
			lastNonZero = i
			break
		}
	}

	// If all coefficients are zero, encode EOB at first position
	if lastNonZero < firstCoeff {
		te.EncodeEOB(blockType, firstCoeff, 0) // context 0 for first
		return false
	}

	// First coefficient position: write p[0]=true (has coefficients)
	band := coeffBand[firstCoeff]
	probs := &te.coeffProbs[blockType][band][0] // context 0 for first
	te.boolEnc.putBit(probs[0], true)           // not EOB, has coefficients

	// Encode all coefficients from firstCoeff to lastNonZero
	// The decoder reads p[0] only after non-zero coefficients (not after zeros).
	context := 0
	for i := firstCoeff; i <= lastNonZero; i++ {
		band = coeffBand[i]
		probs = &te.coeffProbs[blockType][band][context]

		// Encode the coefficient value
		token := te.encodeCoeffValue(probs, coeffs[i])
		context = getContext(token)

		// After a NON-ZERO coefficient, write the "continue" bit
		// (The decoder only reads p[0] after non-zero coefficients)
		if token != DCT_0 {
			if i < lastNonZero {
				// More coefficients to come: write p[0]=true
				nextBand := coeffBand[i+1]
				nextProbs := &te.coeffProbs[blockType][nextBand][context]
				te.boolEnc.putBit(nextProbs[0], true) // not EOB, continue
			} else if lastNonZero < 15 {
				// This was the last non-zero, encode EOB
				nextBand := coeffBand[lastNonZero+1]
				nextProbs := &te.coeffProbs[blockType][nextBand][context]
				te.boolEnc.putBit(nextProbs[0], false) // EOB
			}
			// If lastNonZero == 15, no EOB bit is needed (implicit)
		}
		// After a zero coefficient, the decoder doesn't read p[0],
		// it just continues to read p[1] for the next position.
	}

	return true
}

// CoeffProbUpdate holds the probability table update flags.
// Default probabilities used when no update is provided.
var CoeffProbUpdateProbs = [4][8][3][11]uint8{
	// Block type 0 (Y_DC after WHT)
	{
		{
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{176, 246, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{223, 241, 252, 255, 255, 255, 255, 255, 255, 255, 255},
			{249, 253, 253, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 244, 252, 255, 255, 255, 255, 255, 255, 255, 255},
			{234, 254, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{253, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 246, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{239, 253, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{254, 255, 254, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 248, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{251, 255, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 253, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{251, 254, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{254, 255, 254, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 254, 253, 255, 254, 255, 255, 255, 255, 255, 255},
			{250, 255, 254, 255, 254, 255, 255, 255, 255, 255, 255},
			{254, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
	},
	// Block type 1 (Y_AC) - simplified, using common update probabilities
	{
		{
			{217, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{225, 252, 241, 253, 255, 255, 254, 255, 255, 255, 255},
			{234, 250, 241, 250, 253, 255, 253, 254, 255, 255, 255},
		},
		{
			{255, 254, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{223, 254, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{238, 253, 254, 254, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 248, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{249, 254, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 253, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{247, 254, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 253, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{252, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 254, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{253, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 254, 253, 255, 255, 255, 255, 255, 255, 255, 255},
			{250, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{254, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
	},
	// Block type 2 (UV_DC) - simplified
	{
		{
			{186, 251, 250, 255, 255, 255, 255, 255, 255, 255, 255},
			{234, 251, 244, 254, 255, 255, 255, 255, 255, 255, 255},
			{251, 251, 243, 253, 254, 255, 254, 255, 255, 255, 255},
		},
		{
			{255, 253, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{236, 253, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{251, 253, 253, 254, 254, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 254, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{254, 254, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 254, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{254, 254, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{254, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{254, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
	},
	// Block type 3: Y1 sans Y2 (B_PRED mode, or plane 3 in decoder)
	// Must match decoder's tokenProbUpdateProb[3] exactly
	{
		{
			{248, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{250, 254, 252, 254, 255, 255, 255, 255, 255, 255, 255},
			{248, 254, 249, 253, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 253, 253, 255, 255, 255, 255, 255, 255, 255, 255},
			{246, 253, 253, 255, 255, 255, 255, 255, 255, 255, 255},
			{252, 254, 251, 254, 254, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 254, 252, 255, 255, 255, 255, 255, 255, 255, 255},
			{248, 254, 253, 255, 255, 255, 255, 255, 255, 255, 255},
			{253, 255, 254, 254, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 251, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{245, 251, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{253, 253, 254, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 251, 253, 255, 255, 255, 255, 255, 255, 255, 255},
			{252, 253, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 254, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 252, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{249, 255, 254, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 254, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 255, 253, 255, 255, 255, 255, 255, 255, 255, 255},
			{250, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{254, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
	},
}

// EncodeCoeffProbUpdates encodes coefficient probability updates into the frame header.
// This is called when refresh_entropy_probs is set and the encoder wants to
// communicate updated probabilities to the decoder.
//
// Reference: RFC 6386 §13.4
//
// currentProbs: the probabilities currently in use (will be updated)
// newProbs: the desired new probabilities
// enc: the boolean encoder for the frame header
//
// Returns true if any updates were made.
func EncodeCoeffProbUpdates(enc *boolEncoder, currentProbs, newProbs *[4][8][3][11]uint8) bool {
	anyUpdates := false

	for i := 0; i < 4; i++ {
		for j := 0; j < 8; j++ {
			for k := 0; k < 3; k++ {
				for l := 0; l < 11; l++ {
					newP := newProbs[i][j][k][l]
					oldP := currentProbs[i][j][k][l]

					// Probability of sending an update
					updateProb := CoeffProbUpdateProbs[i][j][k][l]

					if newP != oldP {
						// Signal that we have an update
						enc.putBit(updateProb, true)
						// Encode the new probability value (8 bits, literal)
						enc.putLiteral(uint32(newP), 8)
						currentProbs[i][j][k][l] = newP
						anyUpdates = true
					} else {
						// No update needed
						enc.putBit(updateProb, false)
					}
				}
			}
		}
	}

	return anyUpdates
}

// EncodeNoCoeffProbUpdates encodes that no coefficient probability updates
// are being made. This writes all "no update" flags.
func EncodeNoCoeffProbUpdates(enc *boolEncoder) {
	for i := 0; i < 4; i++ {
		for j := 0; j < 8; j++ {
			for k := 0; k < 3; k++ {
				for l := 0; l < 11; l++ {
					updateProb := CoeffProbUpdateProbs[i][j][k][l]
					enc.putBit(updateProb, false) // no update
				}
			}
		}
	}
}

// CopyCoeffProbs creates a deep copy of coefficient probability tables.
func CopyCoeffProbs(src *[4][8][3][11]uint8) [4][8][3][11]uint8 {
	var dst [4][8][3][11]uint8
	for i := 0; i < 4; i++ {
		for j := 0; j < 8; j++ {
			for k := 0; k < 3; k++ {
				copy(dst[i][j][k][:], src[i][j][k][:])
			}
		}
	}
	return dst
}
