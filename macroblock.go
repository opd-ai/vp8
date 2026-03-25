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

	// Inter-frame prediction fields.
	// isInter indicates this macroblock uses inter-frame prediction.
	isInter bool
	// refFrame indicates which reference frame is used for inter prediction.
	refFrame refFrameType
	// mv is the motion vector for inter prediction (quarter-pel precision).
	mv motionVector
	// predMV is the predicted motion vector derived from neighboring macroblocks.
	// Used as the base for delta-coding the MV in NEWMV mode.
	predMV motionVector
	// interMode is the inter prediction mode (NEARESTMV, NEARMV, ZEROMV, NEWMV).
	interMode interMode

	// yCoeffs holds the quantized DCT coefficients for the 16 Y (luma) 4x4 blocks.
	// Each block has 16 coefficients in zigzag order.
	yCoeffs [16][16]int16

	// y2Coeffs holds the quantized WHT coefficients for the Y2 (DC-of-DC) block.
	// Used for 16x16 prediction modes (not B_PRED) and inter modes.
	y2Coeffs [16]int16

	// uCoeffs holds the quantized DCT coefficients for the 4 U chroma 4x4 blocks.
	uCoeffs [4][16]int16

	// vCoeffs holds the quantized DCT coefficients for the 4 V chroma 4x4 blocks.
	vCoeffs [4][16]int16
}

// mbContext holds the neighbor context needed for macroblock prediction.
type mbContext struct {
	// Luma neighbors — fixed-size backing arrays to avoid per-MB heap allocations.
	lumaAboveBuf   [16]byte
	lumaLeftBuf    [16]byte
	lumaAbove      []byte // 16 pixels above (nil if not available, else slice of lumaAboveBuf)
	lumaLeft       []byte // 16 pixels to the left (nil if not available, else slice of lumaLeftBuf)
	lumaTopLeft    byte   // pixel at (-1, -1)

	// Chroma neighbors (8x8 for each U/V) — fixed-size backing arrays.
	chromaAboveUBuf [8]byte
	chromaAboveVBuf [8]byte
	chromaLeftUBuf  [8]byte
	chromaLeftVBuf  [8]byte
	chromaAboveU    []byte
	chromaLeftU     []byte
	chromaTopLeftU byte
	chromaAboveV   []byte
	chromaLeftV    []byte
	chromaTopLeftV byte
}

// bPredSADThreshold controls when B_PRED mode is selected over 16×16 modes.
// B_PRED is selected when (sum of 4×4 SADs) * bPredSADThreshold < (best 16×16 SAD) * 100.
// A threshold of 90 means B_PRED must be at least 10% better to be selected,
// accounting for the additional bits needed to encode 16 sub-block modes.
// TODO: B_PRED encoding has a bitstream issue causing decode failures.
// Temporarily disabled by setting threshold to 0 (never select B_PRED).
const bPredSADThreshold = 0

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
	best16x16Mode, best16x16SAD := SelectBest16x16Mode(srcY, ctx.lumaAbove, ctx.lumaLeft, ctx.lumaTopLeft)

	// Evaluate B_PRED: compute sum of SADs for all 16 4×4 blocks
	bpredSAD, bModes := evaluateBPredMode(srcY, ctx)

	// Compare: select B_PRED if it's significantly better
	// B_PRED requires encoding 16 sub-block modes, so it must be notably better
	if bpredSAD*100 < best16x16SAD*bPredSADThreshold {
		mb.lumaMode = B_PRED
		mb.bModes = bModes
	} else {
		mb.lumaMode = best16x16Mode
	}

	// Select best 8x8 chroma prediction mode (same for U and V)
	mb.chromaMode, _ = SelectBest8x8ChromaMode(srcU, ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU)

	// Process Y blocks based on prediction mode
	if mb.lumaMode == B_PRED {
		// B_PRED: use per-block 4×4 predictions
		processYBlocksBPred(srcY, ctx, &mb, qf)
	} else {
		// 16×16 mode: generate single prediction and process with WHT
		processYBlocks16x16(srcY, ctx, &mb, qf)
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

// evaluateBPredMode evaluates B_PRED mode for a 16×16 luma block.
// It returns the total SAD across all 16 4×4 blocks and the selected sub-modes.
func evaluateBPredMode(srcY []byte, ctx *mbContext) (int, [16]intraBMode) {
	var bModes [16]intraBMode
	totalSAD := 0

	// We need reconstructed pixels from previously encoded blocks to serve as
	// prediction context for subsequent blocks. Start with neighbor context.
	// For simplicity in mode selection, we use the original source pixels as
	// an approximation of the reconstruction (actual encoding uses true reconstruction).

	// Build reconstruction buffer initialized with neighbor context
	var recon [256]byte

	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx

			// Extract 4×4 source block
			var src4x4 [16]byte
			for row := 0; row < 4; row++ {
				srcRow := (by*4 + row) * 16
				copy(src4x4[row*4:row*4+4], srcY[srcRow+bx*4:srcRow+bx*4+4])
			}

			// Build context for this 4×4 block
			above, left := build4x4Context(by, bx, ctx, recon[:])

			// Select best mode for this block
			mode, sad := SelectBest4x4Mode(src4x4[:], above, left)
			bModes[blockIdx] = mode
			totalSAD += sad

			// Generate prediction and store in recon buffer for subsequent blocks
			var pred [16]byte
			Predict4x4(pred[:], above, left, mode)
			for row := 0; row < 4; row++ {
				reconRow := (by*4 + row) * 16
				// Use source as approximation of reconstruction for mode selection
				copy(recon[reconRow+bx*4:reconRow+bx*4+4], src4x4[row*4:row*4+4])
			}
		}
	}

	return totalSAD, bModes
}

