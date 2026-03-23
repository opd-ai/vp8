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

// Predict16x16 fills a 16x16 prediction buffer using the specified intra mode.
// The buffer dst must have at least 256 bytes (16x16).
// Parameters:
//   - dst: destination buffer (16x16 = 256 bytes, row-major order)
//   - above: 16-pixel row immediately above the block (nil if not available)
//   - left: 16-pixel column immediately to the left (nil if not available)
//   - topLeft: pixel at position (-1, -1), above-left corner
//   - mode: prediction mode (DC_PRED, V_PRED, H_PRED, or TM_PRED)
//
// Reference: RFC 6386 §12.3
func Predict16x16(dst, above, left []byte, topLeft byte, mode intraMode) {
	switch mode {
	case DC_PRED:
		predict16x16DC(dst, above, left)
	case V_PRED:
		predict16x16V(dst, above)
	case H_PRED:
		predict16x16H(dst, left)
	case TM_PRED:
		predict16x16TM(dst, above, left, topLeft)
	}
}

// predict16x16DC fills a 16x16 block with a DC value derived from above/left.
// Reference: RFC 6386 §12.3 (DC_PRED for luma)
func predict16x16DC(dst, above, left []byte) {
	var dc byte
	haveAbove := len(above) >= 16
	haveLeft := len(left) >= 16

	if !haveAbove && !haveLeft {
		dc = 128
	} else if haveAbove && haveLeft {
		sum := 0
		for i := 0; i < 16; i++ {
			sum += int(above[i]) + int(left[i])
		}
		dc = byte((sum + 16) >> 5) // average of 32 pixels, rounding
	} else if haveAbove {
		sum := 0
		for i := 0; i < 16; i++ {
			sum += int(above[i])
		}
		dc = byte((sum + 8) >> 4) // average of 16 pixels, rounding
	} else { // haveLeft
		sum := 0
		for i := 0; i < 16; i++ {
			sum += int(left[i])
		}
		dc = byte((sum + 8) >> 4) // average of 16 pixels, rounding
	}

	// Fill entire 16x16 block with the DC value
	for i := 0; i < 256; i++ {
		dst[i] = dc
	}
}

// predict16x16V fills a 16x16 block by replicating the row above.
// Reference: RFC 6386 §12.3 (V_PRED for luma)
func predict16x16V(dst, above []byte) {
	// If above is not available, use 127 as per RFC 6386
	var row [16]byte
	if len(above) >= 16 {
		copy(row[:], above[:16])
	} else {
		for i := range row {
			row[i] = 127
		}
	}

	// Copy the above row to all 16 rows
	for r := 0; r < 16; r++ {
		copy(dst[r*16:r*16+16], row[:])
	}
}

// predict16x16H fills a 16x16 block by replicating the column to the left.
// Reference: RFC 6386 §12.3 (H_PRED for luma)
func predict16x16H(dst, left []byte) {
	// If left is not available, use 129 as per RFC 6386
	var col [16]byte
	if len(left) >= 16 {
		copy(col[:], left[:16])
	} else {
		for i := range col {
			col[i] = 129
		}
	}

	// Fill each row with the corresponding left pixel
	for r := 0; r < 16; r++ {
		for c := 0; c < 16; c++ {
			dst[r*16+c] = col[r]
		}
	}
}

// predict16x16TM fills a 16x16 block using TrueMotion prediction.
// TM_PRED: X[r][c] = clamp(left[r] + above[c] - topLeft)
// Reference: RFC 6386 §12.3 (TM_PRED for luma)
func predict16x16TM(dst, above, left []byte, topLeft byte) {
	// Handle missing neighbors per RFC 6386
	var aboveRow [16]byte
	var leftCol [16]byte
	tl := int(topLeft)

	if len(above) >= 16 {
		copy(aboveRow[:], above[:16])
	} else {
		for i := range aboveRow {
			aboveRow[i] = 127
		}
		tl = 127 // Also use 127 for topLeft when above is missing
	}

	if len(left) >= 16 {
		copy(leftCol[:], left[:16])
	} else {
		for i := range leftCol {
			leftCol[i] = 129
		}
		tl = 129 // Use 129 for topLeft when left is missing
	}

	for r := 0; r < 16; r++ {
		for c := 0; c < 16; c++ {
			// X[r][c] = clamp(left[r] + above[c] - topLeft)
			val := int(leftCol[r]) + int(aboveRow[c]) - tl
			dst[r*16+c] = clamp8(val)
		}
	}
}

// Predict8x8Chroma fills an 8x8 chroma prediction buffer using the specified mode.
// The buffer dst must have at least 64 bytes (8x8).
// Parameters:
//   - dst: destination buffer (8x8 = 64 bytes, row-major order)
//   - above: 8-pixel row immediately above the block (nil if not available)
//   - left: 8-pixel column immediately to the left (nil if not available)
//   - topLeft: pixel at position (-1, -1), above-left corner
//   - mode: prediction mode (DC_PRED_CHROMA, V_PRED_CHROMA, H_PRED_CHROMA, TM_PRED_CHROMA)
//
// Reference: RFC 6386 §12.2
func Predict8x8Chroma(dst, above, left []byte, topLeft byte, mode chromaMode) {
	switch mode {
	case DC_PRED_CHROMA:
		predict8x8DC(dst, above, left)
	case V_PRED_CHROMA:
		predict8x8V(dst, above)
	case H_PRED_CHROMA:
		predict8x8H(dst, left)
	case TM_PRED_CHROMA:
		predict8x8TM(dst, above, left, topLeft)
	}
}

