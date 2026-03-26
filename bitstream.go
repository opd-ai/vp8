package vp8

import (
	"encoding/binary"
	"errors"
)

var (
	errInvalidDimensions = errors.New("vp8: invalid frame dimensions (must be positive and even)")
	errInvalidFrameSize  = errors.New("vp8: frame buffer too small for given dimensions")
)

// vp8KeyFrameStartCode is the 3-byte marker that identifies a VP8 key frame.
var vp8KeyFrameStartCode = [3]byte{0x9D, 0x01, 0x2A}

// QuantDeltas holds the per-plane quantizer delta values for frame encoding.
type QuantDeltas struct {
	Y1DC int // Y1 DC coefficient delta
	Y2DC int // Y2 DC coefficient delta
	Y2AC int // Y2 AC coefficient delta
	UVDC int // Chroma DC coefficient delta
	UVAC int // Chroma AC coefficient delta
}

// encodeDelta encodes a quantizer delta value in the VP8 frame header.
// If delta is 0, only a "not present" bit is written.
// If delta is non-zero, a "present" bit is written, followed by the
// magnitude (4 bits) and sign (1 bit).
// Reference: RFC 6386 §9.6
func encodeDelta(enc *boolEncoder, delta int) {
	if delta == 0 {
		enc.putBit(128, false) // delta_present = false
		return
	}
	enc.putBit(128, true) // delta_present = true

	// Encode magnitude (4 bits, unsigned)
	magnitude := delta
	if magnitude < 0 {
		magnitude = -magnitude
	}
	if magnitude > 15 {
		magnitude = 15 // clamp to 4-bit range
	}
	enc.putLiteral(uint32(magnitude), 4)

	// Encode sign (1 bit): 0 = positive, 1 = negative
	enc.putBit(128, delta < 0)
}

// encodeFrameHeader boolean-encodes the VP8 key-frame header into the
// first partition. It encodes the minimum required fields so that a
// conformant VP8 decoder can parse the bitstream.
// Reference: RFC 6386, Section 9.2.
func encodeFrameHeader(enc *boolEncoder, width, height, qi int, deltas QuantDeltas, partCount PartitionCount, loopFilter loopFilterParams, numMBs int, mbs []macroblock) {
	// color_space (1 bit): 0 = BT.601
	enc.putBit(128, false)
	// clamping_type (1 bit): 0 = clamped
	enc.putBit(128, false)

	// segmentation_enabled (1 bit): 0 = disabled
	enc.putBit(128, false)

	// filter_type (1 bit): 0 = normal (simple filter disabled)
	enc.putBit(128, false)
	// loop_filter_level (6 bits)
	enc.putLiteral(uint32(loopFilter.level), 6)
	// sharpness_level (3 bits)
	enc.putLiteral(uint32(loopFilter.sharpness), 3)

	// mb_no_lf_delta (1 bit): 0
	enc.putBit(128, false)

	// nb_dct_partitions: log2(partition_count) (2 bits)
	enc.putLiteral(uint32(partCount), 2)

	// Quantizer indices (base + deltas).
	// y_ac_qi (7 bits)
	enc.putLiteral(uint32(qi), 7)
	// y1_dc_delta: present if non-zero
	encodeDelta(enc, deltas.Y1DC)
	// y2_dc_delta: present if non-zero
	encodeDelta(enc, deltas.Y2DC)
	// y2_ac_delta: present if non-zero
	encodeDelta(enc, deltas.Y2AC)
	// uv_dc_delta: present if non-zero
	encodeDelta(enc, deltas.UVDC)
	// uv_ac_delta: present if non-zero
	encodeDelta(enc, deltas.UVAC)

	// refresh_last_frame_buffer (1 bit): 0 (for keyframes, ignored by decoder)
	// Note: For keyframes, this bit is read but ignored (RFC 6386 §9.8)
	enc.putBit(128, false)

	// Token probability updates (RFC 6386 §13.4)
	// For simplicity, we signal no updates and use default probabilities.
	EncodeNoCoeffProbUpdates(enc)

	// mb_no_skip_coeff (1 bit): 1 → emit prob_skip_false
	enc.putBit(128, true)
	// prob_skip_false (8 bits): set high so most MBs are marked as skip
	enc.putLiteral(255, 8)

	// Macroblock-level data: encode prediction modes.
	for _, mb := range mbs {
		// coeff_skip (1 bit, prob = prob_skip_false = 255):
		// A value of 1 (true) means the macroblock has no non-zero DCT
		// coefficients and the residual partition is not read for this MB.
		enc.putBit(255, mb.skip)

		// Encode intra_mb_mode (y_mode) using the mode tree from RFC 6386 §11.2.
		// Tree structure:
		//   B_PRED vs {DC_PRED, V_PRED, H_PRED, TM_PRED}
		//   If not B_PRED: DC_PRED vs {V_PRED, H_PRED, TM_PRED}
		//   If not DC_PRED: V_PRED vs {H_PRED, TM_PRED}
		//   If not V_PRED: H_PRED vs TM_PRED
		encodeYMode(enc, mb.lumaMode, mb.bModes)

		// Encode uv_mode (chroma mode) using the chroma mode tree.
		encodeUVMode(enc, mb.chromaMode)
	}
}

