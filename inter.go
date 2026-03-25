package vp8

// This file implements VP8 inter-frame (P-frame) macroblock processing.
// Inter-frame macroblocks use motion-compensated prediction from a reference
// frame instead of intra prediction from neighboring pixels.
//
// Reference: RFC 6386 §16 – Inter-frame Macroblock Prediction

// processInterMacroblock processes a 16x16 macroblock for inter-frame encoding.
// It performs motion estimation, computes the motion-compensated residual,
// and then applies the same DCT/quantization pipeline as intra macroblocks.
//
// Parameters:
//   - srcY: source luma block (256 bytes, 16x16)
//   - srcU: source U chroma block (64 bytes, 8x8)
//   - srcV: source V chroma block (64 bytes, 8x8)
//   - ref: reference frame buffer
//   - mbX, mbY: macroblock grid position
//   - mbW: macroblock grid width
//   - mbs: macroblock array (for MV prediction from neighbors)
//   - qf: quantization factors
//   - ctx: neighbor context for intra fallback comparison
//
// Returns the processed macroblock with either inter or intra mode selected.
func processInterMacroblock(srcY, srcU, srcV []byte, ref *refFrameBuffer,
	mbX, mbY, mbW int, mbs []macroblock, qf QuantFactors, ctx *mbContext) macroblock {

	mb := macroblock{
		skip:    true,
		isInter: false,
	}

	// Get motion vector prediction from neighbors
	nearestMV, _ := findNearestMV(mbs, mbX, mbY, mbW)

	// Perform motion estimation
	meResult := estimateMotion(srcY, ref.Y, ref.Width, ref.Height,
		mbX*16, mbY*16, nearestMV)

	// Compare with intra prediction cost
	best16x16Mode, intraSAD := SelectBest16x16Mode(srcY, ctx.lumaAbove, ctx.lumaLeft, ctx.lumaTopLeft)

	// Inter mode cost includes MV coding overhead
	interCost := meResult.sad + mvCost(meResult.mv, nearestMV)

	// Choose between inter and intra mode
	// Use a bias toward inter mode since it typically provides better compression
	// for temporal prediction
	if interCost < intraSAD {
		// Inter mode wins
		mb.isInter = true
		mb.refFrame = refFrameLast
		mb.mv = meResult.mv
		mb.interMode = meResult.mode

		// Compute motion-compensated prediction
		var predY [256]byte
		motionCompensate16x16(predY[:], ref.Y, ref.Width, ref.Height, mbX*16, mbY*16, meResult.mv)

		// Process Y blocks with MC prediction
		processInterYBlocks(srcY, predY[:], &mb, qf)

		// Process chroma with halved MV
		chromaW := ref.Width / 2
		chromaH := ref.Height / 2
		chromaMV := motionVector{
			dx: meResult.mv.dx / 2,
			dy: meResult.mv.dy / 2,
		}

		var predU, predV [64]byte
		motionCompensate8x8(predU[:], ref.Cb, chromaW, chromaH, mbX*8, mbY*8, chromaMV)
		motionCompensate8x8(predV[:], ref.Cr, chromaW, chromaH, mbX*8, mbY*8, chromaMV)

		processInterChromaBlocks(srcU, predU[:], &mb, qf, true)
		processInterChromaBlocks(srcV, predV[:], &mb, qf, false)
	} else {
		// Intra mode wins — encode as intra within the inter frame
		mb.isInter = false
		mb.lumaMode = best16x16Mode
		chromaMode, _ := SelectBest8x8ChromaMode(srcU, ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU)
		mb.chromaMode = chromaMode

		// Process using the standard intra pipeline
		processYBlocks16x16(srcY, ctx, &mb, qf)
		processIntraChromaInInterFrame(srcU, srcV, ctx, &mb, qf)
	}

	return mb
}

