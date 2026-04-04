package vp8

// This file implements VP8 inter-frame (P-frame) bitstream encoding.
// It handles:
//   - Inter-frame header encoding (different from key-frame header)
//   - Motion vector encoding
//   - Inter prediction mode encoding
//   - Frame assembly for inter frames
//
// Reference: RFC 6386 §9.2 (inter frame header), §16 (inter prediction),
//            §17 (motion vector encoding)

// interMVProbs contains the VP8 default motion vector component probabilities.
// These are used to encode/decode motion vector components.
// Reference: RFC 6386 §17.2, Table 5
var interMVProbs = [2][19]uint8{
	// Horizontal (x) component probabilities
	{
		162,               // is_short
		128,               // sign
		225,               // short tree bit 0
		146,               // short tree bit 1
		172,               // short tree bit 2
		147, 214, 39, 156, // short tree bits 3..6
		128, 129, 132, 75, 145, 178, 206, 239, 254, 254, // long bits
	},
	// Vertical (y) component probabilities
	{
		164,                // is_short
		128,                // sign
		204,                // short tree bit 0
		170,                // short tree bit 1
		119,                // short tree bit 2
		235, 140, 230, 228, // short tree bits 3..6
		128, 130, 130, 74, 148, 180, 203, 236, 254, 254, // long bits
	},
}

// encodeMVComponent encodes a single motion vector component (dx or dy)
// using the VP8 motion vector coding scheme.
// The component is a signed value in quarter-pixel units.
// Reference: RFC 6386 §17
func encodeMVComponent(enc *boolEncoder, val int16, probs [19]uint8) {
	// Sign
	sign := val < 0
	absVal := int(val)
	if sign {
		absVal = -absVal
	}

	// VP8 MVs are encoded as (magnitude - 1) since 0 means "zero MV" which
	// is handled at the mode level. For NEWMV, the delta from predicted MV
	// is encoded. The magnitude is the absolute value of this delta.

	if absVal < 8 {
		// Short encoding (magnitude 0..7)
		enc.putBit(probs[0], false) // is_short = false (short)

		// Encode 3-bit value using the short tree
		// Tree: bit[0] splits {0,1,2,3} vs {4,5,6,7}
		v := absVal
		enc.putBit(probs[2], v >= 4)
		if v >= 4 {
			v -= 4
		}
		enc.putBit(probs[3], v >= 2)
		if v >= 2 {
			v -= 2
		}
		enc.putBit(probs[4], v >= 1)
	} else {
		// Long encoding (magnitude >= 8)
		enc.putBit(probs[0], true) // is_short = true (long)

		// Encode using bit-by-bit scheme for magnitudes >= 8
		// Bits are encoded from MSB to LSB
		v := absVal
		for i := 0; i < 10; i++ {
			bit := (v >> uint(9-i)) & 1
			enc.putBit(probs[9+i], bit == 1)
		}
	}

	// Encode sign if value is non-zero
	if absVal > 0 {
		enc.putBit(probs[1], sign)
	}
}

// encodeMV encodes a full motion vector (dx, dy) as the difference from
// the predicted motion vector.
func encodeMV(enc *boolEncoder, mv, predMV motionVector) {
	dmvX := mv.dx - predMV.dx
	dmvY := mv.dy - predMV.dy

	encodeMVComponent(enc, dmvX, interMVProbs[0])
	encodeMVComponent(enc, dmvY, interMVProbs[1])
}

// VP8 inter-frame macroblock mode probabilities
// Reference: RFC 6386 §11.3
var interMBModeProbs = [4]uint8{
	112, // P(NEARESTMV)
	64,  // P(NEARMV)
	128, // P(ZEROMV)
	// NEWMV is implicit (remaining probability)
}