// build4x4Context builds the neighbor context for a 4×4 sub-block.
// For blocks at macroblock edges, it uses the macroblock context.
// For interior blocks, it uses reconstructed pixels from previous blocks.
//
// Returns above (9 bytes: P + A[0..7]) and left (4 bytes: L[0..3]).
func build4x4Context(by, bx int, ctx *mbContext, recon []byte) (above, left []byte) {
	// The 'above' array for 4×4 prediction has format:
	// above[0] = P (top-left corner)
	// above[1..4] = A[0..3] (4 pixels directly above)
	// above[5..8] = A[4..7] (4 extra pixels for LD/VL modes)

	above = make([]byte, 9)
	left = make([]byte, 4)

	// Determine P (top-left corner)
	if by == 0 && bx == 0 {
		// Use macroblock's top-left
		above[0] = ctx.lumaTopLeft
	} else if by == 0 {
		// First row: P comes from macroblock above, last pixel of left column
		if ctx.lumaAbove != nil && bx > 0 {
			above[0] = ctx.lumaAbove[bx*4-1]
		} else {
			above[0] = 128
		}
	} else if bx == 0 {
		// First column: P comes from left neighbor macroblock
		if ctx.lumaLeft != nil {
			above[0] = ctx.lumaLeft[by*4-1]
		} else {
			above[0] = 128
		}
	} else {
		// Interior: P is the bottom-right of the top-left neighbor block
		above[0] = recon[(by*4-1)*16+(bx*4-1)]
	}

	// Determine A[0..7] (above row)
	if by == 0 {
		// First row of macroblock: use macroblock's above context
		if ctx.lumaAbove != nil {
			for i := 0; i < 4; i++ {
				col := bx*4 + i
				if col < len(ctx.lumaAbove) {
					above[1+i] = ctx.lumaAbove[col]
				} else {
					above[1+i] = 128
				}
			}
			// Extra 4 pixels for LD/VL modes
			for i := 0; i < 4; i++ {
				col := bx*4 + 4 + i
				if col < len(ctx.lumaAbove) {
					above[5+i] = ctx.lumaAbove[col]
				} else {
					above[5+i] = above[4] // repeat last available
				}
			}
		} else {
			for i := 0; i < 8; i++ {
				above[1+i] = 127
			}
		}
	} else {
		// Use reconstructed row above
		for i := 0; i < 4; i++ {
			above[1+i] = recon[(by*4-1)*16+bx*4+i]
		}
		// Extra pixels: from right neighbor block's top row or extend
		if bx < 3 {
			for i := 0; i < 4; i++ {
				above[5+i] = recon[(by*4-1)*16+(bx+1)*4+i]
			}
		} else {
			// Rightmost column: repeat last pixel
			for i := 0; i < 4; i++ {
				above[5+i] = above[4]
			}
		}
	}

	// Determine L[0..3] (left column)
	if bx == 0 {
		// First column of macroblock: use macroblock's left context
		if ctx.lumaLeft != nil {
			for i := 0; i < 4; i++ {
				row := by*4 + i
				if row < len(ctx.lumaLeft) {
					left[i] = ctx.lumaLeft[row]
				} else {
					left[i] = 129
				}
			}
		} else {
			for i := 0; i < 4; i++ {
				left[i] = 129
			}
		}
	} else {
		// Use reconstructed column to the left
		for i := 0; i < 4; i++ {
			left[i] = recon[(by*4+i)*16+bx*4-1]
		}
	}

	return above, left
}