// encodeYMode encodes the 16x16 luma prediction mode using the VP8 mode tree.
// When mode is B_PRED, it also encodes the 16 sub-block modes.
// Reference: RFC 6386 §11.2
// Decoder reads: readBit(145) -> if false: B_PRED mode (parse 16 sub-block modes)
//
//	-> if true: 16x16 mode, then parse which mode
func encodeYMode(enc *boolEncoder, mode intraMode, bModes [16]intraBMode) {
	// Key-frame y_mode probabilities (from decoder)
	const prob16x16 = 145 // true = 16x16 mode, false = B_PRED
	const probDCvsRest = 156
	const probDCvsV = 163
	const probHvsT = 128

	if mode == B_PRED {
		// B_PRED: readBit(145) returns false
		enc.putBit(prob16x16, false)
		// Encode 16 sub-block modes
		encodeBPredModes(enc, bModes)
		return
	}

	// 16x16 mode: readBit(145) returns true
	enc.putBit(prob16x16, true)

	// Decoder tree after 16x16 is confirmed:
	// !readBit(156) -> DC or V
	//   !readBit(163) -> DC
	//   readBit(163) -> V
	// readBit(156) -> H or TM
	//   !readBit(128) -> H
	//   readBit(128) -> TM

	if mode == DC_PRED || mode == V_PRED {
		enc.putBit(probDCvsRest, false) // DC or V branch
		if mode == DC_PRED {
			enc.putBit(probDCvsV, false) // DC
		} else {
			enc.putBit(probDCvsV, true) // V
		}
		return
	}

	// H or TM
	enc.putBit(probDCvsRest, true) // H or TM branch
	if mode == H_PRED {
		enc.putBit(probHvsT, false) // H
	} else {
		enc.putBit(probHvsT, true) // TM
	}
}

