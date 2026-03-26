package vp8

// This file implements VP8 reference frame buffer management.
// VP8 maintains three reference frame buffers:
//   - Last: the most recently encoded frame
//   - Golden: a selected reference frame for longer-term prediction
//   - AltRef: an alternate reference frame
//
// Reference: RFC 6386 §9.8 – Reference Frame Buffer Management

// refFrameType identifies which reference frame buffer to use for prediction.
type refFrameType uint8

const (
	// refFrameCurrent is a sentinel used during encoding (intra prediction).
	refFrameCurrent refFrameType = iota
	// refFrameLast is the most recently encoded frame.
	refFrameLast
	// refFrameGolden is the golden reference frame.
	refFrameGolden
	// refFrameAltRef is the alternate reference frame.
	refFrameAltRef
)

// refFrameBuffer holds a single reconstructed reference frame in YUV420 format.
type refFrameBuffer struct {
	// Y is the reconstructed luma plane.
	Y []byte
	// Cb is the reconstructed Cb chroma plane.
	Cb []byte
	// Cr is the reconstructed Cr chroma plane.
	Cr []byte
	// Width and Height are the frame dimensions.
	Width, Height int
	// valid indicates whether this buffer has been initialized.
	valid bool
}

// refFrameManager manages the three VP8 reference frame buffers.
// After each frame is encoded and reconstructed, the buffers are updated
// according to VP8 refresh flags.
type refFrameManager struct {
	last   refFrameBuffer
	golden refFrameBuffer
	altRef refFrameBuffer
	width  int
	height int
}

// newRefFrameManager creates a new reference frame manager for the given
// frame dimensions.
func newRefFrameManager(width, height int) *refFrameManager {
	return &refFrameManager{
		width:  width,
		height: height,
	}
}

// allocBuffer allocates a reference frame buffer for the configured dimensions.
func (m *refFrameManager) allocBuffer() refFrameBuffer {
	ySize := m.width * m.height
	uvSize := (m.width / 2) * (m.height / 2)
	return refFrameBuffer{
		Y:      make([]byte, ySize),
		Cb:     make([]byte, uvSize),
		Cr:     make([]byte, uvSize),
		Width:  m.width,
		Height: m.height,
		valid:  false,
	}
}

// getRef returns the reference frame buffer for the given type.
// Returns nil if the requested buffer is not valid.
func (m *refFrameManager) getRef(ref refFrameType) *refFrameBuffer {
	switch ref {
	case refFrameLast:
		if m.last.valid {
			return &m.last
		}
	case refFrameGolden:
		if m.golden.valid {
			return &m.golden
		}
	case refFrameAltRef:
		if m.altRef.valid {
			return &m.altRef
		}
	}
	return nil
}

// hasReference returns true if the specified reference frame is available.
func (m *refFrameManager) hasReference(ref refFrameType) bool {
	return m.getRef(ref) != nil
}

// updateLast updates the last reference frame buffer with the given
// reconstructed frame data. This is called after every key frame and
// after inter frames with refresh_last set.
func (m *refFrameManager) updateLast(y, cb, cr []byte) {
	if !m.last.valid {
		m.last = m.allocBuffer()
		m.last.valid = true
	}
	copy(m.last.Y, y)
	copy(m.last.Cb, cb)
	copy(m.last.Cr, cr)
}

// updateGolden updates the golden reference frame buffer.
func (m *refFrameManager) updateGolden(y, cb, cr []byte) {
	if !m.golden.valid {
		m.golden = m.allocBuffer()
		m.golden.valid = true
	}
	copy(m.golden.Y, y)
	copy(m.golden.Cb, cb)
	copy(m.golden.Cr, cr)
}

// updateAltRef updates the alternate reference frame buffer.
func (m *refFrameManager) updateAltRef(y, cb, cr []byte) {
	if !m.altRef.valid {
		m.altRef = m.allocBuffer()
		m.altRef.valid = true
	}
	copy(m.altRef.Y, y)
	copy(m.altRef.Cb, cb)
	copy(m.altRef.Cr, cr)
}

// copyLastToGolden copies the last frame buffer to the golden buffer.
// This is used when refresh_golden_frame is set with copy_buffer_to_gf=1.
func (m *refFrameManager) copyLastToGolden() {
	if !m.last.valid {
		return
	}
	if !m.golden.valid {
		m.golden = m.allocBuffer()
		m.golden.valid = true
	}
	copy(m.golden.Y, m.last.Y)
	copy(m.golden.Cb, m.last.Cb)
	copy(m.golden.Cr, m.last.Cr)
}