// encodeInterMBMode encodes the macroblock mode for an inter-frame macroblock.
// For macroblocks that use inter prediction, this encodes the MV mode.
// For intra macroblocks within inter frames, this signals the intra mode.
// Reference: RFC 6386 §11.3
func encodeInterMBMode(enc *boolEncoder, mb *macroblock) {
	if !mb.isInter {
		// Intra macroblock within inter frame
		// is_inter = false
		enc.putBit(63, false) // P(is_inter) - inter frame probability
		// Encode intra y_mode (same as key-frame mode tree)
		encodeYMode(enc, mb.lumaMode, mb.bModes)
		return
	}

	// Inter macroblock: is_inter = true
	enc.putBit(63, true)

	// Encode reference frame (Last, Golden, AltRef)
	// For simplicity, we always use Last reference frame
	// ref_frame tree: last vs {golden, altref}
	switch mb.refFrame {
	case refFrameLast:
		enc.putBit(128, false) // last
	case refFrameGolden:
		enc.putBit(128, true)  // not last
		enc.putBit(128, false) // golden
	case refFrameAltRef:
		enc.putBit(128, true) // not last
		enc.putBit(128, true) // altref
	}

	// Encode inter prediction mode
	// Mode tree: NEARESTMV vs {NEARMV, ZEROMV, NEWMV}
	switch mb.interMode {
	case mvModeNearestMV:
		enc.putBit(interMBModeProbs[0], false) // NEARESTMV
	case mvModeNearMV:
		enc.putBit(interMBModeProbs[0], true)  // not NEARESTMV
		enc.putBit(interMBModeProbs[1], false) // NEARMV
	case mvModeZeroMV:
		enc.putBit(interMBModeProbs[0], true)  // not NEARESTMV
		enc.putBit(interMBModeProbs[1], true)  // not NEARMV
		enc.putBit(interMBModeProbs[2], false) // ZEROMV
	case mvModeNewMV:
		enc.putBit(interMBModeProbs[0], true) // not NEARESTMV
		enc.putBit(interMBModeProbs[1], true) // not NEARMV
		enc.putBit(interMBModeProbs[2], true) // NEWMV
	}

	// If NEWMV, encode the motion vector difference
	if mb.interMode == mvModeNewMV {
		// The MV must be encoded as delta from the predicted MV (typically
		// derived from neighboring macroblocks), so that the decoder, which
		// uses the same predictor, reconstructs the correct absolute MV.
		// The caller is responsible for setting mb.predMV to the chosen
		// predictor during inter processing.
		encodeMV(enc, mb.mv, mb.predMV)
	}
}

// encodeInterFrameHeader encodes the VP8 inter-frame (P-frame) header into
// the first partition. Inter-frame headers differ from key-frame headers in
// several ways per RFC 6386 §9.
func encodeInterFrameHeader(enc *boolEncoder, width, height, qi int, deltas QuantDeltas,
	partCount PartitionCount, loopFilter loopFilterParams, refreshGolden bool, mbs []macroblock,
) {
	encodeInterFrameHeaderWithProbs(enc, width, height, qi, deltas, partCount, loopFilter, refreshGolden, mbs, nil)
}

// encodeInterFrameHeaderWithProbs encodes the inter-frame header with optional probability updates.
func encodeInterFrameHeaderWithProbs(enc *boolEncoder, width, height, qi int, deltas QuantDeltas,
	partCount PartitionCount, loopFilter loopFilterParams, refreshGolden bool, mbs []macroblock, probCfg *ProbConfig,
) {
	encodeCommonFrameHeader(enc, qi, deltas, partCount, loopFilter)
	encodeRefFrameFlags(enc, refreshGolden)
	encodeProbUpdates(enc, probCfg)
	encodeInterFrameProbs(enc)
	encodeMVProbUpdates(enc)
	encodeInterMBModes(enc, width, mbs)
}

// encodeRefFrameFlags encodes reference frame refresh and copy flags.
func encodeRefFrameFlags(enc *boolEncoder, refreshGolden bool) {
	enc.putBit(128, refreshGolden) // refresh_golden_frame
	enc.putBit(128, false)         // refresh_alternate_frame
	enc.putLiteral(0, 2)           // copy_buffer_to_golden
	enc.putLiteral(0, 2)           // copy_buffer_to_alternate
	enc.putBit(128, false)         // sign_bias_golden
	enc.putBit(128, false)         // sign_bias_alternate
	enc.putBit(128, false)         // refresh_entropy_probs
	enc.putBit(128, true)          // refresh_last_frame_buffer
}

// encodeProbUpdates encodes token probability updates if configured.
func encodeProbUpdates(enc *boolEncoder, probCfg *ProbConfig) {
	if probCfg != nil && probCfg.NewProbs != nil && probCfg.CurrentProbs != nil {
		EncodeCoeffProbUpdates(enc, probCfg.CurrentProbs, probCfg.NewProbs)
	} else {
		EncodeNoCoeffProbUpdates(enc)
	}
}

// encodeInterFrameProbs encodes the inter-frame specific probability values.
func encodeInterFrameProbs(enc *boolEncoder) {
	enc.putBit(128, true)  // mb_no_skip_coeff
	enc.putLiteral(255, 8) // prob_skip_false
	enc.putLiteral(63, 8)  // prob_intra
	enc.putLiteral(128, 8) // prob_last
	enc.putLiteral(128, 8) // prob_golden
}

// encodeMVProbUpdates signals no MV probability updates.
func encodeMVProbUpdates(enc *boolEncoder) {
	for i := 0; i < 2; i++ {
		for j := 0; j < 19; j++ {
			enc.putBit(128, false)
		}
	}
}

// encodeInterMBModes encodes macroblock modes for an inter frame.
func encodeInterMBModes(enc *boolEncoder, width int, mbs []macroblock) {
	mbW := (width + 15) / 16
	aboveBModes := make([][4]intraBMode, mbW)
	var leftBModes [4]intraBMode

	for mbIdx, mb := range mbs {
		mbX := mbIdx % mbW
		if mbX == 0 {
			leftBModes = [4]intraBMode{B_DC_PRED, B_DC_PRED, B_DC_PRED, B_DC_PRED}
		}

		enc.putBit(255, mb.skip)
		encodeInterMBModeWithContext(enc, &mb, aboveBModes[mbX], leftBModes)
		updateBPredContext(&mb, aboveBModes, leftBModes, mbX)

		if !mb.isInter {
			encodeUVMode(enc, mb.chromaMode)
		}
	}
}

