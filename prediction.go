package vp8

// intraMode represents the VP8 intra prediction mode for a macroblock.
type intraMode uint8

const (
	// DC_PRED uses the average of above and left boundary pixels.
	DC_PRED intraMode = iota
	// V_PRED uses vertical replication of the row above.
	V_PRED
	// H_PRED uses horizontal replication of the column to the left.
	H_PRED
	// TM_PRED is the TrueMotion predictor.
	TM_PRED
	// B_PRED signals 4x4 per-subblock intra prediction.
	B_PRED
)

// chromaMode represents the VP8 intra prediction mode for chroma blocks.
type chromaMode uint8

const (
	DC_PRED_CHROMA chromaMode = iota
	V_PRED_CHROMA
	H_PRED_CHROMA
	TM_PRED_CHROMA
)

// predictDC fills a 16x16 block with a DC value derived from above/left samples.
// For simplicity (boundary macroblocks with no neighbours) we use 128.
func predictDC(above []byte, left []byte, haveAbove, haveLeft bool) byte {
	if !haveAbove && !haveLeft {
		return 128
	}
	sum := 0
	n := 0
	if haveAbove {
		for _, v := range above {
			sum += int(v)
		}
		n += len(above)
	}
	if haveLeft {
		for _, v := range left {
			sum += int(v)
		}
		n += len(left)
	}
	return byte((sum + n/2) / n)
}

// clamp8 clamps v to the range [0, 255].
func clamp8(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}