// copyLastToAltRef copies the last frame buffer to the alternate reference.
func (m *refFrameManager) copyLastToAltRef() {
	if !m.last.valid {
		return
	}
	if !m.altRef.valid {
		m.altRef = m.allocBuffer()
		m.altRef.valid = true
	}
	copy(m.altRef.Y, m.last.Y)
	copy(m.altRef.Cb, m.last.Cb)
	copy(m.altRef.Cr, m.last.Cr)
}

// reset invalidates all reference frame buffers.
// This is called when dimensions change or on encoder reset.
func (m *refFrameManager) reset() {
	m.last.valid = false
	m.golden.valid = false
	m.altRef.valid = false
}

// reconstructFrame reconstructs a full frame from encoded macroblocks and stores
// it in the provided output buffers. This is needed to build reference frames for
// inter-frame prediction.
//
// For key frames: reconstructs from intra predictions.
// For inter frames: reconstructs from motion-compensated predictions.
func reconstructFrame(recon *refFrameBuffer, mbs []macroblock, qf QuantFactors,
	ref *refFrameManager, frame *Frame,
) {
	width := recon.Width
	height := recon.Height
	mbW := (width + 15) / 16
	mbH := (height + 15) / 16
	chromaW := width / 2

	for mbY := 0; mbY < mbH; mbY++ {
		for mbX := 0; mbX < mbW; mbX++ {
			mbIdx := mbY*mbW + mbX
			mb := &mbs[mbIdx]

			if mb.isInter {
				// Inter-frame reconstruction: motion-compensated prediction + residual
				reconstructInterMB(recon, mb, mbX, mbY, width, height, chromaW, qf, ref)
			} else {
				// Intra-frame reconstruction: intra prediction + residual
				reconstructIntraMB(recon, mb, mbX, mbY, width, height, chromaW, qf)
			}
		}
	}
}

// reconstructIntraMB reconstructs a single intra-predicted macroblock.
func reconstructIntraMB(recon *refFrameBuffer, mb *macroblock, mbX, mbY, width, height, chromaW int, qf QuantFactors) {
	// Build neighbor context from reconstructed frame
	ctx := buildReconContext(recon, mbX, mbY, width, height, chromaW)

	// Reconstruct luma
	if mb.lumaMode == B_PRED {
		reconstructLumaBPred(recon, mb, ctx, mbX, mbY, width, qf)
	} else {
		reconstructLuma16x16(recon, mb, ctx, mbX, mbY, width, qf)
	}

	// Reconstruct chroma
	reconstructChroma(recon, mb, ctx, mbX, mbY, width, chromaW, qf)
}

// buildReconContext builds neighbor context from the reconstructed frame buffer.
// Uses fixed-size backing arrays in mbContext to avoid per-MB heap allocations.
func buildReconContext(recon *refFrameBuffer, mbX, mbY, width, height, chromaW int) *mbContext {
	ctx := &mbContext{}
	chromaH := height / 2

	if mbY > 0 {
		aboveRow := mbY*16 - 1
		for i := 0; i < 16; i++ {
			col := mbX*16 + i
			if col < width {
				ctx.lumaAboveBuf[i] = recon.Y[aboveRow*width+col]
			}
		}
		ctx.lumaAbove = ctx.lumaAboveBuf[:]
	}

	if mbX > 0 {
		leftCol := mbX*16 - 1
		for i := 0; i < 16; i++ {
			row := mbY*16 + i
			if row < height {
				ctx.lumaLeftBuf[i] = recon.Y[row*width+leftCol]
			}
		}
		ctx.lumaLeft = ctx.lumaLeftBuf[:]
	}

	if mbX > 0 && mbY > 0 {
		ctx.lumaTopLeft = recon.Y[(mbY*16-1)*width+(mbX*16-1)]
	} else {
		ctx.lumaTopLeft = 128
	}

	if mbY > 0 {
		aboveRow := mbY*8 - 1
		for i := 0; i < 8; i++ {
			col := mbX*8 + i
			if col < chromaW {
				ctx.chromaAboveUBuf[i] = recon.Cb[aboveRow*chromaW+col]
				ctx.chromaAboveVBuf[i] = recon.Cr[aboveRow*chromaW+col]
			}
		}
		ctx.chromaAboveU = ctx.chromaAboveUBuf[:]
		ctx.chromaAboveV = ctx.chromaAboveVBuf[:]
	}

	if mbX > 0 {
		leftCol := mbX*8 - 1
		for i := 0; i < 8; i++ {
			row := mbY*8 + i
			if row < chromaH {
				ctx.chromaLeftUBuf[i] = recon.Cb[row*chromaW+leftCol]
				ctx.chromaLeftVBuf[i] = recon.Cr[row*chromaW+leftCol]
			}
		}
		ctx.chromaLeftU = ctx.chromaLeftUBuf[:]
		ctx.chromaLeftV = ctx.chromaLeftVBuf[:]
	}

	if mbX > 0 && mbY > 0 {
		ctx.chromaTopLeftU = recon.Cb[(mbY*8-1)*chromaW+(mbX*8-1)]
		ctx.chromaTopLeftV = recon.Cr[(mbY*8-1)*chromaW+(mbX*8-1)]
	} else {
		ctx.chromaTopLeftU = 128
		ctx.chromaTopLeftV = 128
	}

	return ctx
}

