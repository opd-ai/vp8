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

// encodeFrameHeader boolean-encodes the VP8 key-frame header into the
// first partition. It encodes the minimum required fields so that a
// conformant VP8 decoder can parse the bitstream.
// Reference: RFC 6386, Section 9.2.
func encodeFrameHeader(enc *boolEncoder, width, height, qi, numMBs int, mbs []macroblock) {
	// color_space (1 bit): 0 = BT.601
	enc.putBit(128, false)
	// clamping_type (1 bit): 0 = clamped
	enc.putBit(128, false)

	// segmentation_enabled (1 bit): 0 = disabled
	enc.putBit(128, false)

	// filter_type (1 bit): 0 = normal (simple filter disabled)
	enc.putBit(128, false)
	// loop_filter_level (6 bits)
	enc.putLiteral(0, 6)
	// sharpness_level (3 bits)
	enc.putLiteral(0, 3)

	// mb_no_lf_delta (1 bit): 0
	enc.putBit(128, false)

	// nb_dct_partitions: 0 → 1 partition (2 bits)
	enc.putLiteral(0, 2)

	// Quantizer indices (base + deltas).
	// y_ac_qi (7 bits)
	enc.putLiteral(uint32(qi), 7)
	// y_dc_delta_present (1 bit): 0
	enc.putBit(128, false)
	// y2_dc_delta_present (1 bit): 0
	enc.putBit(128, false)
	// y2_ac_delta_present (1 bit): 0
	enc.putBit(128, false)
	// uv_dc_delta_present (1 bit): 0
	enc.putBit(128, false)
	// uv_ac_delta_present (1 bit): 0
	enc.putBit(128, false)

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
		encodeYMode(enc, mb.lumaMode)

		// Encode uv_mode (chroma mode) using the chroma mode tree.
		encodeUVMode(enc, mb.chromaMode)
	}
}

// encodeYMode encodes the 16x16 luma prediction mode using the VP8 mode tree.
// Reference: RFC 6386 §11.2
// Decoder reads: readBit(145) -> if false: B_PRED mode (parse 16 sub-block modes)
//
//	-> if true: 16x16 mode, then parse which mode
func encodeYMode(enc *boolEncoder, mode intraMode) {
	// Key-frame y_mode probabilities (from decoder)
	const prob16x16 = 145 // true = 16x16 mode, false = B_PRED
	const probDCvsRest = 156
	const probDCvsV = 163
	const probHvsT = 128

	if mode == B_PRED {
		// B_PRED: readBit(145) returns false
		enc.putBit(prob16x16, false)
		// For B_PRED, encoder would need to write 16 sub-block modes
		// (not implemented - would need separate function)
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

// BuildKeyFrame constructs a complete VP8 key-frame bitstream from a
// processed macroblock slice. It returns the raw VP8 frame bytes suitable
// for RTP packetisation.
func BuildKeyFrame(width, height, qi int, mbs []macroblock) ([]byte, error) {
	if width <= 0 || height <= 0 || width%2 != 0 || height%2 != 0 {
		return nil, errInvalidDimensions
	}

	// Encode first partition (frame header + MB modes).
	partEnc := newBoolEncoder()
	encodeFrameHeader(partEnc, width, height, qi, len(mbs), mbs)
	firstPart := partEnc.flush()

	// Second partition (residual data): encode DCT coefficients for non-skip MBs.
	residualEnc := newBoolEncoder()
	coeffProbs := DefaultCoeffProbs
	tokenEnc := NewTokenEncoder(residualEnc, &coeffProbs)
	encodeResidualPartition(tokenEnc, mbs)
	secondPart := residualEnc.flush()

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

	out := make([]byte, 0, 3+3+4+firstPartSize+len(secondPart))
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
	// Second (residual) partition data.
	out = append(out, secondPart...)

	return out, nil
}

// encodeResidualPartition encodes DCT coefficients for all non-skip macroblocks.
// Reference: RFC 6386 §13
func encodeResidualPartition(te *TokenEncoder, mbs []macroblock) {
	for _, mb := range mbs {
		if mb.skip {
			continue
		}

		// For 16x16 modes (not B_PRED), encode Y2 block first (DC-of-DCs)
		if mb.lumaMode != B_PRED {
			// Y2 block: DC values of all 16 Y blocks, transformed with WHT
			// Uses plane 1 (PlaneY2)
			te.EncodeBlock(mb.y2Coeffs, PlaneY2, 0)
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
		for i := 0; i < 16; i++ {
			te.EncodeBlock(mb.yCoeffs[i], yPlane, firstYCoeff)
		}

		// Encode 4 U blocks (plane 2 = PlaneUV)
		for i := 0; i < 4; i++ {
			te.EncodeBlock(mb.uCoeffs[i], PlaneUV, 0)
		}

		// Encode 4 V blocks (plane 2 = PlaneUV)
		for i := 0; i < 4; i++ {
			te.EncodeBlock(mb.vCoeffs[i], PlaneUV, 0)
		}
	}
}
