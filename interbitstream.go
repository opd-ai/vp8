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
		162, // is_short
		128, // sign
		225, // short tree bit 0
		146, // short tree bit 1
		172, // short tree bit 2
		147, 214, 39, 156, // short tree bits 3..6
		128, 129, 132, 75, 145, 178, 206, 239, 254, 254, // long bits
	},
	// Vertical (y) component probabilities
	{
		164, // is_short
		128, // sign
		204, // short tree bit 0
		170, // short tree bit 1
		119, // short tree bit 2
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
		enc.putBit(128, true) // not last
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
		enc.putBit(interMBModeProbs[0], true) // not NEARESTMV
		enc.putBit(interMBModeProbs[1], true) // not NEARMV
		enc.putBit(interMBModeProbs[2], false) // ZEROMV
	case mvModeNewMV:
		enc.putBit(interMBModeProbs[0], true) // not NEARESTMV
		enc.putBit(interMBModeProbs[1], true) // not NEARMV
		enc.putBit(interMBModeProbs[2], true) // NEWMV
	}

	// If NEWMV, encode the motion vector difference
	if mb.interMode == mvModeNewMV {
		// The MV is encoded as delta from the "best" predicted MV (nearest)
		// For simplicity, use zero as the prediction base; actual encoding
		// should use nearest MV from neighbors.
		// Note: The actual prediction is handled by the caller setting up
		// the appropriate predMV, but for the bitstream we encode the full MV.
		encodeMV(enc, mb.mv, zeroMV)
	}
}

// encodeInterFrameHeader encodes the VP8 inter-frame (P-frame) header into
// the first partition. Inter-frame headers differ from key-frame headers in
// several ways per RFC 6386 §9.
func encodeInterFrameHeader(enc *boolEncoder, width, height, qi int, deltas QuantDeltas,
	partCount PartitionCount, mbs []macroblock) {

	// Segmentation (1 bit): disabled
	enc.putBit(128, false)

	// Filter type (1 bit): 0 = normal
	enc.putBit(128, false)
	// Loop filter level (6 bits): 0 (disabled for now)
	enc.putLiteral(0, 6)
	// Sharpness level (3 bits)
	enc.putLiteral(0, 3)

	// Loop filter delta flags (1 bit): 0 = no delta
	enc.putBit(128, false)

	// Number of DCT partitions (2 bits)
	enc.putLiteral(uint32(partCount), 2)

	// Quantizer index (7 bits)
	enc.putLiteral(uint32(qi), 7)
	// Quantizer deltas
	encodeDelta(enc, deltas.Y1DC)
	encodeDelta(enc, deltas.Y2DC)
	encodeDelta(enc, deltas.Y2AC)
	encodeDelta(enc, deltas.UVDC)
	encodeDelta(enc, deltas.UVAC)

	// Reference frame refresh flags (for inter frames)
	// refresh_golden_frame (1 bit): 0 = don't refresh
	enc.putBit(128, false)
	// refresh_alternate_frame (1 bit): 0 = don't refresh
	enc.putBit(128, false)

	// Copy buffer flags (when not refreshing):
	// copy_buffer_to_golden (2 bits): 0 = no copy
	enc.putLiteral(0, 2)
	// copy_buffer_to_alternate (2 bits): 0 = no copy
	enc.putLiteral(0, 2)

	// sign_bias_golden (1 bit): 0
	enc.putBit(128, false)
	// sign_bias_alternate (1 bit): 0
	enc.putBit(128, false)

	// refresh_entropy_probs (1 bit): 0 = don't save probs
	enc.putBit(128, false)

	// refresh_last_frame_buffer (1 bit): 1 = refresh last buffer
	enc.putBit(128, true)

	// Token probability updates
	EncodeNoCoeffProbUpdates(enc)

	// mb_no_skip_coeff (1 bit): 1 = use skip flag
	enc.putBit(128, true)
	// prob_skip_false (8 bits)
	enc.putLiteral(255, 8)

	// prob_intra (8 bits): probability of intra mode within inter frame
	enc.putLiteral(63, 8)

	// prob_last (8 bits): probability of using last reference vs golden/altref
	enc.putLiteral(128, 8)

	// prob_golden (8 bits): probability of golden vs altref
	enc.putLiteral(128, 8)

	// intra_16x16_mode_probs (no update flag) - not transmitted for inter frames
	// These use the key-frame defaults

	// intra_chroma_mode_probs (no update flag)

	// MV probability update flag (1 bit per component per probability)
	// For simplicity, signal no MV probability updates
	for i := 0; i < 2; i++ {
		for j := 0; j < 19; j++ {
			enc.putBit(128, false) // no update
		}
	}

	// Macroblock modes
	for _, mb := range mbs {
		// Skip flag
		enc.putBit(255, mb.skip)

		// Encode macroblock mode
		encodeInterMBMode(enc, &mb)

		// For intra macroblocks, also encode chroma mode
		if !mb.isInter {
			encodeUVMode(enc, mb.chromaMode)
		}
	}
}