// reconstructLuma16x16 reconstructs luma using 16x16 prediction mode.
func reconstructLuma16x16(recon *refFrameBuffer, mb *macroblock, ctx *mbContext, mbX, mbY, width int, qf QuantFactors) {
	// Generate 16x16 prediction
	var predY [256]byte
	Predict16x16(predY[:], ctx.lumaAbove, ctx.lumaLeft, ctx.lumaTopLeft, mb.lumaMode)

	// Dequantize Y2 (WHT DC block)
	dequantWHT := DequantizeBlock(FromZigzag(mb.y2Coeffs), qf.Y2DC, qf.Y2AC)
	dcValues := InverseWHT4x4(dequantWHT)

	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx

			// Dequantize AC coefficients
			zigzagCoeffs := mb.yCoeffs[blockIdx]
			rasterCoeffs := FromZigzag(zigzagCoeffs)
			// Replace DC with value from WHT
			rasterCoeffs[0] = dcValues[blockIdx]
			dequantized := DequantizeBlock(rasterCoeffs, qf.Y1DC, qf.Y1AC)
			// But DC was already dequantized through WHT, so use it directly
			dequantized[0] = dcValues[blockIdx]

			invDCT := InverseDCT4x4(dequantized)

			// Add prediction to residual and store in recon
			for row := 0; row < 4; row++ {
				for col := 0; col < 4; col++ {
					py := mbY*16 + by*4 + row
					px := mbX*16 + bx*4 + col
					if py < recon.Height && px < width {
						predIdx := (by*4+row)*16 + bx*4 + col
						val := int(predY[predIdx]) + int(invDCT[row*4+col])
						recon.Y[py*width+px] = clamp8(val)
					}
				}
			}
		}
	}
}

// reconstructLumaBPred reconstructs luma using B_PRED mode.
func reconstructLumaBPred(recon *refFrameBuffer, mb *macroblock, ctx *mbContext, mbX, mbY, width int, qf QuantFactors) {
	// Local reconstruction buffer for inter-block dependencies
	var localRecon [256]byte

	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx

			above, left := build4x4Context(by, bx, ctx, localRecon[:])

			var pred4x4 [16]byte
			Predict4x4(pred4x4[:], above, left, mb.bModes[blockIdx])

			// Dequantize
			zigzagCoeffs := mb.yCoeffs[blockIdx]
			rasterCoeffs := FromZigzag(zigzagCoeffs)
			dequantized := DequantizeBlock(rasterCoeffs, qf.Y1DC, qf.Y1AC)
			invDCT := InverseDCT4x4(dequantized)

			for row := 0; row < 4; row++ {
				for col := 0; col < 4; col++ {
					val := int(pred4x4[row*4+col]) + int(invDCT[row*4+col])
					clamped := clamp8(val)
					localRecon[(by*4+row)*16+bx*4+col] = clamped

					py := mbY*16 + by*4 + row
					px := mbX*16 + bx*4 + col
					if py < recon.Height && px < width {
						recon.Y[py*width+px] = clamped
					}
				}
			}
		}
	}
}

// reconstructChroma reconstructs the chroma planes for a macroblock.
func reconstructChroma(recon *refFrameBuffer, mb *macroblock, ctx *mbContext, mbX, mbY, width, chromaW int, qf QuantFactors) {
	chromaH := recon.Height / 2

	// U plane
	var predU [64]byte
	Predict8x8Chroma(predU[:], ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU, mb.chromaMode)

	for by := 0; by < 2; by++ {
		for bx := 0; bx < 2; bx++ {
			blockIdx := by*2 + bx
			rasterCoeffs := FromZigzag(mb.uCoeffs[blockIdx])
			dequantized := DequantizeBlock(rasterCoeffs, qf.UVDC, qf.UVAC)
			invDCT := InverseDCT4x4(dequantized)

			for row := 0; row < 4; row++ {
				for col := 0; col < 4; col++ {
					py := mbY*8 + by*4 + row
					px := mbX*8 + bx*4 + col
					if py < chromaH && px < chromaW {
						predIdx := (by*4+row)*8 + bx*4 + col
						val := int(predU[predIdx]) + int(invDCT[row*4+col])
						recon.Cb[py*chromaW+px] = clamp8(val)
					}
				}
			}
		}
	}

	// V plane
	var predV [64]byte
	Predict8x8Chroma(predV[:], ctx.chromaAboveV, ctx.chromaLeftV, ctx.chromaTopLeftV, mb.chromaMode)

	for by := 0; by < 2; by++ {
		for bx := 0; bx < 2; bx++ {
			blockIdx := by*2 + bx
			rasterCoeffs := FromZigzag(mb.vCoeffs[blockIdx])
			dequantized := DequantizeBlock(rasterCoeffs, qf.UVDC, qf.UVAC)
			invDCT := InverseDCT4x4(dequantized)

			for row := 0; row < 4; row++ {
				for col := 0; col < 4; col++ {
					py := mbY*8 + by*4 + row
					px := mbX*8 + bx*4 + col
					if py < chromaH && px < chromaW {
						predIdx := (by*4+row)*8 + bx*4 + col
						val := int(predV[predIdx]) + int(invDCT[row*4+col])
						recon.Cr[py*chromaW+px] = clamp8(val)
					}
				}
			}
		}
	}
}

