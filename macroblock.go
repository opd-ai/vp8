package vp8

// macroblock holds the data for one 16x16 VP8 macroblock.
type macroblock struct {
	lumaMode   intraMode
	chromaMode chromaMode
	// skip indicates all quantized coefficients are zero.
	skip bool
	// dcValue is the DC coefficient value (after quantization).
	dcValue int16

	// bModes holds the 4x4 sub-block prediction modes when lumaMode is B_PRED.
	// Layout: 16 blocks in raster order (row-major, left-to-right, top-to-bottom).
	bModes [16]intraBMode

	// yCoeffs holds the quantized DCT coefficients for the 16 Y (luma) 4x4 blocks.
	// Each block has 16 coefficients in zigzag order.
	yCoeffs [16][16]int16

	// y2Coeffs holds the quantized WHT coefficients for the Y2 (DC-of-DC) block.
	// Used for 16x16 prediction modes (not B_PRED).
	y2Coeffs [16]int16

	// uCoeffs holds the quantized DCT coefficients for the 4 U chroma 4x4 blocks.
	uCoeffs [4][16]int16

	// vCoeffs holds the quantized DCT coefficients for the 4 V chroma 4x4 blocks.
	vCoeffs [4][16]int16
}

// mbContext holds the neighbor context needed for macroblock prediction.
type mbContext struct {
	// Luma neighbors
	lumaAbove   []byte // 16 pixels above
	lumaLeft    []byte // 16 pixels to the left
	lumaTopLeft byte   // pixel at (-1, -1)

	// Chroma neighbors (8x8 for each U/V)
	chromaAboveU   []byte
	chromaLeftU    []byte
	chromaTopLeftU byte
	chromaAboveV   []byte
	chromaLeftV    []byte
	chromaTopLeftV byte
}

// processMacroblock processes a 16x16 macroblock from source YUV data.
// It selects the best prediction modes, computes residuals, transforms
// and quantizes coefficients, and determines the skip flag.
//
// Parameters:
//   - srcY: source luma block (256 bytes, 16x16)
//   - srcU: source U chroma block (64 bytes, 8x8)
//   - srcV: source V chroma block (64 bytes, 8x8)
//   - ctx: neighbor context for prediction
//   - qf: quantization factors
func processMacroblock(srcY, srcU, srcV []byte, ctx *mbContext, qf QuantFactors) macroblock {
	mb := macroblock{
		skip: true,
	}

	// Select best 16x16 luma prediction mode
	mb.lumaMode, _ = SelectBest16x16Mode(srcY, ctx.lumaAbove, ctx.lumaLeft, ctx.lumaTopLeft)

	// Select best 8x8 chroma prediction mode (same for U and V)
	mb.chromaMode, _ = SelectBest8x8ChromaMode(srcU, ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU)

	// Generate luma prediction
	var predY [256]byte
	Predict16x16(predY[:], ctx.lumaAbove, ctx.lumaLeft, ctx.lumaTopLeft, mb.lumaMode)

	// Process 16 Y 4x4 blocks and collect DC values for WHT
	var dcValues [16]int16
	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx

			// Extract 4x4 source block
			var src4x4 [16]byte
			for row := 0; row < 4; row++ {
				srcRow := (by*4 + row) * 16
				copy(src4x4[row*4:row*4+4], srcY[srcRow+bx*4:srcRow+bx*4+4])
			}

			// Extract 4x4 prediction block
			var pred4x4 [16]byte
			for row := 0; row < 4; row++ {
				predRow := (by*4 + row) * 16
				copy(pred4x4[row*4:row*4+4], predY[predRow+bx*4:predRow+bx*4+4])
			}

			// Compute residual
			residual := ComputeResidual(src4x4[:], pred4x4[:])

			// Forward DCT
			dctCoeffs := ForwardDCT4x4(residual[:])

			// Store DC coefficient for WHT, quantize AC
			dcValues[blockIdx] = dctCoeffs[0]

			// Quantize (DC will be replaced after WHT)
			quantized := QuantizeBlock(dctCoeffs, qf.Y1DC, qf.Y1AC)

			// Store in zigzag order
			mb.yCoeffs[blockIdx] = ToZigzag(quantized)

			// Check for non-zero AC coefficients
			for i := 1; i < 16; i++ {
				if mb.yCoeffs[blockIdx][i] != 0 {
					mb.skip = false
				}
			}
		}
	}

	// Apply WHT to DC values
	whtCoeffs := ForwardWHT4x4(dcValues)
	quantizedWHT := QuantizeBlock(whtCoeffs, qf.Y2DC, qf.Y2AC)
	mb.y2Coeffs = ToZigzag(quantizedWHT)

	// Clear DC coefficients from Y blocks (they're encoded via Y2)
	for i := 0; i < 16; i++ {
		mb.yCoeffs[i][0] = 0
	}

	// Check if Y2 has non-zero coefficients
	for i := 0; i < 16; i++ {
		if mb.y2Coeffs[i] != 0 {
			mb.skip = false
		}
	}

	// Process U chroma (4 blocks of 4x4)
	var predU [64]byte
	Predict8x8Chroma(predU[:], ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU, mb.chromaMode)
	for by := 0; by < 2; by++ {
		for bx := 0; bx < 2; bx++ {
			blockIdx := by*2 + bx

			// Extract 4x4 blocks
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

	// Process V chroma (4 blocks of 4x4)
	var predV [64]byte
	Predict8x8Chroma(predV[:], ctx.chromaAboveV, ctx.chromaLeftV, ctx.chromaTopLeftV, mb.chromaMode)
	for by := 0; by < 2; by++ {
		for bx := 0; bx < 2; bx++ {
			blockIdx := by*2 + bx

			// Extract 4x4 blocks
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

	return mb
}

// processMacroblockSimple returns a macroblock descriptor with fixed modes.
// This is the legacy function for backward compatibility with existing tests.
func processMacroblockSimple() macroblock {
	return macroblock{
		lumaMode:   DC_PRED,
		chromaMode: DC_PRED_CHROMA,
		skip:       true,
		dcValue:    0,
	}
}
