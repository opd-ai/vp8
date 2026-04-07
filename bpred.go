package vp8

// intraBMode represents the VP8 intra prediction sub-mode for 4×4 luma blocks.
// Used when the macroblock mode is B_PRED.
type intraBMode uint8

// VP8 4×4 intra prediction sub-modes as defined in RFC 6386 §12.3.
// The ordering must match golang.org/x/image/vp8 decoder constants:
// predDC=0, predTM=1, predVE=2, predHE=3, predRD=4, predVR=5, predLD=6, predVL=7, predHD=8, predHU=9
// Constants use underscore naming (e.g., B_DC_PRED) to match RFC 6386 terminology,
// deviating from Go's MixedCaps convention for clarity when referencing the spec.
const (
	// B_DC_PRED predicts DC using row above and column to the left.
	B_DC_PRED intraBMode = iota
	// B_TM_PRED propagates second differences a la "True Motion".
	B_TM_PRED
	// B_VE_PRED predicts rows using averaged row above.
	B_VE_PRED
	// B_HE_PRED predicts columns using averaged column to the left.
	B_HE_PRED
	// B_RD_PRED is southeast (right and down) 45° diagonal prediction.
	B_RD_PRED
	// B_VR_PRED is SSE (vertical right) diagonal prediction.
	B_VR_PRED
	// B_LD_PRED is southwest (left and down) 45° diagonal prediction.
	B_LD_PRED
	// B_VL_PRED is SSW (vertical left) diagonal prediction.
	B_VL_PRED
	// B_HD_PRED is ESE (horizontal down) diagonal prediction.
	B_HD_PRED
	// B_HU_PRED is ENE (horizontal up) diagonal prediction.
	B_HU_PRED

	numIntraBModes = 10
)

// Predict4x4 fills a 4x4 prediction buffer using the specified intra B-mode.
// Parameters:
//   - dst: destination buffer (16 bytes, row-major order)
//   - above: 8-pixel row immediately above the block (A[-1] through A[7])
//     where A[-1] is also the topLeft (P) pixel
//   - left: 4-pixel column immediately to the left (L[0] through L[3])
//   - mode: prediction sub-mode
//
// Reference: RFC 6386 §12.3
func Predict4x4(dst, above, left []byte, mode intraBMode) {
	// Build the E array (edge pixels) for diagonal modes
	// E[0]=L[3], E[1]=L[2], E[2]=L[1], E[3]=L[0], E[4]=P, E[5]=A[0], E[6]=A[1], E[7]=A[2], E[8]=A[3]
	var E [9]byte
	if len(left) >= 4 {
		E[0] = left[3]
		E[1] = left[2]
		E[2] = left[1]
		E[3] = left[0]
	}
	if len(above) >= 1 {
		E[4] = above[0] // P = A[-1] in the above array, but we use above[0] as topLeft
	}
	// Adjust: above array is expected to have A[-1]=topLeft at index 0, A[0..7] at indices 1..8
	if len(above) >= 5 {
		E[4] = above[0] // topLeft (P)
		E[5] = above[1] // A[0]
		E[6] = above[2] // A[1]
		E[7] = above[3] // A[2]
		E[8] = above[4] // A[3]
	}

	switch mode {
	case B_DC_PRED:
		predict4x4DC(dst, above, left)
	case B_TM_PRED:
		predict4x4TM(dst, above, left)
	case B_VE_PRED:
		predict4x4VE(dst, above)
	case B_HE_PRED:
		predict4x4HE(dst, above, left)
	case B_LD_PRED:
		predict4x4LD(dst, above)
	case B_RD_PRED:
		predict4x4RD(dst, E[:])
	case B_VR_PRED:
		predict4x4VR(dst, E[:])
	case B_VL_PRED:
		predict4x4VL(dst, above)
	case B_HD_PRED:
		predict4x4HD(dst, E[:])
	case B_HU_PRED:
		predict4x4HU(dst, left)
	}
}

// avg2 computes (x + y + 1) >> 1
func avg2(x, y byte) byte {
	return byte((int(x) + int(y) + 1) >> 1)
}

// avg3 computes (x + y + y + z + 2) >> 2
func avg3(x, y, z byte) byte {
	return byte((int(x) + int(y)*2 + int(z) + 2) >> 2)
}