// kfBModeProb contains the key-frame sub-block mode probabilities.
// For each of the 10 possible left modes, there are 10 rows (for each above mode),
// and each row has 9 probability values for the binary tree.
// Reference: RFC 6386 §12.1, Table 13.
var kfBModeProb = [10][10][9]uint8{
	// left=B_DC_PRED
	{
		{231, 120, 48, 89, 115, 113, 120, 152, 112}, // above=B_DC_PRED
		{152, 179, 64, 126, 170, 118, 46, 70, 95},   // above=B_TM_PRED
		{175, 69, 143, 80, 85, 82, 72, 155, 103},    // above=B_VE_PRED
		{56, 58, 10, 171, 218, 189, 17, 13, 152},    // above=B_HE_PRED
		{144, 71, 10, 38, 171, 213, 144, 34, 26},    // above=B_LD_PRED
		{114, 26, 17, 163, 44, 195, 21, 10, 173},    // above=B_RD_PRED
		{121, 24, 80, 195, 26, 62, 44, 64, 85},      // above=B_VR_PRED
		{170, 46, 55, 19, 136, 160, 33, 206, 71},    // above=B_VL_PRED
		{63, 20, 8, 114, 114, 208, 12, 9, 226},      // above=B_HD_PRED
		{81, 40, 11, 96, 182, 84, 29, 16, 36},       // above=B_HU_PRED
	},
	// left=B_TM_PRED
	{
		{134, 183, 89, 137, 98, 101, 106, 165, 148},
		{72, 187, 100, 130, 157, 111, 32, 75, 80},
		{66, 102, 167, 99, 74, 62, 40, 234, 128},
		{41, 53, 9, 178, 241, 141, 26, 8, 107},
		{104, 79, 12, 27, 217, 255, 87, 17, 7},
		{74, 43, 26, 146, 73, 166, 49, 23, 157},
		{65, 38, 105, 160, 51, 52, 31, 115, 128},
		{87, 68, 71, 44, 114, 51, 15, 186, 23},
		{47, 41, 14, 110, 182, 183, 21, 17, 194},
		{66, 45, 25, 102, 197, 189, 23, 18, 22},
	},
	// left=B_VE_PRED
	{
		{88, 88, 147, 150, 42, 46, 45, 196, 205},
		{43, 97, 183, 117, 85, 38, 35, 179, 61},
		{39, 53, 200, 87, 26, 21, 43, 232, 171},
		{56, 34, 51, 104, 114, 102, 29, 93, 77},
		{107, 54, 32, 26, 51, 1, 81, 43, 31},
		{39, 28, 85, 171, 58, 165, 90, 98, 64},
		{34, 22, 116, 206, 23, 34, 43, 166, 73},
		{68, 25, 106, 22, 64, 171, 36, 225, 114},
		{34, 19, 21, 102, 132, 188, 16, 76, 124},
		{62, 18, 78, 95, 85, 57, 50, 48, 51},
	},
	// left=B_HE_PRED
	{
		{193, 101, 35, 159, 215, 111, 89, 46, 111},
		{60, 148, 31, 172, 219, 228, 21, 18, 111},
		{112, 113, 77, 85, 179, 255, 38, 120, 114},
		{40, 42, 1, 196, 245, 209, 10, 25, 109},
		{100, 80, 8, 43, 154, 1, 51, 26, 71},
		{88, 43, 29, 140, 166, 213, 37, 43, 154},
		{61, 63, 30, 155, 67, 45, 68, 1, 209},
		{142, 78, 78, 16, 255, 128, 34, 197, 171},
		{41, 40, 5, 102, 211, 183, 4, 1, 221},
		{51, 50, 17, 168, 209, 192, 23, 25, 82},
	},
	// left=B_LD_PRED
	{
		{138, 31, 36, 171, 27, 166, 38, 44, 229},
		{67, 87, 58, 169, 82, 115, 26, 59, 179},
		{63, 59, 90, 180, 59, 166, 93, 73, 154},
		{40, 40, 21, 116, 143, 209, 34, 39, 175},
		{57, 46, 22, 24, 128, 1, 54, 17, 37},
		{47, 15, 16, 183, 34, 223, 49, 45, 183},
		{46, 17, 33, 183, 6, 98, 15, 32, 183},
		{65, 32, 73, 115, 28, 128, 23, 128, 205},
		{40, 3, 9, 115, 51, 192, 18, 6, 223},
		{87, 37, 9, 115, 59, 77, 64, 21, 47},
	},
	// left=B_RD_PRED
	{
		{104, 55, 44, 218, 9, 54, 53, 130, 226},
		{64, 90, 70, 205, 40, 41, 23, 26, 57},
		{54, 57, 112, 184, 5, 41, 38, 166, 213},
		{30, 34, 26, 133, 152, 116, 10, 32, 134},
		{75, 32, 12, 51, 192, 255, 160, 43, 51},
		{39, 19, 53, 221, 26, 114, 32, 73, 255},
		{31, 9, 65, 234, 2, 15, 1, 118, 73},
		{88, 31, 35, 67, 102, 85, 55, 186, 85},
		{56, 21, 23, 111, 59, 205, 45, 37, 192},
		{55, 38, 70, 124, 73, 102, 1, 34, 98},
	},
	// left=B_VR_PRED
	{
		{125, 98, 42, 88, 104, 85, 117, 175, 82},
		{95, 84, 53, 89, 128, 100, 113, 101, 45},
		{75, 79, 123, 47, 51, 128, 81, 171, 1},
		{57, 17, 5, 71, 102, 57, 53, 41, 49},
		{115, 21, 2, 10, 102, 255, 166, 23, 6},
		{38, 33, 13, 121, 57, 73, 26, 1, 85},
		{41, 10, 67, 138, 77, 110, 90, 47, 114},
		{101, 29, 16, 10, 85, 128, 101, 196, 26},
		{57, 18, 10, 102, 102, 213, 34, 20, 43},
		{117, 20, 15, 36, 163, 128, 68, 1, 26},
	},
	// left=B_VL_PRED
	{
		{138, 31, 36, 171, 27, 166, 38, 44, 229},
		{67, 87, 58, 169, 82, 115, 26, 59, 179},
		{63, 59, 90, 180, 59, 166, 93, 73, 154},
		{40, 40, 21, 116, 143, 209, 34, 39, 175},
		{57, 46, 22, 24, 128, 1, 54, 17, 37},
		{47, 15, 16, 183, 34, 223, 49, 45, 183},
		{46, 17, 33, 183, 6, 98, 15, 32, 183},
		{65, 32, 73, 115, 28, 128, 23, 128, 205},
		{40, 3, 9, 115, 51, 192, 18, 6, 223},
		{87, 37, 9, 115, 59, 77, 64, 21, 47},
	},
	// left=B_HD_PRED
	{
		{101, 75, 35, 218, 9, 54, 53, 130, 226},
		{64, 90, 70, 205, 40, 41, 23, 26, 57},
		{54, 57, 112, 184, 5, 41, 38, 166, 213},
		{30, 34, 26, 133, 152, 116, 10, 32, 134},
		{75, 32, 12, 51, 192, 255, 160, 43, 51},
		{39, 19, 53, 221, 26, 114, 32, 73, 255},
		{31, 9, 65, 234, 2, 15, 1, 118, 73},
		{88, 31, 35, 67, 102, 85, 55, 186, 85},
		{56, 21, 23, 111, 59, 205, 45, 37, 192},
		{55, 38, 70, 124, 73, 102, 1, 34, 98},
	},
	// left=B_HU_PRED
	{
		{101, 75, 35, 218, 9, 54, 53, 130, 226},
		{64, 90, 70, 205, 40, 41, 23, 26, 57},
		{54, 57, 112, 184, 5, 41, 38, 166, 213},
		{30, 34, 26, 133, 152, 116, 10, 32, 134},
		{75, 32, 12, 51, 192, 255, 160, 43, 51},
		{39, 19, 53, 221, 26, 114, 32, 73, 255},
		{31, 9, 65, 234, 2, 15, 1, 118, 73},
		{88, 31, 35, 67, 102, 85, 55, 186, 85},
		{56, 21, 23, 111, 59, 205, 45, 37, 192},
		{55, 38, 70, 124, 73, 102, 1, 34, 98},
	},
}

