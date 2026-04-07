package vp8

// This file implements a simple VP8 loop filter.
// The loop filter is applied to reconstructed frames before they are stored
// as reference frames for inter-frame prediction. It reduces blocking
// artifacts at macroblock boundaries.
//
// VP8 supports two loop filter types: normal and simple.
// This implementation provides the simple filter per RFC 6386 §15.
//
// Reference: RFC 6386 §15 – Loop Filter

// loopFilterParams holds configuration for the loop filter.
type loopFilterParams struct {
	// level is the loop filter strength (0–63). 0 disables the filter.
	level int
	// sharpness limits filter strength at block boundaries (0–7).
	sharpness int
}

// applyLoopFilter applies the VP8 simple loop filter to a reconstructed frame.
// The filter smooths block boundary pixels to reduce visual blocking artifacts.
// It operates on 4x4 block edges (both macroblock and sub-block boundaries).
//
// The filter strength is determined by the level parameter (0–63).
// Level 0 means no filtering (pass-through).
func applyLoopFilter(recon *refFrameBuffer, params loopFilterParams) {
	if params.level == 0 {
		return
	}

	width := recon.Width
	height := recon.Height
	chromaW := width / 2
	chromaH := height / 2

	// Compute filter limit thresholds from level and sharpness
	limit := computeFilterLimit(params.level, params.sharpness)

	// Filter luma (Y) plane
	filterPlane(recon.Y, width, height, limit, 4)

	// Filter chroma (Cb, Cr) planes
	filterPlane(recon.Cb, chromaW, chromaH, limit, 4)
	filterPlane(recon.Cr, chromaW, chromaH, limit, 4)
}

// computeFilterLimit computes the interior and edge limit values from the
// loop filter level and sharpness parameters.
// Reference: RFC 6386 §15.4
func computeFilterLimit(level, sharpness int) int {
	limit := level
	if sharpness > 0 {
		limit >>= (sharpness + 3) >> 2
		if limit < 1 {
			limit = 1
		}
	}
	if limit > 63 {
		limit = 63
	}
	return limit
}

// filterPlane applies the simple loop filter to a single plane.
// It filters vertical and horizontal edges at the specified step interval.
func filterPlane(plane []byte, width, height, limit, step int) {
	// Filter vertical edges (between columns)
	for y := 0; y < height; y++ {
		for x := step; x < width; x += step {
			simpleFilterVerticalEdge(plane, width, x, y, limit)
		}
	}

	// Filter horizontal edges (between rows)
	for y := step; y < height; y += step {
		for x := 0; x < width; x++ {
			simpleFilterHorizontalEdge(plane, width, x, y, limit)
		}
	}
}

// clampS8 clamps a value to signed 8-bit range [-128, 127].
func clampS8(v int) int {
	if v < -128 {
		return -128
	}
	if v > 127 {
		return 127
	}
	return v
}

// computeSimpleFilterFull calculates the 4-tap simple filter adjustment per RFC 6386 §15.2.
// Returns (adjustment_a, adjustment_b, apply) where:
//   - adjustment_a is subtracted from q0
//   - adjustment_b is added to p0
//   - apply indicates whether filtering should be applied
//
// The simple filter uses 4 pixels: p1, p0, q0, q1
// a = clamp(p1 - q1 + 3*(q0 - p0))
// The final adjustments are a/8 with different rounding for p0 and q0.
func computeSimpleFilterFull(p1, p0, q0, q1, limit int) (a, b int, apply bool) {
	// Check edge limit: |p0 - q0| must be <= limit
	diff := p0 - q0
	if diff < 0 {
		diff = -diff
	}
	if diff > limit {
		return 0, 0, false
	}

	// Convert to signed 8-bit representation (subtract 128)
	sp1 := p1 - 128
	sp0 := p0 - 128
	sq0 := q0 - 128
	sq1 := q1 - 128

	// Simple filter uses outer taps: a = clamp(p1 - q1 + 3*(q0 - p0))
	filter := clampS8(clampS8(sp1-sq1) + 3*(sq0-sp0))

	// b is used for p0, rounds differently for balance
	b = clampS8(filter+3) >> 3

	// a is used for q0, rounds up when fraction >= 1/2
	a = clampS8(filter+4) >> 3

	return a, b, true
}

// simpleFilterVerticalEdge applies the simple filter to a vertical edge
// at position (x, y). The filter adjusts pixels at x-1 and x.
// Reference: RFC 6386 §15.2
func simpleFilterVerticalEdge(plane []byte, stride, x, y, limit int) {
	if x <= 1 || x >= stride {
		return
	}

	idx := y*stride + x
	p1 := int(plane[idx-2])
	p0 := int(plane[idx-1])
	q0 := int(plane[idx])
	q1 := int(plane[idx+1]) // Safe: we're not at right edge (checked x < stride)

	if a, b, apply := computeSimpleFilterFull(p1, p0, q0, q1, limit); apply {
		plane[idx-1] = clamp8(p0 + b)
		plane[idx] = clamp8(q0 - a)
	}
}

// simpleFilterHorizontalEdge applies the simple filter to a horizontal edge
// at position (x, y). The filter adjusts pixels at y-1 and y.
// Reference: RFC 6386 §15.2
func simpleFilterHorizontalEdge(plane []byte, stride, x, y, limit int) {
	if y <= 1 {
		return
	}

	p1Idx := (y-2)*stride + x
	p0Idx := (y-1)*stride + x
	q0Idx := y*stride + x
	q1Idx := (y+1)*stride + x

	p1 := int(plane[p1Idx])
	p0 := int(plane[p0Idx])
	q0 := int(plane[q0Idx])
	q1 := int(plane[q1Idx])

	if a, b, apply := computeSimpleFilterFull(p1, p0, q0, q1, limit); apply {
		plane[p0Idx] = clamp8(p0 + b)
		plane[q0Idx] = clamp8(q0 - a)
	}
}