// extractAbove8 extracts up to 8 pixels from the above array (A[0..7]).
// The above array has topLeft at index 0 and A[0..n] at indices 1..n+1.
func extractAbove8(above []byte) [8]byte {
	var A [8]byte
	if len(above) >= 9 {
		copy(A[:], above[1:9])
	} else if len(above) >= 5 {
		copy(A[:4], above[1:5])
		extendLast4(A[:], A[3])
	} else {
		fill8Bytes(&A, 127)
	}
	return A
}

// extendLast4 fills the last 4 elements with the given value.
func extendLast4(buf []byte, val byte) {
	for i := 4; i < 8; i++ {
		buf[i] = val
	}
}

// fill8Bytes fills an 8-byte array with a single value.
func fill8Bytes(buf *[8]byte, val byte) {
	for i := range buf {
		buf[i] = val
	}
}

// predict4x4DC fills a 4x4 block with a DC value.
func predict4x4DC(dst, above, left []byte) {
	dc := compute4x4DC(above, left)
	for i := 0; i < 16; i++ {
		dst[i] = dc
	}
}

// compute4x4DC calculates the DC value for 4x4 prediction.
func compute4x4DC(above, left []byte) byte {
	haveAbove := len(above) >= 5
	haveLeft := len(left) >= 4

	if !haveAbove && !haveLeft {
		return 128
	}
	if haveAbove && haveLeft {
		return computeDCBoth(above, left)
	}
	if haveAbove {
		return computeDCAbove(above)
	}
	return computeDCLeft(left)
}

// computeDCBoth computes DC from both above and left neighbors.
func computeDCBoth(above, left []byte) byte {
	sum := 0
	for i := 1; i <= 4; i++ {
		sum += int(above[i])
	}
	for i := 0; i < 4; i++ {
		sum += int(left[i])
	}
	return byte((sum + 4) >> 3)
}

// computeDCAbove computes DC from above neighbors only.
func computeDCAbove(above []byte) byte {
	sum := 0
	for i := 1; i <= 4; i++ {
		sum += int(above[i])
	}
	return byte((sum + 2) >> 2)
}

// computeDCLeft computes DC from left neighbors only.
func computeDCLeft(left []byte) byte {
	sum := 0
	for i := 0; i < 4; i++ {
		sum += int(left[i])
	}
	return byte((sum + 2) >> 2)
}

// predict4x4TM fills a 4x4 block using TrueMotion prediction.
func predict4x4TM(dst, above, left []byte) {
	P, A := extract4x4TMAbove(above)
	L := extract4x4TMLeft(left, &P)

	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			val := int(L[r]) + int(A[c]) - int(P)
			dst[r*4+c] = clamp8(val)
		}
	}
}

// extract4x4TMAbove extracts above pixels and top-left for TM prediction.
func extract4x4TMAbove(above []byte) (byte, [4]byte) {
	var P byte = 128
	var A [4]byte

	if len(above) >= 1 {
		P = above[0]
	}
	if len(above) >= 5 {
		for i := 0; i < 4; i++ {
			A[i] = above[i+1]
		}
	} else {
		for i := range A {
			A[i] = 127
		}
		P = 127
	}
	return P, A
}

// extract4x4TMLeft extracts left pixels for TM prediction.
func extract4x4TMLeft(left []byte, P *byte) [4]byte {
	var L [4]byte
	if len(left) >= 4 {
		copy(L[:], left[:4])
	} else {
		for i := range L {
			L[i] = 129
		}
		*P = 129
	}
	return L
}

// predict4x4VE fills a 4x4 block using vertical prediction with smoothing.
func predict4x4VE(dst, above []byte) {
	A := extractAbove5(above)
	P := extractTopLeft(above)

	row := [4]byte{
		avg3(P, A[0], A[1]),
		avg3(A[0], A[1], A[2]),
		avg3(A[1], A[2], A[3]),
		avg3(A[2], A[3], A[4]),
	}
	fill4x4Rows(dst, row)
}

// extractAbove5 extracts the 5 above pixels (A[0..4]) for VE prediction.
func extractAbove5(above []byte) [5]byte {
	var A [5]byte
	if len(above) >= 6 {
		for i := 0; i < 5; i++ {
			A[i] = above[i+1]
		}
	} else if len(above) >= 5 {
		for i := 0; i < 4; i++ {
			A[i] = above[i+1]
		}
		A[4] = A[3]
	} else {
		for i := range A {
			A[i] = 127
		}
	}
	return A
}

// extractTopLeft extracts the top-left pixel (P) from the above array.
func extractTopLeft(above []byte) byte {
	if len(above) >= 1 {
		return above[0]
	}
	return 127
}