// encodeBPredModes encodes the 16 sub-block modes for a B_PRED macroblock.
// Reference: RFC 6386 §12.1
func encodeBPredModes(enc *boolEncoder, bModes [16]intraBMode) {
	// The sub-block mode tree uses context from above and left modes.
	// For the first row/column, we assume B_DC_PRED as the context mode.
	const dcContext = B_DC_PRED

	for blockIdx := 0; blockIdx < 16; blockIdx++ {
		by := blockIdx / 4
		bx := blockIdx % 4

		// Get context modes (above and left sub-block modes)
		var aboveMode, leftMode intraBMode
		if by == 0 {
			aboveMode = dcContext // No above sub-block, use default
		} else {
			aboveMode = bModes[(by-1)*4+bx]
		}
		if bx == 0 {
			leftMode = dcContext // No left sub-block, use default
		} else {
			leftMode = bModes[by*4+(bx-1)]
		}

		// Encode this sub-block mode using the probability table
		encodeBMode(enc, bModes[blockIdx], aboveMode, leftMode)
	}
}

// encodeBMode encodes a single 4×4 sub-block mode using the context-dependent tree.
// Reference: RFC 6386 §12.1
//
// The B_PRED mode tree structure (as decoded by golang.org/x/image/vp8):
//
//	prob[0]: DC (false) vs rest (true)
//	prob[1]: TM (false) vs rest (true)
//	prob[2]: VE (false) vs rest (true)
//	prob[3]: {HE,RD,VR} (false) vs {LD,VL,HD,HU} (true)
//	  if false: prob[4]: HE (false) vs {RD,VR} (true)
//	             if true: prob[5]: RD (false) vs VR (true)
//	  if true: prob[6]: LD (false) vs {VL,HD,HU} (true)
//	           if true: prob[7]: VL (false) vs {HD,HU} (true)
//	                   if true: prob[8]: HD (false) vs HU (true)
func encodeBMode(enc *boolEncoder, mode, aboveMode, leftMode intraBMode) {
	// Get the probability row for this context
	probs := kfBModeProb[leftMode][aboveMode]

	// Navigate the binary tree to encode the mode
	switch mode {
	case B_DC_PRED:
		enc.putBit(probs[0], false) // DC
	case B_TM_PRED:
		enc.putBit(probs[0], true)  // not DC
		enc.putBit(probs[1], false) // TM
	case B_VE_PRED:
		enc.putBit(probs[0], true)  // not DC
		enc.putBit(probs[1], true)  // not TM
		enc.putBit(probs[2], false) // VE
	case B_HE_PRED:
		enc.putBit(probs[0], true)  // not DC
		enc.putBit(probs[1], true)  // not TM
		enc.putBit(probs[2], true)  // not VE
		enc.putBit(probs[3], false) // HE/RD/VR branch
		enc.putBit(probs[4], false) // HE
	case B_RD_PRED:
		enc.putBit(probs[0], true)  // not DC
		enc.putBit(probs[1], true)  // not TM
		enc.putBit(probs[2], true)  // not VE
		enc.putBit(probs[3], false) // HE/RD/VR branch
		enc.putBit(probs[4], true)  // not HE
		enc.putBit(probs[5], false) // RD
	case B_VR_PRED:
		enc.putBit(probs[0], true)  // not DC
		enc.putBit(probs[1], true)  // not TM
		enc.putBit(probs[2], true)  // not VE
		enc.putBit(probs[3], false) // HE/RD/VR branch
		enc.putBit(probs[4], true)  // not HE
		enc.putBit(probs[5], true)  // VR
	case B_LD_PRED:
		enc.putBit(probs[0], true)  // not DC
		enc.putBit(probs[1], true)  // not TM
		enc.putBit(probs[2], true)  // not VE
		enc.putBit(probs[3], true)  // LD/VL/HD/HU branch
		enc.putBit(probs[6], false) // LD
	case B_VL_PRED:
		enc.putBit(probs[0], true)  // not DC
		enc.putBit(probs[1], true)  // not TM
		enc.putBit(probs[2], true)  // not VE
		enc.putBit(probs[3], true)  // LD/VL/HD/HU branch
		enc.putBit(probs[6], true)  // not LD
		enc.putBit(probs[7], false) // VL
	case B_HD_PRED:
		enc.putBit(probs[0], true)  // not DC
		enc.putBit(probs[1], true)  // not TM
		enc.putBit(probs[2], true)  // not VE
		enc.putBit(probs[3], true)  // LD/VL/HD/HU branch
		enc.putBit(probs[6], true)  // not LD
		enc.putBit(probs[7], true)  // not VL
		enc.putBit(probs[8], false) // HD
	case B_HU_PRED:
		enc.putBit(probs[0], true) // not DC
		enc.putBit(probs[1], true) // not TM
		enc.putBit(probs[2], true) // not VE
		enc.putBit(probs[3], true) // LD/VL/HD/HU branch
		enc.putBit(probs[6], true) // not LD
		enc.putBit(probs[7], true) // not VL
		enc.putBit(probs[8], true) // HU
	}
}

