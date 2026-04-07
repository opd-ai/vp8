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
// using the VP8 motion vector coding scheme per RFC 6386 §17.1.
// The component is a signed value in quarter-pixel units.
//
// Probability table offsets:
//   - probs[0]: mvpis_short (short vs long)
//   - probs[1]: sign
//   - probs[2..8]: short tree (7 probabilities for 8 values)
//   - probs[9..18]: long bits (10 probabilities for bits 0-9)
func encodeMVComponent(enc *boolEncoder, val int16, probs [19]uint8) {
	sign := val < 0
	absVal := int(val)
	if sign {
		absVal = -absVal
	}

	if absVal < 8 {
		// Short encoding (magnitude 0..7): use 7-node tree at probs[2..8]
		enc.putBit(probs[0], false) // is_short = false (short)
		encodeSmallMV(enc, absVal, probs)
	} else {
		// Long encoding (magnitude >= 8)
		enc.putBit(probs[0], true) // is_short = true (long)
		encodeLargeMV(enc, absVal, probs)
	}

	// Encode sign if value is non-zero
	if absVal > 0 {
		enc.putBit(probs[1], sign)
	}
}

// encodeSmallMV encodes MV magnitude 0-7 using the RFC 6386 §17.1 small_mvtree.
// Tree structure (probs[2..8]):
//
//	       [0]
//	      /   \
//	    [1]   [4]
//	   /   \  /   \
//	 [2]  [3][5]  [6]
//	 /\   /\  /\   /\
//	0  1 2  3 4  5 6  7
//
// Node indices: 0->probs[2], 1->probs[3], 2->probs[4], 3->probs[5],
//
//	4->probs[6], 5->probs[7], 6->probs[8]
func encodeSmallMV(enc *boolEncoder, v int, probs [19]uint8) {
	// First split: {0,1,2,3} vs {4,5,6,7}
	enc.putBit(probs[2], v >= 4)
	if v >= 4 {
		// Right subtree {4,5,6,7}
		v -= 4
		// Split: {4,5} vs {6,7}
		enc.putBit(probs[6], v >= 2)
		if v >= 2 {
			// {6,7}
			enc.putBit(probs[8], v == 3) // 6 or 7
		} else {
			// {4,5}
			enc.putBit(probs[7], v == 1) // 4 or 5
		}
	} else {
		// Left subtree {0,1,2,3}
		// Split: {0,1} vs {2,3}
		enc.putBit(probs[3], v >= 2)
		if v >= 2 {
			// {2,3}
			enc.putBit(probs[5], v == 3) // 2 or 3
		} else {
			// {0,1}
			enc.putBit(probs[4], v == 1) // 0 or 1
		}
	}
}

// encodeLargeMV encodes MV magnitude >= 8 per RFC 6386 §17.1.
// For long values, bits are encoded in a specific order:
//   - Bits 0, 1, 2 (using probs[9], probs[10], probs[11])
//   - Bits 9, 8, 7, 6, 5, 4 (using probs[18], probs[17], probs[16], probs[15], probs[14], probs[13])
//   - Bit 3 is conditionally encoded (using probs[12]) only if needed
//
// Since the value is >= 8, if the high bits (bits 4-9) are all zero, bit 3 must
// be 1 and is not explicitly coded.
func encodeLargeMV(enc *boolEncoder, v int, probs [19]uint8) {
	// Encode bits 0, 1, 2 (LSBs)
	for i := 0; i < 3; i++ {
		bit := (v >> i) & 1
		enc.putBit(probs[9+i], bit == 1)
	}

	// Encode bits 9, 8, 7, 6, 5, 4 (high bits, descending order)
	for i := 9; i >= 4; i-- {
		bit := (v >> i) & 1
		enc.putBit(probs[9+i], bit == 1)
	}

	// Bit 3: only encode if high bits (4-9) are non-zero
	// If v <= 15 (only bits 0-3 set) and v >= 8, then bit 3 must be 1
	// and is implicitly known to the decoder
	if v&0xFFF0 != 0 {
		// High bits are non-zero, so bit 3 must be explicitly coded
		bit := (v >> 3) & 1
		enc.putBit(probs[12], bit == 1)
	}
	// If high bits are zero, bit 3 is implicitly 1 (since v >= 8)
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
		updateBPredContext(&mb, aboveBModes, &leftBModes, mbX)

		if !mb.isInter {
			encodeUVMode(enc, mb.chromaMode)
		}
	}
}

// updateBPredContext updates B_PRED context for the next macroblock.
// Note: leftBModes is passed by pointer to allow updates to propagate to the caller.
func updateBPredContext(mb *macroblock, aboveBModes [][4]intraBMode, leftBModes *[4]intraBMode, mbX int) {
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