// fill4x4Rows fills all 4 rows of a 4x4 block with the same row values.
func fill4x4Rows(dst []byte, row [4]byte) {
	for r := 0; r < 4; r++ {
		copy(dst[r*4:r*4+4], row[:])
	}
}

// predict4x4HE fills a 4x4 block using horizontal prediction with smoothing.
// Per RFC 6386 §12.3, row 0 uses avg3(P, L[0], L[1]) where P is the top-left pixel.
func predict4x4HE(dst, above, left []byte) {
	var L [4]byte
	var P byte = 129

	// Extract top-left pixel P from above array (above[0] contains P)
	if len(above) >= 1 {
		P = above[0]
	}

	if len(left) >= 4 {
		copy(L[:], left[:4])
	} else {
		for i := range L {
			L[i] = 129
		}
	}

	// Row 0: avg3(P, L[0], L[1])
	// Row 1: avg3(L[0], L[1], L[2])
	// Row 2: avg3(L[1], L[2], L[3])
	// Row 3: avg3(L[2], L[3], L[3]) - L[4] doesn't exist

	cols := [4]byte{
		avg3(P, L[0], L[1]),
		avg3(L[0], L[1], L[2]),
		avg3(L[1], L[2], L[3]),
		avg3(L[2], L[3], L[3]),
	}

	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			dst[r*4+c] = cols[r]
		}
	}
}

// predict4x4LD fills a 4x4 block using left-down diagonal prediction.
func predict4x4LD(dst, above []byte) {
	A := extractAbove8(above)

	dst[0] = avg3(A[0], A[1], A[2])
	dst[1] = avg3(A[1], A[2], A[3])
	dst[4] = dst[1]
	dst[2] = avg3(A[2], A[3], A[4])
	dst[5] = dst[2]
	dst[8] = dst[2]
	dst[3] = avg3(A[3], A[4], A[5])
	dst[6] = dst[3]
	dst[9] = dst[3]
	dst[12] = dst[3]
	dst[7] = avg3(A[4], A[5], A[6])
	dst[10] = dst[7]
	dst[13] = dst[7]
	dst[11] = avg3(A[5], A[6], A[7])
	dst[14] = dst[11]
	dst[15] = avg3(A[6], A[7], A[7])
}

// predict4x4RD fills a 4x4 block using right-down diagonal prediction.
func predict4x4RD(dst, E []byte) {
	// E[0]=L[3], E[1]=L[2], E[2]=L[1], E[3]=L[0], E[4]=P, E[5]=A[0], E[6]=A[1], E[7]=A[2], E[8]=A[3]

	dst[12] = avg3(E[0], E[1], E[2]) // (3,0)
	dst[8] = avg3(E[1], E[2], E[3])  // (2,0)
	dst[13] = dst[8]                 // (3,1)
	dst[4] = avg3(E[2], E[3], E[4])  // (1,0)
	dst[9] = dst[4]                  // (2,1)
	dst[14] = dst[4]                 // (3,2)
	dst[0] = avg3(E[3], E[4], E[5])  // (0,0)
	dst[5] = dst[0]                  // (1,1)
	dst[10] = dst[0]                 // (2,2)
	dst[15] = dst[0]                 // (3,3)
	dst[1] = avg3(E[4], E[5], E[6])  // (0,1)
	dst[6] = dst[1]                  // (1,2)
	dst[11] = dst[1]                 // (2,3)
	dst[2] = avg3(E[5], E[6], E[7])  // (0,2)
	dst[7] = dst[2]                  // (1,3)
	dst[3] = avg3(E[6], E[7], E[8])  // (0,3)
}

// predict4x4VR fills a 4x4 block using vertical-right diagonal prediction.
func predict4x4VR(dst, E []byte) {
	dst[12] = avg3(E[1], E[2], E[3]) // (3,0)
	dst[8] = avg3(E[2], E[3], E[4])  // (2,0)
	dst[4] = avg3(E[3], E[4], E[5])  // (1,0)
	dst[13] = dst[4]                 // (3,1)
	dst[0] = avg2(E[4], E[5])        // (0,0)
	dst[9] = dst[0]                  // (2,1)
	dst[5] = avg3(E[4], E[5], E[6])  // (1,1)
	dst[14] = dst[5]                 // (3,2)
	dst[1] = avg2(E[5], E[6])        // (0,1)
	dst[10] = dst[1]                 // (2,2)
	dst[6] = avg3(E[5], E[6], E[7])  // (1,2)
	dst[15] = dst[6]                 // (3,3)
	dst[2] = avg2(E[6], E[7])        // (0,2)
	dst[11] = dst[2]                 // (2,3)
	dst[7] = avg3(E[6], E[7], E[8])  // (1,3)
	dst[3] = avg2(E[7], E[8])        // (0,3)
}