// encodeUVMode encodes the 8x8 chroma prediction mode using the VP8 mode tree.
// Reference: RFC 6386 §11.2
func encodeUVMode(enc *boolEncoder, mode chromaMode) {
	// Key-frame uv_mode probabilities
	const probDCPred = 142
	const probVPred = 114
	const probHvsT = 183

	if mode == DC_PRED_CHROMA {
		enc.putBit(probDCPred, false) // DC_PRED
		return
	}
	enc.putBit(probDCPred, true) // Not DC_PRED

	if mode == V_PRED_CHROMA {
		enc.putBit(probVPred, false) // V_PRED
		return
	}
	enc.putBit(probVPred, true) // Not V_PRED

	if mode == H_PRED_CHROMA {
		enc.putBit(probHvsT, false) // H_PRED
	} else {
		enc.putBit(probHvsT, true) // TM_PRED
	}
}

// encodeChromaPlane encodes 4 chroma blocks (2x2) for a single plane (U or V).
// It returns the new left and up non-zero masks for the encoded plane.
// The shift parameter determines which bits of the combined UV mask to use
// (0 for U plane, 2 for V plane).
func encodeChromaPlane(te *TokenEncoder, coeffs [4][16]int16, leftMask, upMask uint8, shift int) (newLeft, newUp uint8) {
	var lnz, unz [2]uint8
	lnz[0] = (leftMask >> (shift + 0)) & 1
	lnz[1] = (leftMask >> (shift + 1)) & 1
	unz[0] = (upMask >> (shift + 0)) & 1
	unz[1] = (upMask >> (shift + 1)) & 1

	for blockY := 0; blockY < 2; blockY++ {
		nz := lnz[blockY]
		for blockX := 0; blockX < 2; blockX++ {
			blockIdx := blockY*2 + blockX
			ctx := int(nz + unz[blockX])
			if ctx > 2 {
				ctx = 2
			}
			hasNz := te.EncodeBlockWithContext(coeffs[blockIdx], PlaneUV, 0, ctx)
			nzVal := uint8(0)
			if hasNz {
				nzVal = 1
			}
			nz = nzVal
			unz[blockX] = nzVal
		}
		lnz[blockY] = nz
		newLeft |= nz << blockY
	}
	for i := 0; i < 2; i++ {
		newUp |= unz[i] << i
	}
	return newLeft, newUp
}