// processInterYBlocks processes Y blocks using motion-compensated prediction.
// Similar to processYBlocks16x16 but uses the MC prediction instead of intra.
func processInterYBlocks(srcY, predY []byte, mb *macroblock, qf QuantFactors) {
	var dcValues [16]int16

	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx

			// Extract 4x4 source block
			var src4x4, pred4x4 [16]byte
			for row := 0; row < 4; row++ {
				srcRow := (by*4 + row) * 16
				copy(src4x4[row*4:row*4+4], srcY[srcRow+bx*4:srcRow+bx*4+4])
				copy(pred4x4[row*4:row*4+4], predY[srcRow+bx*4:srcRow+bx*4+4])
			}

			residual := ComputeResidual(src4x4[:], pred4x4[:])
			dctCoeffs := ForwardDCT4x4(residual[:])
			dcValues[blockIdx] = dctCoeffs[0]
			quantized := QuantizeBlock(dctCoeffs, qf.Y1DC, qf.Y1AC)
			mb.yCoeffs[blockIdx] = ToZigzag(quantized)

			for i := 1; i < 16; i++ {
				if mb.yCoeffs[blockIdx][i] != 0 {
					mb.skip = false
				}
			}
		}
	}

	// WHT of DC values
	whtCoeffs := ForwardWHT4x4(dcValues)
	quantizedWHT := QuantizeBlock(whtCoeffs, qf.Y2DC, qf.Y2AC)
	mb.y2Coeffs = ToZigzag(quantizedWHT)

	// Clear DC from Y blocks (encoded via Y2)
	for i := 0; i < 16; i++ {
		mb.yCoeffs[i][0] = 0
	}

	for i := 0; i < 16; i++ {
		if mb.y2Coeffs[i] != 0 {
			mb.skip = false
		}
	}
}

// processInterChromaBlocks processes chroma blocks with MC prediction.
func processInterChromaBlocks(src, pred []byte, mb *macroblock, qf QuantFactors, isU bool) {
	for by := 0; by < 2; by++ {
		for bx := 0; bx < 2; bx++ {
			blockIdx := by*2 + bx

			var src4x4, pred4x4 [16]byte
			for row := 0; row < 4; row++ {
				srcRow := (by*4 + row) * 8
				copy(src4x4[row*4:row*4+4], src[srcRow+bx*4:srcRow+bx*4+4])
				copy(pred4x4[row*4:row*4+4], pred[srcRow+bx*4:srcRow+bx*4+4])
			}

			residual := ComputeResidual(src4x4[:], pred4x4[:])
			dctCoeffs := ForwardDCT4x4(residual[:])
			quantized := QuantizeBlock(dctCoeffs, qf.UVDC, qf.UVAC)

			if isU {
				mb.uCoeffs[blockIdx] = ToZigzag(quantized)
				for i := 0; i < 16; i++ {
					if mb.uCoeffs[blockIdx][i] != 0 {
						mb.skip = false
					}
				}
			} else {
				mb.vCoeffs[blockIdx] = ToZigzag(quantized)
				for i := 0; i < 16; i++ {
					if mb.vCoeffs[blockIdx][i] != 0 {
						mb.skip = false
					}
				}
			}
		}
	}
}

// processIntraChromaInInterFrame processes chroma blocks using intra prediction
// within an inter frame. This is used when a macroblock falls back to intra mode.
func processIntraChromaInInterFrame(srcU, srcV []byte, ctx *mbContext, mb *macroblock, qf QuantFactors) {
	// U plane
	var predU [64]byte
	Predict8x8Chroma(predU[:], ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU, mb.chromaMode)
	for by := 0; by < 2; by++ {
		for bx := 0; bx < 2; bx++ {
			blockIdx := by*2 + bx
			var src4x4, pred4x4 [16]byte
			for row := 0; row < 4; row++ {
				srcRow := (by*4 + row) * 8
				copy(src4x4[row*4:row*4+4], srcU[srcRow+bx*4:srcRow+bx*4+4])
				copy(pred4x4[row*4:row*4+4], predU[srcRow+bx*4:srcRow+bx*4+4])
			}
			residual := ComputeResidual(src4x4[:], pred4x4[:])
			dctCoeffs := ForwardDCT4x4(residual[:])
			quantized := QuantizeBlock(dctCoeffs, qf.UVDC, qf.UVAC)
			mb.uCoeffs[blockIdx] = ToZigzag(quantized)
			for i := 0; i < 16; i++ {
				if mb.uCoeffs[blockIdx][i] != 0 {
					mb.skip = false
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
			var src4x4, pred4x4 [16]byte
			for row := 0; row < 4; row++ {
				srcRow := (by*4 + row) * 8
				copy(src4x4[row*4:row*4+4], srcV[srcRow+bx*4:srcRow+bx*4+4])
				copy(pred4x4[row*4:row*4+4], predV[srcRow+bx*4:srcRow+bx*4+4])
			}
			residual := ComputeResidual(src4x4[:], pred4x4[:])
			dctCoeffs := ForwardDCT4x4(residual[:])
			quantized := QuantizeBlock(dctCoeffs, qf.UVDC, qf.UVAC)
			mb.vCoeffs[blockIdx] = ToZigzag(quantized)
			for i := 0; i < 16; i++ {
				if mb.vCoeffs[blockIdx][i] != 0 {
					mb.skip = false
				}
			}
		}
	}
}