// predict8x8DC fills an 8x8 chroma block with a DC value.
// Reference: RFC 6386 §12.2
func predict8x8DC(dst, above, left []byte) {
	var dc byte
	haveAbove := len(above) >= 8
	haveLeft := len(left) >= 8

	if !haveAbove && !haveLeft {
		dc = 128
	} else if haveAbove && haveLeft {
		sum := 0
		for i := 0; i < 8; i++ {
			sum += int(above[i]) + int(left[i])
		}
		dc = byte((sum + 8) >> 4) // average of 16 pixels, rounding
	} else if haveAbove {
		sum := 0
		for i := 0; i < 8; i++ {
			sum += int(above[i])
		}
		dc = byte((sum + 4) >> 3) // average of 8 pixels, rounding
	} else { // haveLeft
		sum := 0
		for i := 0; i < 8; i++ {
			sum += int(left[i])
		}
		dc = byte((sum + 4) >> 3) // average of 8 pixels, rounding
	}

	// Fill entire 8x8 block with the DC value
	for i := 0; i < 64; i++ {
		dst[i] = dc
	}
}

// predict8x8V fills an 8x8 chroma block by replicating the row above.
// Reference: RFC 6386 §12.2
func predict8x8V(dst, above []byte) {
	var row [8]byte
	if len(above) >= 8 {
		copy(row[:], above[:8])
	} else {
		for i := range row {
			row[i] = 127
		}
	}

	for r := 0; r < 8; r++ {
		copy(dst[r*8:r*8+8], row[:])
	}
}

// predict8x8H fills an 8x8 chroma block by replicating the column to the left.
// Reference: RFC 6386 §12.2
func predict8x8H(dst, left []byte) {
	var col [8]byte
	if len(left) >= 8 {
		copy(col[:], left[:8])
	} else {
		for i := range col {
			col[i] = 129
		}
	}

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			dst[r*8+c] = col[r]
		}
	}
}

// predict8x8TM fills an 8x8 chroma block using TrueMotion prediction.
// Reference: RFC 6386 §12.2
func predict8x8TM(dst, above, left []byte, topLeft byte) {
	var aboveRow [8]byte
	var leftCol [8]byte
	tl := int(topLeft)

	if len(above) >= 8 {
		copy(aboveRow[:], above[:8])
	} else {
		for i := range aboveRow {
			aboveRow[i] = 127
		}
		tl = 127
	}

	if len(left) >= 8 {
		copy(leftCol[:], left[:8])
	} else {
		for i := range leftCol {
			leftCol[i] = 129
		}
		tl = 129
	}

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			val := int(leftCol[r]) + int(aboveRow[c]) - tl
			dst[r*8+c] = clamp8(val)
		}
	}
}

// SelectBest16x16Mode evaluates all 16x16 prediction modes and returns the one
// with the lowest Sum of Absolute Differences (SAD) compared to the source block.
// Parameters:
//   - src: source 16x16 luma block (256 bytes, row-major)
//   - above: 16-pixel row above (nil if not available)
//   - left: 16-pixel column to left (nil if not available)
//   - topLeft: corner pixel above and to the left
//
// Returns the best mode and the SAD value for that mode.
func SelectBest16x16Mode(src, above, left []byte, topLeft byte) (intraMode, int) {
	var pred [256]byte
	bestMode := DC_PRED
	bestSAD := 1 << 30 // Large initial value

	modes := []intraMode{DC_PRED, V_PRED, H_PRED, TM_PRED}
	for _, mode := range modes {
		Predict16x16(pred[:], above, left, topLeft, mode)
		sad := computeSAD16x16(src, pred[:])
		if sad < bestSAD {
			bestSAD = sad
			bestMode = mode
		}
	}

	return bestMode, bestSAD
}

// computeSAD16x16 computes Sum of Absolute Differences between two 16x16 blocks.
func computeSAD16x16(a, b []byte) int {
	sad := 0
	for i := 0; i < 256; i++ {
		diff := int(a[i]) - int(b[i])
		if diff < 0 {
			diff = -diff
		}
		sad += diff
	}
	return sad
}

// SelectBest8x8ChromaMode evaluates all 8x8 chroma prediction modes and returns
// the one with the lowest SAD compared to the source block.
func SelectBest8x8ChromaMode(src, above, left []byte, topLeft byte) (chromaMode, int) {
	var pred [64]byte
	bestMode := DC_PRED_CHROMA
	bestSAD := 1 << 30

	modes := []chromaMode{DC_PRED_CHROMA, V_PRED_CHROMA, H_PRED_CHROMA, TM_PRED_CHROMA}
	for _, mode := range modes {
		Predict8x8Chroma(pred[:], above, left, topLeft, mode)
		sad := computeSAD8x8(src, pred[:])
		if sad < bestSAD {
			bestSAD = sad
			bestMode = mode
		}
	}

	return bestMode, bestSAD
}

// computeSAD8x8 computes Sum of Absolute Differences between two 8x8 blocks.
func computeSAD8x8(a, b []byte) int {
	sad := 0
	for i := 0; i < 64; i++ {
		diff := int(a[i]) - int(b[i])
		if diff < 0 {
			diff = -diff
		}
		sad += diff
	}
	return sad
}