// BuildInterFrame constructs a complete VP8 inter-frame (P-frame) bitstream.
// Inter frames use a frame tag with key_frame=1 (inter) and do not include
// the start code or dimensions.
//
// Reference: RFC 6386 §9.1 (frame tag for inter frames)
func BuildInterFrame(width, height, qi, y1DCDelta, y2DCDelta, y2ACDelta, uvDCDelta, uvACDelta int,
	partCount PartitionCount, mbs []macroblock) ([]byte, error) {

	if width <= 0 || height <= 0 || width%2 != 0 || height%2 != 0 {
		return nil, errInvalidDimensions
	}

	mbW := (width + 15) / 16

	deltas := QuantDeltas{
		Y1DC: y1DCDelta,
		Y2DC: y2DCDelta,
		Y2AC: y2ACDelta,
		UVDC: uvDCDelta,
		UVAC: uvACDelta,
	}

	// Encode first partition (inter-frame header + MB modes)
	partEnc := newBoolEncoder()
	encodeInterFrameHeader(partEnc, width, height, qi, deltas, partCount, mbs)
	firstPart := partEnc.flush()

	// Encode residual partitions (same codec as key frames)
	coeffProbs := DefaultCoeffProbs
	var residualParts [][]byte

	if partCount == OnePartition {
		residualEnc := newBoolEncoder()
		tokenEnc := NewTokenEncoder(residualEnc, &coeffProbs)
		encodeResidualPartition(tokenEnc, mbs, mbW)
		residualParts = [][]byte{residualEnc.flush()}
	} else {
		mbH := (height + 15) / 16
		pw := NewPartitionWriter(partCount, &coeffProbs)
		encodeResidualMultiPartition(pw, mbs, mbW, mbH)
		residualParts = pw.Finalize()
	}

	firstPartSize := len(firstPart)

	// Frame tag (3 bytes, little-endian) for inter frame:
	//   bits [0]:     key_frame = 1 (inter frame)
	//   bits [3:1]:   version = 0
	//   bits [4]:     show_frame = 1
	//   bits [23:5]:  first_part_size
	tag := uint32(1)                  // key_frame = 1 (inter frame)
	tag |= 0 << 1                     // version = 0
	tag |= 1 << 4                     // show_frame = 1
	tag |= uint32(firstPartSize) << 5 // first_part_size

	// Inter frames do NOT have the start code or dimensions
	partSizes := BuildPartitionSizes(residualParts)
	residualData := ConcatPartitions(residualParts)
	totalSize := 3 + firstPartSize + len(partSizes) + len(residualData)

	out := make([]byte, 0, totalSize)
	out = append(out, byte(tag), byte(tag>>8), byte(tag>>16))

	// First partition data
	out = append(out, firstPart...)

	// Partition sizes (if multiple partitions)
	out = append(out, partSizes...)

	// Residual partition data
	out = append(out, residualData...)

	return out, nil
}