// BuildKeyFrame constructs a complete VP8 key-frame bitstream from a
// processed macroblock slice. It returns the raw VP8 frame bytes suitable
// for RTP packetisation.
//
// Parameters:
//   - width, height: frame dimensions (must be positive and even)
//   - qi: base quantizer index [0, 127]
//   - y1DCDelta, y2DCDelta, y2ACDelta, uvDCDelta, uvACDelta: per-plane quantizer deltas
//   - partCount: number of DCT partitions (OnePartition, TwoPartitions, etc.)
//   - loopFilter: loop filter parameters (level and sharpness)
//   - mbs: processed macroblocks
func BuildKeyFrame(width, height, qi, y1DCDelta, y2DCDelta, y2ACDelta, uvDCDelta, uvACDelta int, partCount PartitionCount, loopFilter loopFilterParams, mbs []macroblock) ([]byte, error) {
	if width <= 0 || height <= 0 || width%2 != 0 || height%2 != 0 {
		return nil, errInvalidDimensions
	}

	mbW := (width + 15) / 16
	mbH := (height + 15) / 16

	deltas := QuantDeltas{
		Y1DC: y1DCDelta,
		Y2DC: y2DCDelta,
		Y2AC: y2ACDelta,
		UVDC: uvDCDelta,
		UVAC: uvACDelta,
	}

	// Encode first partition (frame header + MB modes).
	partEnc := newBoolEncoder()
	encodeFrameHeader(partEnc, width, height, qi, deltas, partCount, loopFilter, len(mbs), mbs)
	firstPart := partEnc.flush()

	// Encode residual partitions
	coeffProbs := DefaultCoeffProbs
	var residualParts [][]byte

	if partCount == OnePartition {
		// Single partition: use simple token encoder
		residualEnc := newBoolEncoder()
		tokenEnc := NewTokenEncoder(residualEnc, &coeffProbs)
		encodeResidualPartition(tokenEnc, mbs, mbW)
		residualParts = [][]byte{residualEnc.flush()}
	} else {
		// Multiple partitions: distribute by macroblock row
		pw := NewPartitionWriter(partCount, &coeffProbs)
		encodeResidualMultiPartition(pw, mbs, mbW, mbH)
		residualParts = pw.Finalize()
	}

	firstPartSize := len(firstPart)

	// Frame tag (3 bytes, little-endian):
	//   bits [0]:     key_frame = 0 (0 = key)
	//   bits [3:1]:   version = 0
	//   bits [4]:     show_frame = 1
	//   bits [23:5]:  first_part_size
	tag := uint32(0)                  // key_frame = 0
	tag |= 0 << 1                     // version = 0
	tag |= 1 << 4                     // show_frame = 1
	tag |= uint32(firstPartSize) << 5 // first_part_size

	// Calculate total size
	partSizes := BuildPartitionSizes(residualParts)
	residualData := ConcatPartitions(residualParts)
	totalSize := 3 + 3 + 4 + firstPartSize + len(partSizes) + len(residualData)

	out := make([]byte, 0, totalSize)
	out = append(out, byte(tag), byte(tag>>8), byte(tag>>16))

	// Key-frame start code.
	out = append(out, vp8KeyFrameStartCode[:]...)

	// Width and height (each 16 bits, little-endian).
	var dim [4]byte
	binary.LittleEndian.PutUint16(dim[0:], uint16(width))
	binary.LittleEndian.PutUint16(dim[2:], uint16(height))
	out = append(out, dim[:]...)

	// First partition data.
	out = append(out, firstPart...)

	// Partition sizes (if multiple partitions)
	out = append(out, partSizes...)

	// Residual partition data.
	out = append(out, residualData...)

	return out, nil
}