// processYBlocks16x16 processes luma blocks using 16×16 prediction mode.
// It generates a single prediction, computes residuals, applies DCT/WHT.
func processYBlocks16x16(srcY []byte, ctx *mbContext, mb *macroblock, qf QuantFactors) {
	// Generate 16×16 prediction
	var predY [256]byte
	Predict16x16(predY[:], ctx.lumaAbove, ctx.lumaLeft, ctx.lumaTopLeft, mb.lumaMode)

	// Process 16 Y 4×4 blocks and collect DC values for WHT
	var dcValues [16]int16
	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx

			// Extract 4×4 source block
			var src4x4 [16]byte
			for row := 0; row < 4; row++ {
				srcRow := (by*4 + row) * 16
				copy(src4x4[row*4:row*4+4], srcY[srcRow+bx*4:srcRow+bx*4+4])
			}

			// Extract 4×4 prediction block
			var pred4x4 [16]byte
			for row := 0; row < 4; row++ {
				predRow := (by*4 + row) * 16
				copy(pred4x4[row*4:row*4+4], predY[predRow+bx*4:predRow+bx*4+4])
			}

			// Compute residual
			residual := ComputeResidual(src4x4[:], pred4x4[:])

			// Forward DCT
			dctCoeffs := ForwardDCT4x4(residual[:])

			// Store DC coefficient for WHT
			dcValues[blockIdx] = dctCoeffs[0]

			// Quantize
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
}

// processYBlocksBPred processes luma blocks using B_PRED (4×4) mode.
// Each sub-block uses its own prediction mode; no WHT is applied.
func processYBlocksBPred(srcY []byte, ctx *mbContext, mb *macroblock, qf QuantFactors) {
	// Build reconstruction buffer for inter-block dependencies
	var recon [256]byte

	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx

			// Extract 4×4 source block
			var src4x4 [16]byte
			for row := 0; row < 4; row++ {
				srcRow := (by*4 + row) * 16
				copy(src4x4[row*4:row*4+4], srcY[srcRow+bx*4:srcRow+bx*4+4])
			}

			// Build context for this block
			above, left := build4x4Context(by, bx, ctx, recon[:])

			// Generate prediction using the selected mode
			var pred4x4 [16]byte
			Predict4x4(pred4x4[:], above, left, mb.bModes[blockIdx])

			// Compute residual
			residual := ComputeResidual(src4x4[:], pred4x4[:])

			// Forward DCT
			dctCoeffs := ForwardDCT4x4(residual[:])

			// Quantize (for B_PRED, DC is included in Y1 plane)
			quantized := QuantizeBlock(dctCoeffs, qf.Y1DC, qf.Y1AC)

			// Store in zigzag order
			mb.yCoeffs[blockIdx] = ToZigzag(quantized)

			// Reconstruct for context of subsequent blocks
			dequantized := DequantizeBlock(quantized, qf.Y1DC, qf.Y1AC)
			invDCT := InverseDCT4x4(dequantized)
			for row := 0; row < 4; row++ {
				for col := 0; col < 4; col++ {
					val := int(pred4x4[row*4+col]) + int(invDCT[row*4+col])
					if val < 0 {
						val = 0
					}
					if val > 255 {
						val = 255
					}
					recon[(by*4+row)*16+bx*4+col] = byte(val)
				}
			}

			// Check for non-zero coefficients (include DC for B_PRED)
			for i := 0; i < 16; i++ {
				if mb.yCoeffs[blockIdx][i] != 0 {
					mb.skip = false
				}
			}
		}
	}

	// Y2 is not used for B_PRED mode
	// (y2Coeffs remains zero-initialized)
}
