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
	dc := compute16x16DC(above, left)
	for i := 0; i < 256; i++ {
		dst[i] = dc
	}
}

// compute16x16DC calculates the DC value for 16x16 prediction.
func compute16x16DC(above, left []byte) byte {
	haveAbove := len(above) >= 16
	haveLeft := len(left) >= 16

	if !haveAbove && !haveLeft {
		return 128
	}
	if haveAbove && haveLeft {
		return compute16x16DCBoth(above, left)
	}
	if haveAbove {
		return compute16x16DCAbove(above)
	}
	return compute16x16DCLeft(left)
}

// compute16x16DCBoth computes DC from both above and left neighbors.
func compute16x16DCBoth(above, left []byte) byte {
	sum := 0
	for i := 0; i < 16; i++ {
		sum += int(above[i]) + int(left[i])
	}
	return byte((sum + 16) >> 5)
}

// compute16x16DCAbove computes DC from above neighbors only.
func compute16x16DCAbove(above []byte) byte {
	sum := 0
	for i := 0; i < 16; i++ {
		sum += int(above[i])
	}
	return byte((sum + 8) >> 4)
}

// compute16x16DCLeft computes DC from left neighbors only.
func compute16x16DCLeft(left []byte) byte {
	sum := 0
	for i := 0; i < 16; i++ {
		sum += int(left[i])
	}
	return byte((sum + 8) >> 4)
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
	aboveRow, tl := prepareAboveContext16(above, topLeft)
	leftCol, tl := prepareLeftContext16(left, tl)
	fillTMPrediction(dst, aboveRow[:], leftCol[:], tl, 16)
}

// prepareAboveContext16 prepares the above row for 16x16 TM prediction.
func prepareAboveContext16(above []byte, topLeft byte) ([16]byte, int) {
	var aboveRow [16]byte
	tl := int(topLeft)
	if len(above) >= 16 {
		copy(aboveRow[:], above[:16])
	} else {
		for i := range aboveRow {
			aboveRow[i] = 127
		}
		tl = 127
	}
	return aboveRow, tl
}

// prepareLeftContext16 prepares the left column for 16x16 TM prediction.
func prepareLeftContext16(left []byte, tl int) ([16]byte, int) {
	var leftCol [16]byte
	if len(left) >= 16 {
		copy(leftCol[:], left[:16])
	} else {
		for i := range leftCol {
			leftCol[i] = 129
		}
		tl = 129
	}
	return leftCol, tl
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
	dc := compute8x8DC(above, left)
	fill8x8Block(dst, dc)
}

// compute8x8DC computes the DC value for 8x8 chroma prediction.
func compute8x8DC(above, left []byte) byte {
	haveAbove := len(above) >= 8
	haveLeft := len(left) >= 8

	if !haveAbove && !haveLeft {
		return 128
	}
	if haveAbove && haveLeft {
		return byte((sum8(above) + sum8(left) + 8) >> 4)
	}
	if haveAbove {
		return byte((sum8(above) + 4) >> 3)
	}
	return byte((sum8(left) + 4) >> 3)
}

// sum8 computes the sum of the first 8 bytes.
func sum8(data []byte) int {
	sum := 0
	for i := 0; i < 8; i++ {
		sum += int(data[i])
	}
	return sum
}

// fill8x8Block fills an 8x8 block with a single value.
func fill8x8Block(dst []byte, val byte) {
	for i := 0; i < 64; i++ {
		dst[i] = val
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
	aboveRow, tl := prepareAboveContext8(above, topLeft)
	leftCol, tl := prepareLeftContext8(left, tl)
	fillTMPrediction(dst, aboveRow[:], leftCol[:], tl, 8)
}

// prepareAboveContext8 prepares the above row for 8x8 TM prediction.
func prepareAboveContext8(above []byte, topLeft byte) ([8]byte, int) {
	var aboveRow [8]byte
	tl := int(topLeft)
	if len(above) >= 8 {
		copy(aboveRow[:], above[:8])
	} else {
		for i := range aboveRow {
			aboveRow[i] = 127
		}
		tl = 127
	}
	return aboveRow, tl
}

// prepareLeftContext8 prepares the left column for 8x8 TM prediction.
func prepareLeftContext8(left []byte, tl int) ([8]byte, int) {
	var leftCol [8]byte
	if len(left) >= 8 {
		copy(leftCol[:], left[:8])
	} else {
		for i := range leftCol {
			leftCol[i] = 129
		}
		tl = 129
	}
	return leftCol, tl
}

// fillTMPrediction fills a block using TrueMotion prediction formula.
func fillTMPrediction(dst, above, left []byte, tl, size int) {
	for r := 0; r < size; r++ {
		for c := 0; c < size; c++ {
			val := int(left[r]) + int(above[c]) - tl
			dst[r*size+c] = clamp8(val)
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