// updateBPredContext updates B_PRED context for the next macroblock.
func updateBPredContext(mb *macroblock, aboveBModes [][4]intraBMode, leftBModes [4]intraBMode, mbX int) {
	if !mb.isInter && mb.lumaMode == B_PRED {
		for i := 0; i < 4; i++ {
			aboveBModes[mbX][i] = mb.bModes[12+i]
		}
		for i := 0; i < 4; i++ {
			leftBModes[i] = mb.bModes[i*4+3]
		}
	} else {
		aboveBModes[mbX] = [4]intraBMode{B_DC_PRED, B_DC_PRED, B_DC_PRED, B_DC_PRED}
		for i := 0; i < 4; i++ {
			leftBModes[i] = B_DC_PRED
		}
	}
}

// encodeInterMBModeWithContext encodes the macroblock mode for an inter-frame macroblock,
// with proper B_PRED sub-block context from neighboring macroblocks.
func encodeInterMBModeWithContext(enc *boolEncoder, mb *macroblock, aboveModes, leftModes [4]intraBMode) {
	if !mb.isInter {
		// Intra macroblock within inter frame
		// is_inter = false
		enc.putBit(63, false) // P(is_inter) - inter frame probability
		// Encode intra y_mode with proper context
		encodeYModeWithContext(enc, mb.lumaMode, mb.bModes, aboveModes, leftModes)
		return
	}

	// Inter macroblock: is_inter = true
	enc.putBit(63, true)

	// Encode reference frame (Last, Golden, AltRef)
	switch mb.refFrame {
	case refFrameLast:
		enc.putBit(128, false) // last
	case refFrameGolden:
		enc.putBit(128, true)  // not last
		enc.putBit(128, false) // golden
	case refFrameAltRef:
		enc.putBit(128, true) // not last
		enc.putBit(128, true) // altref
	}

	// Encode inter prediction mode
	switch mb.interMode {
	case mvModeNearestMV:
		enc.putBit(interMBModeProbs[0], false) // NEARESTMV
	case mvModeNearMV:
		enc.putBit(interMBModeProbs[0], true)  // not NEARESTMV
		enc.putBit(interMBModeProbs[1], false) // NEARMV
	case mvModeZeroMV:
		enc.putBit(interMBModeProbs[0], true)  // not NEARESTMV
		enc.putBit(interMBModeProbs[1], true)  // not NEARMV
		enc.putBit(interMBModeProbs[2], false) // ZEROMV
	case mvModeNewMV:
		enc.putBit(interMBModeProbs[0], true) // not NEARESTMV
		enc.putBit(interMBModeProbs[1], true) // not NEARMV
		enc.putBit(interMBModeProbs[2], true) // NEWMV
	}

	// If NEWMV, encode the motion vector difference
	if mb.interMode == mvModeNewMV {
		encodeMV(enc, mb.mv, mb.predMV)
	}
}

// BuildInterFrame constructs a complete VP8 inter-frame (P-frame) bitstream.
// Inter frames use a frame tag with key_frame=1 (inter) and do not include
// the start code or dimensions.
//
// If refreshGolden is true, the bitstream signals that the decoder should
// update its golden reference frame from the reconstructed frame.
//
// Reference: RFC 6386 §9.1 (frame tag for inter frames)
func BuildInterFrame(width, height, qi, y1DCDelta, y2DCDelta, y2ACDelta, uvDCDelta, uvACDelta int,
	partCount PartitionCount, loopFilter loopFilterParams, refreshGolden bool, mbs []macroblock,
) ([]byte, error) {
	return buildInterFrameWithProbs(width, height, qi, y1DCDelta, y2DCDelta, y2ACDelta, uvDCDelta, uvACDelta,
		partCount, loopFilter, refreshGolden, mbs, nil)
}

// buildInterFrameWithProbs constructs an inter frame with optional probability configuration.
func buildInterFrameWithProbs(width, height, qi, y1DCDelta, y2DCDelta, y2ACDelta, uvDCDelta, uvACDelta int,
	partCount PartitionCount, loopFilter loopFilterParams, refreshGolden bool, mbs []macroblock, probCfg *ProbConfig,
) ([]byte, error) {
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

	// Encode first partition (inter-frame header + MB modes)
	partEnc := newBoolEncoder()
	encodeInterFrameHeaderWithProbs(partEnc, width, height, qi, deltas, partCount, loopFilter, refreshGolden, mbs, probCfg)
	firstPart := partEnc.flush()

	// Encode residual partitions using shared helper from bitstream.go
	residualParts := encodeResidualPartitionsWithProbs(partCount, mbs, mbW, mbH, probCfg)

	// Build inter frame using shared assembler
	return assembleInterFrameBitstream(firstPart, residualParts), nil
}