// reconstructInterMB reconstructs a single inter-predicted macroblock using
// motion compensation from the reference frame.
func reconstructInterMB(recon *refFrameBuffer, mb *macroblock, mbX, mbY, width, height, chromaW int, qf QuantFactors, ref *refFrameManager) {
	refBuf := ref.getRef(mb.refFrame)
	if refBuf == nil {
		// Fallback to intra if reference is not available
		reconstructIntraMB(recon, mb, mbX, mbY, width, height, chromaW, qf)
		return
	}

	// Motion-compensated prediction for luma
	var predY [256]byte
	motionCompensate16x16(predY[:], refBuf.Y, width, height, mbX*16, mbY*16, mb.mv)

	// Dequantize and add residual for Y blocks (same as intra 16x16 but with MC prediction)
	// For inter frames, DC values go through Y2 transform just like 16x16 intra modes
	dequantWHT := DequantizeBlock(FromZigzag(mb.y2Coeffs), qf.Y2DC, qf.Y2AC)
	dcValues := InverseWHT4x4(dequantWHT)

	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx

			zigzagCoeffs := mb.yCoeffs[blockIdx]
			rasterCoeffs := FromZigzag(zigzagCoeffs)
			rasterCoeffs[0] = dcValues[blockIdx]
			dequantized := DequantizeBlock(rasterCoeffs, qf.Y1DC, qf.Y1AC)
			dequantized[0] = dcValues[blockIdx]

			invDCT := InverseDCT4x4(dequantized)

			for row := 0; row < 4; row++ {
				for col := 0; col < 4; col++ {
					py := mbY*16 + by*4 + row
					px := mbX*16 + bx*4 + col
					if py < height && px < width {
						predIdx := (by*4+row)*16 + bx*4 + col
						val := int(predY[predIdx]) + int(invDCT[row*4+col])
						recon.Y[py*width+px] = clamp8(val)
					}
				}
			}
		}
	}

	// Motion-compensated prediction for chroma (MV is halved for chroma)
	chromaH := height / 2
	chromaMV := motionVector{
		dx: mb.mv.dx / 2,
		dy: mb.mv.dy / 2,
	}

	var predU, predV [64]byte
	motionCompensate8x8(predU[:], refBuf.Cb, chromaW, chromaH, mbX*8, mbY*8, chromaMV)
	motionCompensate8x8(predV[:], refBuf.Cr, chromaW, chromaH, mbX*8, mbY*8, chromaMV)

	// Reconstruct U and V
	for by := 0; by < 2; by++ {
		for bx := 0; bx < 2; bx++ {
			blockIdx := by*2 + bx

			// U
			rasterU := FromZigzag(mb.uCoeffs[blockIdx])
			dequantU := DequantizeBlock(rasterU, qf.UVDC, qf.UVAC)
			invU := InverseDCT4x4(dequantU)
			for row := 0; row < 4; row++ {
				for col := 0; col < 4; col++ {
					py := mbY*8 + by*4 + row
					px := mbX*8 + bx*4 + col
					if py < chromaH && px < chromaW {
						predIdx := (by*4+row)*8 + bx*4 + col
						val := int(predU[predIdx]) + int(invU[row*4+col])
						recon.Cb[py*chromaW+px] = clamp8(val)
					}
				}
			}

			// V
			rasterV := FromZigzag(mb.vCoeffs[blockIdx])
			dequantV := DequantizeBlock(rasterV, qf.UVDC, qf.UVAC)
			invV := InverseDCT4x4(dequantV)
			for row := 0; row < 4; row++ {
				for col := 0; col < 4; col++ {
					py := mbY*8 + by*4 + row
					px := mbX*8 + bx*4 + col
					if py < chromaH && px < chromaW {
						predIdx := (by*4+row)*8 + bx*4 + col
						val := int(predV[predIdx]) + int(invV[row*4+col])
						recon.Cr[py*chromaW+px] = clamp8(val)
					}
				}
			}
		}
	}
}