// encodeResidualMultiPartition encodes DCT coefficients using multiple partitions.
// Macroblocks are distributed across partitions by row.
func encodeResidualMultiPartition(pw *PartitionWriter, mbs []macroblock, mbW, mbH int) {
	// Track non-zero status for context calculation
	leftNzY16 := uint8(0)
	upNzY16 := make([]uint8, mbW)
	leftNzMaskY := uint8(0)
	upNzMaskY := make([]uint8, mbW)
	leftNzMaskUV := uint8(0)
	upNzMaskUV := make([]uint8, mbW)

	for mbIdx, mb := range mbs {
		mbX := mbIdx % mbW
		mbY := mbIdx / mbW

		if mbX == 0 && mbIdx > 0 {
			leftNzY16 = 0
			leftNzMaskY = 0
			leftNzMaskUV = 0
		}

		// Get token encoder for this row's partition
		te := pw.GetTokenEncoder(mbY)

		if mb.skip {
			leftNzY16 = 0
			upNzY16[mbX] = 0
			leftNzMaskY = 0
			upNzMaskY[mbX] = 0
			leftNzMaskUV = 0
			upNzMaskUV[mbX] = 0
			continue
		}

		// Encode Y2 block for 16x16 modes
		if mb.lumaMode != B_PRED {
			y2Context := int(leftNzY16 + upNzY16[mbX])
			if y2Context > 2 {
				y2Context = 2
			}
			nz := te.EncodeBlockWithContext(mb.y2Coeffs, PlaneY2, 0, y2Context)
			nzVal := uint8(0)
			if nz {
				nzVal = 1
			}
			leftNzY16 = nzVal
			upNzY16[mbX] = nzVal
		} else {
			// B_PRED mode: no Y2 block, so reset Y2 context to 0
			leftNzY16 = 0
			upNzY16[mbX] = 0
		}

		// Encode Y blocks
		yPlane := PlaneY1WithY2
		firstYCoeff := 1
		if mb.lumaMode == B_PRED {
			yPlane = PlaneY1SansY2
			firstYCoeff = 0
		}

		var lnz, unz [4]uint8
		for i := 0; i < 4; i++ {
			lnz[i] = (leftNzMaskY >> i) & 1
			unz[i] = (upNzMaskY[mbX] >> i) & 1
		}

		var newLeftNzMaskY, newUpNzMaskY uint8
		for blockY := 0; blockY < 4; blockY++ {
			nz := lnz[blockY]
			for blockX := 0; blockX < 4; blockX++ {
				blockIdx := blockY*4 + blockX
				ctx := int(nz + unz[blockX])
				if ctx > 2 {
					ctx = 2
				}
				hasNz := te.EncodeBlockWithContext(mb.yCoeffs[blockIdx], yPlane, firstYCoeff, ctx)
				nzVal := uint8(0)
				if hasNz {
					nzVal = 1
				}
				nz = nzVal
				unz[blockX] = nzVal
			}
			lnz[blockY] = nz
			newLeftNzMaskY |= nz << blockY
		}
		for i := 0; i < 4; i++ {
			newUpNzMaskY |= unz[i] << i
		}
		leftNzMaskY = newLeftNzMaskY
		upNzMaskY[mbX] = newUpNzMaskY

		// Encode U and V chroma blocks using shared helper
		newLeftU, newUpU := encodeChromaPlane(te, mb.uCoeffs, leftNzMaskUV, upNzMaskUV[mbX], 0)
		newLeftV, newUpV := encodeChromaPlane(te, mb.vCoeffs, leftNzMaskUV, upNzMaskUV[mbX], 2)

		leftNzMaskUV = newLeftU | (newLeftV << 2)
		upNzMaskUV[mbX] = newUpU | (newUpV << 2)
	}
}