// predict4x4VL fills a 4x4 block using vertical-left diagonal prediction.
func predict4x4VL(dst, above []byte) {
	A := extractAbove8(above)

	dst[0] = avg2(A[0], A[1])
	dst[4] = avg3(A[0], A[1], A[2])
	dst[1] = avg2(A[1], A[2])
	dst[5] = avg3(A[1], A[2], A[3])
	dst[8] = dst[1]
	dst[2] = avg2(A[2], A[3])
	dst[6] = avg3(A[2], A[3], A[4])
	dst[9] = dst[2]
	dst[12] = dst[5]
	dst[3] = avg2(A[3], A[4])
	dst[7] = avg3(A[3], A[4], A[5])
	dst[10] = dst[3]
	dst[13] = dst[6]
	dst[11] = avg3(A[4], A[5], A[6])
	dst[14] = dst[7]
	dst[15] = avg3(A[5], A[6], A[7])
}

// predict4x4HD fills a 4x4 block using horizontal-down diagonal prediction.
func predict4x4HD(dst, E []byte) {
	// E[0]=L[3], E[1]=L[2], E[2]=L[1], E[3]=L[0], E[4]=P, E[5]=A[0], E[6]=A[1], E[7]=A[2]

	dst[12] = avg2(E[0], E[1])
	dst[13] = avg3(E[0], E[1], E[2])
	dst[8] = avg2(E[1], E[2])
	dst[14] = dst[8]
	dst[9] = avg3(E[1], E[2], E[3])
	dst[15] = dst[9]
	dst[4] = avg2(E[2], E[3])
	dst[10] = dst[4]
	dst[5] = avg3(E[2], E[3], E[4])
	dst[11] = dst[5]
	dst[0] = avg2(E[3], E[4])
	dst[6] = dst[0]
	dst[1] = avg3(E[3], E[4], E[5])
	dst[7] = dst[1]
	dst[2] = avg3(E[4], E[5], E[6])
	dst[3] = avg3(E[5], E[6], E[7])
}

// predict4x4HU fills a 4x4 block using horizontal-up diagonal prediction.
func predict4x4HU(dst, left []byte) {
	var L [4]byte
	if len(left) >= 4 {
		copy(L[:], left[:4])
	} else {
		for i := range L {
			L[i] = 129
		}
	}

	dst[0] = avg2(L[0], L[1])
	dst[1] = avg3(L[0], L[1], L[2])
	dst[2] = avg2(L[1], L[2])
	dst[4] = dst[2]
	dst[3] = avg3(L[1], L[2], L[3])
	dst[5] = dst[3]
	dst[6] = avg2(L[2], L[3])
	dst[8] = dst[6]
	dst[7] = avg3(L[2], L[3], L[3])
	dst[9] = dst[7]
	// Fill rest with L[3]
	dst[10] = L[3]
	dst[11] = L[3]
	dst[12] = L[3]
	dst[13] = L[3]
	dst[14] = L[3]
	dst[15] = L[3]
}

// SelectBest4x4Mode evaluates all 4x4 prediction modes and returns the one
// with the lowest Sum of Absolute Differences (SAD) compared to the source block.
func SelectBest4x4Mode(src, above, left []byte) (intraBMode, int) {
	var pred [16]byte
	bestMode := B_DC_PRED
	bestSAD := 1 << 30

	for mode := B_DC_PRED; mode < numIntraBModes; mode++ {
		Predict4x4(pred[:], above, left, mode)
		sad := computeSAD4x4(src, pred[:])
		if sad < bestSAD {
			bestSAD = sad
			bestMode = mode
		}
	}

	return bestMode, bestSAD
}

// computeSAD4x4 computes Sum of Absolute Differences between two 4x4 blocks.
func computeSAD4x4(a, b []byte) int {
	sad := 0
	for i := 0; i < 16; i++ {
		diff := int(a[i]) - int(b[i])
		if diff < 0 {
			diff = -diff
		}
		sad += diff
	}
	return sad
}
