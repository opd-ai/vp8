package vp8

import (
	"testing"
)

func TestTraceFirstPartition(t *testing.T) {
	// First partition from our test: 00 00 00 30 00 00 30 3a 6e
	// (After frame header and keyframe header)

	// Let's manually encode the first partition
	enc := newBoolEncoder()

	// Frame header fields (RFC 6386 §9.2-9.6)
	// colorSpace, clampType (only for keyframes)
	enc.putBit(128, false) // color space = 0
	enc.putBit(128, false) // clamp type = 0

	// Segmentation data (§9.3) - not used
	enc.putBit(128, false) // segmentation_enabled = false

	// Loop filter (§9.4)
	enc.putBit(128, false) // filter_type = 0 (normal)
	enc.putLiteral(0, 6)   // loop_filter_level = 0
	enc.putLiteral(0, 3)   // sharpness_level = 0
	enc.putBit(128, false) // loop_filter_adj_enable = false

	// Partition count (§9.5) - 1 partition = 0
	enc.putLiteral(0, 2)

	// Quantizer (§9.6)
	enc.putLiteral(24, 7)  // y_ac_qi = 24 (default)
	enc.putBit(128, false) // y_dc_delta_present = false
	enc.putBit(128, false) // y2_dc_delta_present = false
	enc.putBit(128, false) // y2_ac_delta_present = false
	enc.putBit(128, false) // uv_dc_delta_present = false
	enc.putBit(128, false) // uv_ac_delta_present = false

	// refresh_golden_frame, etc (§9.7-9.8) - keyframe defaults
	// refresh_last = 1 for keyframe (implied)
	enc.putBit(128, true) // refresh_entropy_probs = true

	// Token probability updates (§9.9, 9.18)
	// This is complex - for now encode "no updates"
	EncodeNoCoeffProbUpdates(enc)

	// Skip prob (§9.11)
	enc.putBit(128, true)  // mb_no_skip_coeff = true (use skip)
	enc.putLiteral(255, 8) // prob_skip_false = 255

	// Now encode macroblock headers for our 2 MBs
	// MB0: skip=false (has coeffs -177), V_PRED
	enc.putBit(255, false) // skip = false
	// Y mode tree for V_PRED
	enc.putBit(145, true)  // is_16x16 = true
	enc.putBit(156, true)  // not DC_PRED
	enc.putBit(163, false) // V_PRED (not H/TM)
	// UV mode tree for DC_PRED_CHROMA
	enc.putBit(142, false) // DC_PRED

	// MB1: skip=false (has coeffs 178), V_PRED
	enc.putBit(255, false) // skip = false
	enc.putBit(145, true)  // is_16x16 = true
	enc.putBit(156, true)  // not DC_PRED
	enc.putBit(163, false) // V_PRED
	enc.putBit(142, false) // DC_PRED_CHROMA

	firstPart := enc.flush()
	t.Logf("First partition (%d bytes): %x", len(firstPart), firstPart)
}
