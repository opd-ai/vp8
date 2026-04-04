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
	mbX, mbY, mbW int, mbs []macroblock, qf QuantFactors, ctx *mbContext,
) macroblock {
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

	// Choose between inter and intra mode based on the estimated costs.
	// Inter mode cost already includes motion vector coding overhead.
	if interCost < intraSAD {
		// Inter mode wins
		mb.isInter = true
		mb.refFrame = refFrameLast
		mb.mv = meResult.mv
		mb.predMV = nearestMV // Store predicted MV for delta-coding in bitstream
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
	dcValues := processInterLumaBlocksWithDC(srcY, predY, mb, qf)
	applyWHTAndFinalize(dcValues, mb, qf)
}

// processInterLumaBlocksWithDC processes 16 inter luma 4x4 blocks and returns DC values.
func processInterLumaBlocksWithDC(srcY, predY []byte, mb *macroblock, qf QuantFactors) [16]int16 {
	var dcValues [16]int16
	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx
			src4x4 := extract4x4Block(srcY, by, bx)
			pred4x4 := extract4x4Block(predY, by, bx)
			dcValues[blockIdx] = processInterLuma4x4Block(src4x4, pred4x4, mb, blockIdx, qf)
		}
	}
	return dcValues
}

// processInterLuma4x4Block processes a single 4x4 inter luma block, returning its DC value.
func processInterLuma4x4Block(src4x4, pred4x4 [16]byte, mb *macroblock, blockIdx int, qf QuantFactors) int16 {
	residual := ComputeResidual(src4x4[:], pred4x4[:])
	dctCoeffs := ForwardDCT4x4(residual[:])
	quantized := QuantizeBlock(dctCoeffs, qf.Y1DC, qf.Y1AC)
	mb.yCoeffs[blockIdx] = ToZigzag(quantized)

	if hasNonZeroACCoeffs(mb.yCoeffs[blockIdx]) {
		mb.skip = false
	}
	return dctCoeffs[0]
}

// processInterChromaBlocks processes chroma blocks with MC prediction.
// Uses processChroma4x4WithPred for shared block processing logic.
func processInterChromaBlocks(src, pred []byte, mb *macroblock, qf QuantFactors, isU bool) {
	coeffs := mb.uCoeffs[:]
	if !isU {
		coeffs = mb.vCoeffs[:]
	}
	for by := 0; by < 2; by++ {
		for bx := 0; bx < 2; bx++ {
			processChroma4x4WithPred(src, pred, by, bx, coeffs, &mb.skip, qf)
		}
	}
}

// processChroma4x4WithPred processes a single 4x4 chroma block with given prediction.
// Shared helper for both inter and intra chroma processing.
func processChroma4x4WithPred(src, pred []byte, by, bx int, coeffs [][16]int16, skip *bool, qf QuantFactors) {
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
	coeffs[blockIdx] = ToZigzag(quantized)

	if hasNonZeroCoeffs(coeffs[blockIdx][:]) {
		*skip = false
	}
}

// processIntraChromaInInterFrame processes chroma blocks using intra prediction
// within an inter frame. This is used when a macroblock falls back to intra mode.
// Reuses processChromaPlane from macroblock.go for code sharing.
func processIntraChromaInInterFrame(srcU, srcV []byte, ctx *mbContext, mb *macroblock, qf QuantFactors) {
	processChromaPlane(srcU, ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU, mb.chromaMode, mb.uCoeffs[:], &mb.skip, qf)
	processChromaPlane(srcV, ctx.chromaAboveV, ctx.chromaLeftV, ctx.chromaTopLeftV, mb.chromaMode, mb.vCoeffs[:], &mb.skip, qf)
}
