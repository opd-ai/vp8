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

// simpleFilterVerticalEdge applies the simple filter to a vertical edge
// at position (x, y). The filter adjusts pixels at x-1 and x.
// Reference: RFC 6386 §15.2
func simpleFilterVerticalEdge(plane []byte, stride, x, y, limit int) {
	if x <= 0 || x >= stride {
		return
	}

	idx := y*stride + x
	p0 := int(plane[idx-1]) // pixel to the left of the edge
	q0 := int(plane[idx])   // pixel to the right of the edge

	// Compute the difference
	diff := p0 - q0
	if diff < 0 {
		diff = -diff
	}

	// Only filter if the difference is within the limit
	if diff > limit {
		return
	}

	// Simple filter: average the two boundary pixels
	// filter_value = (p0 - q0 + 4) >> 3, clamped to [-limit, limit]
	filterVal := (p0 - q0 + 4) >> 3
	if filterVal > limit {
		filterVal = limit
	} else if filterVal < -limit {
		filterVal = -limit
	}

	plane[idx-1] = clamp8(p0 - filterVal)
	plane[idx] = clamp8(q0 + filterVal)
}

// simpleFilterHorizontalEdge applies the simple filter to a horizontal edge
// at position (x, y). The filter adjusts pixels at y-1 and y.
// Reference: RFC 6386 §15.2
func simpleFilterHorizontalEdge(plane []byte, stride, x, y, limit int) {
	if y <= 0 {
		return
	}

	p0Idx := (y-1)*stride + x
	q0Idx := y*stride + x

	p0 := int(plane[p0Idx])
	q0 := int(plane[q0Idx])

	diff := p0 - q0
	if diff < 0 {
		diff = -diff
	}

	if diff > limit {
		return
	}

	filterVal := (p0 - q0 + 4) >> 3
	if filterVal > limit {
		filterVal = limit
	} else if filterVal < -limit {
		filterVal = -limit
	}

	plane[p0Idx] = clamp8(p0 - filterVal)
	plane[q0Idx] = clamp8(q0 + filterVal)
}