// encodeResidualPartition encodes DCT coefficients for all non-skip macroblocks.
// Reference: RFC 6386 §13
func encodeResidualPartition(te *TokenEncoder, mbs []macroblock, mbW int) {
	// Track non-zero status for context calculation
	// The decoder uses left + above neighbors to compute context (0, 1, or 2)

	// For Y2: track nzY16 per MB
	// nzY16[mbx] = 1 if MB at column mbx had non-zero Y2 coefficients
	leftNzY16 := uint8(0)
	upNzY16 := make([]uint8, mbW)

	// For Y1: track nzMask (4 bits for bottom row of 4x4 blocks)
	// leftNzMaskY = 4 bits for right column of left MB's Y blocks
	// upNzMaskY[mbx] = 4 bits for bottom row of above MB's Y blocks
	leftNzMaskY := uint8(0)
	upNzMaskY := make([]uint8, mbW)

	// For UV: similar tracking
	leftNzMaskUV := uint8(0)
	upNzMaskUV := make([]uint8, mbW)

	mbY := 0
	for mbIdx, mb := range mbs {
		mbX := mbIdx % mbW
		if mbX == 0 && mbIdx > 0 {
			mbY++
			leftNzY16 = 0
			leftNzMaskY = 0
			leftNzMaskUV = 0
		}

		if mb.skip {
			// Skip MB: update context tracking to 0 for this position
			leftNzY16 = 0
			upNzY16[mbX] = 0
			leftNzMaskY = 0
			upNzMaskY[mbX] = 0
			leftNzMaskUV = 0
			upNzMaskUV[mbX] = 0
			continue
		}

		// For 16x16 modes (not B_PRED), encode Y2 block first (DC-of-DCs)
		if mb.lumaMode != B_PRED {
			// Y2 context = left + above (clamped to 2)
			y2Context := int(leftNzY16 + upNzY16[mbX])
			if y2Context > 2 {
				y2Context = 2
			}
			nz := te.EncodeBlockWithContext(mb.y2Coeffs, PlaneY2, 0, y2Context)
			nzVal := uint8(0)
			if nz {
				nzVal = 1
			}
			leftNzY16 = nzVal
			upNzY16[mbX] = nzVal
		} else {
			// B_PRED mode: no Y2 block, so reset Y2 context to 0
			// This is critical for decoder/encoder context synchronization
			leftNzY16 = 0
			upNzY16[mbX] = 0
		}

		// Encode 16 Y blocks (4x4 each)
		// For 16x16 modes: plane 0 (PlaneY1WithY2), start at coefficient 1 (DC is in Y2)
		// For B_PRED: plane 3 (PlaneY1SansY2), start at coefficient 0
		yPlane := PlaneY1WithY2
		firstYCoeff := 1
		if mb.lumaMode == B_PRED {
			yPlane = PlaneY1SansY2
			firstYCoeff = 0
		}

		// Track Y block non-zero for context
		// lnz[0..3] = left column non-zero (from previous MB or 0)
		// unz[0..3] = above row non-zero (from above MB or 0)
		var lnz, unz [4]uint8
		for i := 0; i < 4; i++ {
			lnz[i] = (leftNzMaskY >> i) & 1
			unz[i] = (upNzMaskY[mbX] >> i) & 1
		}

		var newLeftNzMaskY, newUpNzMaskY uint8
		for blockY := 0; blockY < 4; blockY++ {
			nz := lnz[blockY]
			for blockX := 0; blockX < 4; blockX++ {
				blockIdx := blockY*4 + blockX
				ctx := int(nz + unz[blockX])
				if ctx > 2 {
					ctx = 2
				}
				hasNz := te.EncodeBlockWithContext(mb.yCoeffs[blockIdx], yPlane, firstYCoeff, ctx)
				nzVal := uint8(0)
				if hasNz {
					nzVal = 1
				}
				nz = nzVal
				unz[blockX] = nzVal
			}
			lnz[blockY] = nz
			// Record right column for next MB's left context
			newLeftNzMaskY |= nz << blockY
		}
		// Record bottom row for next row's above context
		for i := 0; i < 4; i++ {
			newUpNzMaskY |= unz[i] << i
		}
		leftNzMaskY = newLeftNzMaskY
		upNzMaskY[mbX] = newUpNzMaskY

		// Encode U and V chroma blocks using shared helper
		newLeftU, newUpU := encodeChromaPlane(te, mb.uCoeffs, leftNzMaskUV, upNzMaskUV[mbX], 0)
		newLeftV, newUpV := encodeChromaPlane(te, mb.vCoeffs, leftNzMaskUV, upNzMaskUV[mbX], 2)

		leftNzMaskUV = newLeftU | (newLeftV << 2)
		upNzMaskUV[mbX] = newUpU | (newUpV << 2)
	}
}
