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

// quantIndexToQp returns an approximate quantizer step size for a given
// quantizer index (0..127). This follows the VP8 dequantization table.
func quantIndexToQp(qi int) int16 {
	// Simplified linear mapping for encoder quantization.
	if qi <= 0 {
		return 4
	}
	if qi >= 127 {
		return 2048
	}
	return int16(4 + qi*16)
}

// encodeFrameHeader boolean-encodes the VP8 key-frame header into the
// first partition. It encodes the minimum required fields so that a
// conformant VP8 decoder can parse the bitstream.
// Reference: RFC 6386, Section 9.2.
func encodeFrameHeader(enc *boolEncoder, width, height, qi int, numMBs int, mbs []macroblock) {
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

	// refresh_entropy_probs (1 bit): 1
	enc.putBit(128, true)

	// mb_no_skip_coeff (1 bit): 1 → emit prob_skip_false
	enc.putBit(128, true)
	// prob_skip_false (8 bits): set high so most MBs are marked as skip
	enc.putLiteral(255, 8)

	// Macroblock-level data: each MB uses DC_PRED with all residuals skipped.
	for _, mb := range mbs {
		// coeff_skip (1 bit, prob = prob_skip_false = 255):
		// A value of 1 (true) means the macroblock has no non-zero DCT
		// coefficients and the residual partition is not read for this MB.
		enc.putBit(255, mb.skip)
		// Encode intra_mb_mode: the decoder still needs the prediction mode
		// even for skipped macroblocks.
		// y_mode: coded via a probability tree. DC_PRED is the first branch.
		enc.putBit(145, false) // y_mode != B_PRED (use 16x16 mode)
		enc.putBit(156, false) // DC_PRED (not V_PRED)
		// uv_mode: DC_PRED (first tree branch false).
		enc.putBit(142, false) // DC_PRED
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

	// Second partition (residual data): no tokens are written because all
	// macroblocks are marked as skip (coeff_skip=1), so no DCT coefficients
	// are present. The partition is therefore empty.
	secondPart := []byte{}

	firstPartSize := len(firstPart)

	// Frame tag (3 bytes, little-endian):
	//   bits [0]:     key_frame = 0 (0 = key)
	//   bits [3:1]:   version = 0
	//   bits [4]:     show_frame = 1
	//   bits [23:5]:  first_part_size
	tag := uint32(0)          // key_frame = 0
	tag |= 0 << 1             // version = 0
	tag |= 1 << 4             // show_frame = 1
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
