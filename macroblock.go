package vp8

// macroblock holds the data for one 16x16 VP8 macroblock.
type macroblock struct {
	lumaMode   intraMode
	chromaMode chromaMode
	// skip indicates all quantized coefficients are zero.
	skip bool
	// dcValue is the DC coefficient value (after quantization).
	dcValue int16
}

// processMacroblock returns a macroblock descriptor for a 16x16 luma block.
//
// In this simplified encoder, residuals are not actually encoded in the
// bitstream: the bitstream writer uses fixed intra prediction modes and
// always marks macroblocks as skipped. This function returns a fixed
// descriptor; the signature can be extended when residual coding is added.
func processMacroblock() macroblock {
	return macroblock{
		lumaMode:   DC_PRED,
		chromaMode: DC_PRED_CHROMA,
		skip:       true,
		dcValue:    0, // residuals are not encoded in this encoder
	}
}

