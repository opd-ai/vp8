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

// ProbConfig holds coefficient probability configuration for adaptive encoding.
type ProbConfig struct {
	// CurrentProbs are the probabilities currently in use (may be updated).
	CurrentProbs *[4][8][3][11]uint8
	// NewProbs are the new probabilities to encode in the frame header.
	// If nil, no updates are encoded.
	NewProbs *[4][8][3][11]uint8
	// Histogram for recording token statistics (optional).
	Histogram *CoeffHistogram
}

// encodeCommonFrameHeader encodes the frame header fields shared between
// key frames and inter frames: segmentation, loop filter, partitions, and quantizers.
// Reference: RFC 6386 §9.2, §9.7
func encodeCommonFrameHeader(enc *boolEncoder, qi int, deltas QuantDeltas, partCount PartitionCount, loopFilter loopFilterParams) {
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
	// y1_dc_delta
	encodeDelta(enc, deltas.Y1DC)
	// y2_dc_delta
	encodeDelta(enc, deltas.Y2DC)
	// y2_ac_delta
	encodeDelta(enc, deltas.Y2AC)
	// uv_dc_delta
	encodeDelta(enc, deltas.UVDC)
	// uv_ac_delta
	encodeDelta(enc, deltas.UVAC)
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
	encodeFrameHeaderWithProbs(enc, width, height, qi, deltas, partCount, loopFilter, numMBs, mbs, nil)
}

// encodeFrameHeaderWithProbs encodes the key-frame header with optional probability updates.
func encodeFrameHeaderWithProbs(enc *boolEncoder, width, height, qi int, deltas QuantDeltas, partCount PartitionCount, loopFilter loopFilterParams, numMBs int, mbs []macroblock, probCfg *ProbConfig) {
	// color_space (1 bit): 0 = BT.601
	enc.putBit(128, false)
	// clamping_type (1 bit): 0 = clamped
	enc.putBit(128, false)

	// Encode common header fields (segmentation, loop filter, quantizers)
	encodeCommonFrameHeader(enc, qi, deltas, partCount, loopFilter)

	// refresh_last_frame_buffer (1 bit): 0 (for keyframes, ignored by decoder)
	// Note: For keyframes, this bit is read but ignored (RFC 6386 §9.8)
	enc.putBit(128, false)

	// Token probability updates (RFC 6386 §13.4)
	if probCfg != nil && probCfg.NewProbs != nil && probCfg.CurrentProbs != nil {
		EncodeCoeffProbUpdates(enc, probCfg.CurrentProbs, probCfg.NewProbs)
	} else {
		EncodeNoCoeffProbUpdates(enc)
	}

	// mb_no_skip_coeff (1 bit): 1 → emit prob_skip_false
	enc.putBit(128, true)
	// prob_skip_false (8 bits): set high so most MBs are marked as skip
	enc.putLiteral(255, 8)

	// Calculate MB grid dimensions
	mbW := (width + 15) / 16

	// Track B_PRED sub-block modes for context across macroblock boundaries.
	// For each column, store the bottom row (4 modes) of the above MB.
	// For the left context, store the right column (4 modes) of the left MB.
	aboveBModes := make([][4]intraBMode, mbW) // aboveBModes[mbX] = bottom row of MB above
	var leftBModes [4]intraBMode              // right column of MB to the left

	// Macroblock-level data: encode prediction modes.
	for mbIdx, mb := range mbs {
		mbX := mbIdx % mbW

		// Reset left context at the start of each row
		if mbX == 0 {
			leftBModes = [4]intraBMode{B_DC_PRED, B_DC_PRED, B_DC_PRED, B_DC_PRED}
		}

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
		encodeYModeWithContext(enc, mb.lumaMode, mb.bModes, aboveBModes[mbX], leftBModes)

		// Update context for next MB
		if mb.lumaMode == B_PRED {
			// Save bottom row for above context of the MB below
			for i := 0; i < 4; i++ {
				aboveBModes[mbX][i] = mb.bModes[12+i] // blocks 12, 13, 14, 15
			}
			// Save right column for left context of the next MB
			for i := 0; i < 4; i++ {
				leftBModes[i] = mb.bModes[i*4+3] // blocks 3, 7, 11, 15
			}
		} else {
			// For 16x16 modes, decoder uses B_DC_PRED as context
			aboveBModes[mbX] = [4]intraBMode{B_DC_PRED, B_DC_PRED, B_DC_PRED, B_DC_PRED}
			leftBModes = [4]intraBMode{B_DC_PRED, B_DC_PRED, B_DC_PRED, B_DC_PRED}
		}

		// Encode uv_mode (chroma mode) using the chroma mode tree.
		encodeUVMode(enc, mb.chromaMode)
	}
}

