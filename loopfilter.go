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
// Per RFC 6386 §15.2, the simple filter only applies to luma edges.
// Chroma edges are left unfiltered.
//
// Per RFC 6386 §15.4, macroblock edges (every 16 pixels for luma) use a
// stronger filter threshold than sub-block edges (every 4 pixels).
//
// The filter strength is determined by the level parameter (0–63).
// Level 0 means no filtering (pass-through).
func applyLoopFilter(recon *refFrameBuffer, params loopFilterParams) {
	if params.level == 0 {
		return
	}

	width := recon.Width
	height := recon.Height

	// Compute interior limit from level and sharpness per RFC 6386 §15.4
	interiorLimit := computeInteriorLimit(params.level, params.sharpness)

	// Compute edge limits per RFC 6386 §15.4
	// Macroblock edge limit is stronger (higher threshold)
	mbEdgeLimit := ((params.level + 2) * 2) + interiorLimit
	// Sub-block edge limit is weaker
	subEdgeLimit := (params.level * 2) + interiorLimit

	// Clamp limits to valid range
	if mbEdgeLimit > 255 {
		mbEdgeLimit = 255
	}
	if subEdgeLimit > 255 {
		subEdgeLimit = 255
	}

	// Filter luma (Y) plane only - per RFC 6386 §15.2, simple filter
	// does not filter chroma edges
	filterPlaneLuma(recon.Y, width, height, mbEdgeLimit, subEdgeLimit)
}

// computeInteriorLimit computes the interior limit value from the
// loop filter level and sharpness parameters.
// Reference: RFC 6386 §15.4
func computeInteriorLimit(level, sharpness int) int {
	interiorLimit := level
	if sharpness > 0 {
		if sharpness > 4 {
			interiorLimit >>= 2
		} else {
			interiorLimit >>= 1
		}
		maxLimit := 9 - sharpness
		if interiorLimit > maxLimit {
			interiorLimit = maxLimit
		}
	}
	if interiorLimit < 1 {
		interiorLimit = 1
	}
	return interiorLimit
}

// computeFilterLimit computes the filter limit value from level and sharpness.
// This is a simplified version that returns the sub-block edge limit.
// Reference: RFC 6386 §15.4
func computeFilterLimit(level, sharpness int) int {
	interiorLimit := computeInteriorLimit(level, sharpness)
	return (level * 2) + interiorLimit
}

// filterPlaneLuma applies the simple loop filter to the luma plane,
// differentiating between macroblock edges (every 16 pixels) and
// sub-block edges (every 4 pixels).
// Reference: RFC 6386 §15.4
func filterPlaneLuma(plane []byte, width, height, mbEdgeLimit, subEdgeLimit int) {
	// Filter vertical edges (between columns)
	for y := 0; y < height; y++ {
		for x := 4; x < width; x += 4 {
			// Use stronger limit at macroblock boundaries (every 16 pixels)
			limit := subEdgeLimit
			if x%16 == 0 {
				limit = mbEdgeLimit
			}
			simpleFilterVerticalEdge(plane, width, x, y, limit)
		}
	}

	// Filter horizontal edges (between rows)
	for y := 4; y < height; y += 4 {
		// Use stronger limit at macroblock boundaries (every 16 pixels)
		limit := subEdgeLimit
		if y%16 == 0 {
			limit = mbEdgeLimit
		}
		for x := 0; x < width; x++ {
			simpleFilterHorizontalEdge(plane, width, x, y, limit)
		}
	}
}

// filterPlane applies the simple loop filter to a single plane.
// It filters vertical and horizontal edges at the specified step interval.
// This is kept for backward compatibility but filterPlaneLuma should be
// used for luma filtering with proper MB/sub-block edge differentiation.
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
// Per RFC 6386 §15.2: a = clamp(p1 - q1 + 3*(q0 - p0))
// The filtering condition is: (|p0 - q0| * 2 + |p1 - q1| / 2) <= edge_limit
func computeSimpleFilterFull(p1, p0, q0, q1, limit int) (a, b int, apply bool) {
	// Per RFC 6386 §15.2: Check edge limit using the formula
	// (abs(P0 - Q0) * 2 + abs(P1 - Q1) / 2) <= edge_limit
	diffP0Q0 := p0 - q0
	if diffP0Q0 < 0 {
		diffP0Q0 = -diffP0Q0
	}
	diffP1Q1 := p1 - q1
	if diffP1Q1 < 0 {
		diffP1Q1 = -diffP1Q1
	}
	if diffP0Q0*2+diffP1Q1/2 > limit {
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