// encodeYModeWithContext encodes the 16x16 luma prediction mode using the VP8 mode tree,
// with proper B_PRED sub-block context from neighboring macroblocks.
// Reference: RFC 6386 §11.2, §12.1
func encodeYModeWithContext(enc *boolEncoder, mode intraMode, bModes [16]intraBMode, aboveModes, leftModes [4]intraBMode) {
	// Key-frame y_mode probabilities (from decoder)
	const prob16x16 = 145 // true = 16x16 mode, false = B_PRED
	const probDCvsRest = 156
	const probDCvsV = 163
	const probHvsT = 128

	if mode == B_PRED {
		// B_PRED: readBit(145) returns false
		enc.putBit(prob16x16, false)
		// Encode 16 sub-block modes with proper context
		encodeBPredModesWithContext(enc, bModes, aboveModes, leftModes)
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
// First index is above mode, second index is left mode, matching the decoder's
// predProb[above][left] indexing (golang.org/x/image/vp8/pred.go).
// Each entry has 9 probability values for the binary tree.
// Reference: RFC 6386 §12.1, Table 13.
// kfBModeProb contains probabilities for decoding 4x4 sub-block modes.
// The table is indexed as kfBModeProb[aboveMode][leftMode][probIndex].
// This table matches the golang.org/x/image/vp8 decoder's predProb table.
// Mode order: DC(0), TM(1), VE(2), HE(3), RD(4), VR(5), LD(6), VL(7), HD(8), HU(9)
// Reference: RFC 6386 §11.5
var kfBModeProb = [10][10][9]uint8{
	// above=B_DC_PRED (0)
	{
		{231, 120, 48, 89, 115, 113, 120, 152, 112}, // left=B_DC_PRED
		{152, 179, 64, 126, 170, 118, 46, 70, 95},   // left=B_TM_PRED
		{175, 69, 143, 80, 85, 82, 72, 155, 103},    // left=B_VE_PRED
		{56, 58, 10, 171, 218, 189, 17, 13, 152},    // left=B_HE_PRED
		{114, 26, 17, 163, 44, 195, 21, 10, 173},    // left=B_RD_PRED
		{121, 24, 80, 195, 26, 62, 44, 64, 85},      // left=B_VR_PRED
		{144, 71, 10, 38, 171, 213, 144, 34, 26},    // left=B_LD_PRED
		{170, 46, 55, 19, 136, 160, 33, 206, 71},    // left=B_VL_PRED
		{63, 20, 8, 114, 114, 208, 12, 9, 226},      // left=B_HD_PRED
		{81, 40, 11, 96, 182, 84, 29, 16, 36},       // left=B_HU_PRED
	},
	// above=B_TM_PRED (1)
	{
		{134, 183, 89, 137, 98, 101, 106, 165, 148},
		{72, 187, 100, 130, 157, 111, 32, 75, 80},
		{66, 102, 167, 99, 74, 62, 40, 234, 128},
		{41, 53, 9, 178, 241, 141, 26, 8, 107},
		{74, 43, 26, 146, 73, 166, 49, 23, 157},
		{65, 38, 105, 160, 51, 52, 31, 115, 128},
		{104, 79, 12, 27, 217, 255, 87, 17, 7},
		{87, 68, 71, 44, 114, 51, 15, 186, 23},
		{47, 41, 14, 110, 182, 183, 21, 17, 194},
		{66, 45, 25, 102, 197, 189, 23, 18, 22},
	},
	// above=B_VE_PRED (2)
	{
		{88, 88, 147, 150, 42, 46, 45, 196, 205},
		{43, 97, 183, 117, 85, 38, 35, 179, 61},
		{39, 53, 200, 87, 26, 21, 43, 232, 171},
		{56, 34, 51, 104, 114, 102, 29, 93, 77},
		{39, 28, 85, 171, 58, 165, 90, 98, 64},
		{34, 22, 116, 206, 23, 34, 43, 166, 73},
		{107, 54, 32, 26, 51, 1, 81, 43, 31},
		{68, 25, 106, 22, 64, 171, 36, 225, 114},
		{34, 19, 21, 102, 132, 188, 16, 76, 124},
		{62, 18, 78, 95, 85, 57, 50, 48, 51},
	},
	// above=B_HE_PRED (3)
	{
		{193, 101, 35, 159, 215, 111, 89, 46, 111},
		{60, 148, 31, 172, 219, 228, 21, 18, 111},
		{112, 113, 77, 85, 179, 255, 38, 120, 114},
		{40, 42, 1, 196, 245, 209, 10, 25, 109},
		{88, 43, 29, 140, 166, 213, 37, 43, 154},
		{61, 63, 30, 155, 67, 45, 68, 1, 209},
		{100, 80, 8, 43, 154, 1, 51, 26, 71},
		{142, 78, 78, 16, 255, 128, 34, 197, 171},
		{41, 40, 5, 102, 211, 183, 4, 1, 221},
		{51, 50, 17, 168, 209, 192, 23, 25, 82},
	},
	// above=B_RD_PRED (4)
	{
		{138, 31, 36, 171, 27, 166, 38, 44, 229},
		{67, 87, 58, 169, 82, 115, 26, 59, 179},
		{63, 59, 90, 180, 59, 166, 93, 73, 154},
		{40, 40, 21, 116, 143, 209, 34, 39, 175},
		{47, 15, 16, 183, 34, 223, 49, 45, 183},
		{46, 17, 33, 183, 6, 98, 15, 32, 183},
		{57, 46, 22, 24, 128, 1, 54, 17, 37},
		{65, 32, 73, 115, 28, 128, 23, 128, 205},
		{40, 3, 9, 115, 51, 192, 18, 6, 223},
		{87, 37, 9, 115, 59, 77, 64, 21, 47},
	},
	// above=B_VR_PRED (5)
	{
		{104, 55, 44, 218, 9, 54, 53, 130, 226},
		{64, 90, 70, 205, 40, 41, 23, 26, 57},
		{54, 57, 112, 184, 5, 41, 38, 166, 213},
		{30, 34, 26, 133, 152, 116, 10, 32, 134},
		{39, 19, 53, 221, 26, 114, 32, 73, 255},
		{31, 9, 65, 234, 2, 15, 1, 118, 73},
		{75, 32, 12, 51, 192, 255, 160, 43, 51},
		{88, 31, 35, 67, 102, 85, 55, 186, 85},
		{56, 21, 23, 111, 59, 205, 45, 37, 192},
		{55, 38, 70, 124, 73, 102, 1, 34, 98},
	},
	// above=B_LD_PRED (6)
	{
		{125, 98, 42, 88, 104, 85, 117, 175, 82},
		{95, 84, 53, 89, 128, 100, 113, 101, 45},
		{75, 79, 123, 47, 51, 128, 81, 171, 1},
		{57, 17, 5, 71, 102, 57, 53, 41, 49},
		{38, 33, 13, 121, 57, 73, 26, 1, 85},
		{41, 10, 67, 138, 77, 110, 90, 47, 114},
		{115, 21, 2, 10, 102, 255, 166, 23, 6},
		{101, 29, 16, 10, 85, 128, 101, 196, 26},
		{57, 18, 10, 102, 102, 213, 34, 20, 43},
		{117, 20, 15, 36, 163, 128, 68, 1, 26},
	},
	// above=B_VL_PRED (7) - copied from golang.org/x/image/vp8 predProb[7]
	{
		{102, 61, 71, 37, 34, 53, 31, 243, 192},  // left=DC
		{69, 60, 71, 38, 73, 119, 28, 222, 37},   // left=TM
		{68, 45, 128, 34, 1, 47, 11, 245, 171},   // left=VE
		{62, 17, 19, 70, 146, 85, 55, 62, 70},    // left=HE
		{37, 43, 37, 154, 100, 163, 85, 160, 1},  // left=RD (was at [7][6])
		{63, 9, 92, 136, 28, 64, 32, 201, 85},    // left=VR (was at [7][4])
		{75, 15, 9, 9, 64, 255, 184, 119, 16},    // left=LD (was at [7][5])
		{86, 6, 28, 5, 64, 255, 25, 248, 1},      // left=VL
		{56, 8, 17, 132, 137, 255, 55, 116, 128}, // left=HD
		{58, 15, 20, 82, 135, 57, 26, 121, 40},   // left=HU
	},
	// above=B_HD_PRED (8) - copied from golang.org/x/image/vp8 predProb[8]
	{
		{164, 50, 31, 137, 154, 133, 25, 35, 218}, // left=DC
		{51, 103, 44, 131, 131, 123, 31, 6, 158},  // left=TM
		{86, 40, 64, 135, 148, 224, 45, 183, 128}, // left=VE
		{22, 26, 17, 131, 240, 154, 14, 1, 209},   // left=HE
		{45, 16, 21, 91, 64, 222, 7, 1, 197},      // left=RD (was at [8][6])
		{56, 21, 39, 155, 60, 138, 23, 102, 213},  // left=VR (was at [8][4])
		{83, 12, 13, 54, 192, 255, 68, 47, 28},    // left=LD (was at [8][5])
		{85, 26, 85, 85, 128, 128, 32, 146, 171},  // left=VL
		{18, 11, 7, 63, 144, 171, 4, 4, 246},      // left=HD
		{35, 27, 10, 146, 174, 171, 12, 26, 128},  // left=HU
	},
	// above=B_HU_PRED (9) - copied from golang.org/x/image/vp8 predProb[9]
	{
		{190, 80, 35, 99, 180, 80, 126, 54, 45},     // left=DC
		{85, 126, 47, 87, 176, 51, 41, 20, 32},      // left=TM
		{101, 75, 128, 139, 118, 146, 116, 128, 85}, // left=VE
		{56, 41, 15, 176, 236, 85, 37, 9, 62},       // left=HE
		{71, 30, 17, 119, 118, 255, 17, 18, 138},    // left=RD (was at [9][6])
		{101, 38, 60, 138, 55, 70, 43, 26, 142},     // left=VR (was at [9][4])
		{146, 36, 19, 30, 171, 255, 97, 27, 20},     // left=LD (was at [9][5])
		{138, 45, 61, 62, 219, 1, 81, 188, 64},      // left=VL
		{32, 41, 20, 117, 151, 142, 20, 21, 163},    // left=HD
		{112, 19, 12, 61, 195, 128, 48, 4, 24},      // left=HU
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

// encodeBPredModesWithContext encodes the 16 sub-block modes for a B_PRED macroblock,
// using proper context from neighboring macroblocks.
// aboveModes: bottom row of sub-block modes from the macroblock above (4 modes)
// leftModes: right column of sub-block modes from the macroblock to the left (4 modes)
// Reference: RFC 6386 §12.1
func encodeBPredModesWithContext(enc *boolEncoder, bModes [16]intraBMode, aboveModes, leftModes [4]intraBMode) {
	for blockIdx := 0; blockIdx < 16; blockIdx++ {
		by := blockIdx / 4
		bx := blockIdx % 4

		// Get context modes (above and left sub-block modes)
		var aboveMode, leftMode intraBMode

		if by == 0 {
			// First row of sub-blocks: use context from MB above
			aboveMode = aboveModes[bx]
		} else {
			// Other rows: use context from sub-block above within this MB
			aboveMode = bModes[(by-1)*4+bx]
		}

		if bx == 0 {
			// First column of sub-blocks: use context from MB to the left
			leftMode = leftModes[by]
		} else {
			// Other columns: use context from sub-block to the left within this MB
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
	// Get the probability row for this context.
	// The decoder (golang.org/x/image/vp8) indexes as predProb[above][left],
	// so we must use the same ordering: kfBModeProb[aboveMode][leftMode].
	probs := kfBModeProb[aboveMode][leftMode]

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

// encodeLumaBlocks encodes 16 luma blocks (4x4 grid) for a macroblock.
// It returns the new left and up non-zero masks for context tracking.
// Parameters:
//   - te: token encoder
//   - yCoeffs: 16 coefficient blocks in raster order
//   - yPlane: either PlaneY1WithY2 (for 16x16 modes) or PlaneY1SansY2 (for B_PRED)
//   - firstYCoeff: 1 for 16x16 modes (DC in Y2), 0 for B_PRED
//   - leftNzMaskY: 4-bit mask of non-zero status from right column of left MB
//   - upNzMaskY: 4-bit mask of non-zero status from bottom row of above MB
func encodeLumaBlocks(te *TokenEncoder, yCoeffs [16][16]int16, yPlane, firstYCoeff int,
	leftNzMaskY, upNzMaskY uint8,
) (newLeftNzMaskY, newUpNzMaskY uint8) {
	var lnz, unz [4]uint8
	for i := 0; i < 4; i++ {
		lnz[i] = (leftNzMaskY >> i) & 1
		unz[i] = (upNzMaskY >> i) & 1
	}

	for blockY := 0; blockY < 4; blockY++ {
		nz := lnz[blockY]
		for blockX := 0; blockX < 4; blockX++ {
			blockIdx := blockY*4 + blockX
			ctx := int(nz + unz[blockX])
			if ctx > 2 {
				ctx = 2
			}
			hasNz := te.EncodeBlockWithContext(yCoeffs[blockIdx], yPlane, firstYCoeff, ctx)
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
	return newLeftNzMaskY, newUpNzMaskY
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
	return buildKeyFrameWithProbs(width, height, qi, y1DCDelta, y2DCDelta, y2ACDelta, uvDCDelta, uvACDelta, partCount, loopFilter, mbs, nil)
}

// buildKeyFrameWithProbs constructs a key frame with optional probability configuration.
func buildKeyFrameWithProbs(width, height, qi, y1DCDelta, y2DCDelta, y2ACDelta, uvDCDelta, uvACDelta int, partCount PartitionCount, loopFilter loopFilterParams, mbs []macroblock, probCfg *ProbConfig) ([]byte, error) {
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
	encodeFrameHeaderWithProbs(partEnc, width, height, qi, deltas, partCount, loopFilter, len(mbs), mbs, probCfg)
	firstPart := partEnc.flush()

	// Encode residual partitions using shared helper
	residualParts := encodeResidualPartitionsWithProbs(partCount, mbs, mbW, mbH, probCfg)

	// Build key frame: frame_type=0 with start code and dimensions
	return assembleKeyFrameBitstream(firstPart, residualParts, width, height)
}

// encodeResidualPartitions encodes DCT residual coefficients into partitions.
// Used by both key frame and inter frame encoders.
func encodeResidualPartitions(partCount PartitionCount, mbs []macroblock, mbW, mbH int) [][]byte {
	return encodeResidualPartitionsWithProbs(partCount, mbs, mbW, mbH, nil)
}

// encodeResidualPartitionsWithProbs encodes residuals with optional probability configuration.
func encodeResidualPartitionsWithProbs(partCount PartitionCount, mbs []macroblock, mbW, mbH int, probCfg *ProbConfig) [][]byte {
	var coeffProbs *[4][8][3][11]uint8
	if probCfg != nil && probCfg.CurrentProbs != nil {
		coeffProbs = probCfg.CurrentProbs
	} else {
		defaultProbs := DefaultCoeffProbs
		coeffProbs = &defaultProbs
	}

	if partCount == OnePartition {
		residualEnc := newBoolEncoder()
		tokenEnc := NewTokenEncoder(residualEnc, coeffProbs)
		if probCfg != nil && probCfg.Histogram != nil {
			tokenEnc.SetHistogram(probCfg.Histogram)
		}
		encodeResidualPartition(tokenEnc, mbs, mbW)
		return [][]byte{residualEnc.flush()}
	}

	pw := NewPartitionWriterWithHistogram(partCount, coeffProbs, probCfg)
	encodeResidualMultiPartition(pw, mbs, mbW, mbH)
	return pw.Finalize()
}

// maxFirstPartSize is the maximum size of the first partition in bytes.
// The first partition size is stored in 19 bits of the frame tag.
const maxFirstPartSize = (1 << 19) - 1 // 524,287 bytes

// errFirstPartitionTooLarge is returned when the first partition exceeds the 19-bit limit.
var errFirstPartitionTooLarge = fmt.Errorf("first partition size exceeds 19-bit limit (%d bytes)", maxFirstPartSize)

// buildFrameTag constructs the 3-byte VP8 frame tag.
// frameType: 0=key frame, 1=inter frame
// Returns an error if firstPartSize exceeds the 19-bit limit (524,287 bytes).
func buildFrameTag(frameType, firstPartSize int) ([3]byte, error) {
	if firstPartSize > maxFirstPartSize {
		return [3]byte{}, errFirstPartitionTooLarge
	}
	tag := uint32(frameType)
	tag |= 0 << 1                     // version = 0
	tag |= 1 << 4                     // show_frame = 1
	tag |= uint32(firstPartSize) << 5 // first_part_size
	return [3]byte{byte(tag), byte(tag >> 8), byte(tag >> 16)}, nil
}

// assembleKeyFrameBitstream builds the complete key frame bitstream.
func assembleKeyFrameBitstream(firstPart []byte, residualParts [][]byte, width, height int) ([]byte, error) {
	firstPartSize := len(firstPart)
	tag, err := buildFrameTag(0, firstPartSize)
	if err != nil {
		return nil, err
	}

	partSizes := BuildPartitionSizes(residualParts)
	residualData := ConcatPartitions(residualParts)
	totalSize := 3 + 3 + 4 + firstPartSize + len(partSizes) + len(residualData)

	out := make([]byte, 0, totalSize)
	out = append(out, tag[:]...)
	out = append(out, vp8KeyFrameStartCode[:]...)

	var dim [4]byte
	binary.LittleEndian.PutUint16(dim[0:], uint16(width))
	binary.LittleEndian.PutUint16(dim[2:], uint16(height))
	out = append(out, dim[:]...)

	out = append(out, firstPart...)
	out = append(out, partSizes...)
	out = append(out, residualData...)

	return out, nil
}

// assembleInterFrameBitstream builds the complete inter frame bitstream.
func assembleInterFrameBitstream(firstPart []byte, residualParts [][]byte) ([]byte, error) {
	firstPartSize := len(firstPart)
	tag, err := buildFrameTag(1, firstPartSize)
	if err != nil {
		return nil, err
	}

	partSizes := BuildPartitionSizes(residualParts)
	residualData := ConcatPartitions(residualParts)
	totalSize := 3 + firstPartSize + len(partSizes) + len(residualData)

	out := make([]byte, 0, totalSize)
	out = append(out, tag[:]...)
	out = append(out, firstPart...)
	out = append(out, partSizes...)
	out = append(out, residualData...)

	return out, nil
}

// tokenEncoderProvider abstracts how a TokenEncoder is obtained for each row.
// This allows sharing encoding logic between single and multi-partition modes.
type tokenEncoderProvider interface {
	GetTokenEncoder(mbY int) *TokenEncoder
}

// singlePartitionProvider wraps a single TokenEncoder for single-partition mode.
type singlePartitionProvider struct {
	te *TokenEncoder
}

func (p *singlePartitionProvider) GetTokenEncoder(mbY int) *TokenEncoder { return p.te }

// encodeResidualMultiPartition encodes DCT coefficients using multiple partitions.
// Macroblocks are distributed across partitions by row.
func encodeResidualMultiPartition(pw *PartitionWriter, mbs []macroblock, mbW, mbH int) {
	encodeResidualWithProvider(pw, mbs, mbW)
}

// encodeResidualPartition encodes DCT coefficients for all non-skip macroblocks.
// Reference: RFC 6386 §13
func encodeResidualPartition(te *TokenEncoder, mbs []macroblock, mbW int) {
	encodeResidualWithProvider(&singlePartitionProvider{te: te}, mbs, mbW)
}

// encodeResidualWithProvider encodes DCT coefficients using the provided encoder source.
// Shared implementation for both single and multi-partition modes.
func encodeResidualWithProvider(provider tokenEncoderProvider, mbs []macroblock, mbW int) {
	ctx := newResidualContext(mbW)
	for mbIdx, mb := range mbs {
		ctx.encodeMacroblock(provider, mb, mbIdx, mbW)
	}
}

// residualContext tracks non-zero status for context calculation during residual encoding.
type residualContext struct {
	leftNzY16    uint8
	upNzY16      []uint8
	leftNzMaskY  uint8
	upNzMaskY    []uint8
	leftNzMaskUV uint8
	upNzMaskUV   []uint8
}

// newResidualContext creates a new residual encoding context.
func newResidualContext(mbW int) *residualContext {
	return &residualContext{
		upNzY16:    make([]uint8, mbW),
		upNzMaskY:  make([]uint8, mbW),
		upNzMaskUV: make([]uint8, mbW),
	}
}

// encodeMacroblock encodes a single macroblock's residual data.
func (ctx *residualContext) encodeMacroblock(provider tokenEncoderProvider, mb macroblock, mbIdx, mbW int) {
	mbX := mbIdx % mbW
	mbY := mbIdx / mbW

	if mbX == 0 && mbIdx > 0 {
		ctx.resetLeftContext()
	}

	te := provider.GetTokenEncoder(mbY)

	if mb.skip {
		ctx.clearContext(mbX)
		return
	}

	ctx.encodeY2Block(te, &mb, mbX)
	ctx.encodeLumaAndChroma(te, &mb, mbX)
}

// resetLeftContext resets the left context at the start of each row.
func (ctx *residualContext) resetLeftContext() {
	ctx.leftNzY16 = 0
	ctx.leftNzMaskY = 0
	ctx.leftNzMaskUV = 0
}

// clearContext clears all context for a skipped macroblock.
func (ctx *residualContext) clearContext(mbX int) {
	ctx.leftNzY16 = 0
	ctx.upNzY16[mbX] = 0
	ctx.leftNzMaskY = 0
	ctx.upNzMaskY[mbX] = 0
	ctx.leftNzMaskUV = 0
	ctx.upNzMaskUV[mbX] = 0
}

// encodeY2Block encodes the Y2 block for 16x16 modes.
func (ctx *residualContext) encodeY2Block(te *TokenEncoder, mb *macroblock, mbX int) {
	if mb.lumaMode != B_PRED {
		y2Context := minInt(int(ctx.leftNzY16+ctx.upNzY16[mbX]), 2)
		nz := te.EncodeBlockWithContext(mb.y2Coeffs, PlaneY2, 0, y2Context)
		nzVal := boolToUint8(nz)
		ctx.leftNzY16 = nzVal
		ctx.upNzY16[mbX] = nzVal
	} else {
		ctx.leftNzY16 = 0
		ctx.upNzY16[mbX] = 0
	}
}

// encodeLumaAndChroma encodes luma and chroma blocks.
func (ctx *residualContext) encodeLumaAndChroma(te *TokenEncoder, mb *macroblock, mbX int) {
	yPlane, firstYCoeff := PlaneY1WithY2, 1
	if mb.lumaMode == B_PRED {
		yPlane, firstYCoeff = PlaneY1SansY2, 0
	}

	ctx.leftNzMaskY, ctx.upNzMaskY[mbX] = encodeLumaBlocks(te, mb.yCoeffs, yPlane, firstYCoeff, ctx.leftNzMaskY, ctx.upNzMaskY[mbX])

	newLeftU, newUpU := encodeChromaPlane(te, mb.uCoeffs, ctx.leftNzMaskUV, ctx.upNzMaskUV[mbX], 0)
	newLeftV, newUpV := encodeChromaPlane(te, mb.vCoeffs, ctx.leftNzMaskUV, ctx.upNzMaskUV[mbX], 2)

	ctx.leftNzMaskUV = newLeftU | (newLeftV << 2)
	ctx.upNzMaskUV[mbX] = newUpU | (newUpV << 2)
}

// boolToUint8 converts a bool to uint8 (1 if true, 0 if false).
func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// minInt returns the smaller of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
