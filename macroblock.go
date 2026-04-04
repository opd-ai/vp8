package vp8

import "fmt"

var debugMB = false // Set to true to debug macroblock mode selection

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
	lumaAboveBuf [16]byte
	lumaLeftBuf  [16]byte
	lumaAbove    []byte // 16 pixels above (nil if not available, else slice of lumaAboveBuf)
	lumaLeft     []byte // 16 pixels to the left (nil if not available, else slice of lumaLeftBuf)
	lumaTopLeft  byte   // pixel at (-1, -1)

	// Chroma neighbors (8x8 for each U/V) — fixed-size backing arrays.
	chromaAboveUBuf [8]byte
	chromaAboveVBuf [8]byte
	chromaLeftUBuf  [8]byte
	chromaLeftVBuf  [8]byte
	chromaAboveU    []byte
	chromaLeftU     []byte
	chromaTopLeftU  byte
	chromaAboveV    []byte
	chromaLeftV     []byte
	chromaTopLeftV  byte
}

// bPredSADThreshold controls when B_PRED mode is selected over 16×16 modes.
// B_PRED is selected when (sum of 4×4 SADs) * bPredSADThreshold < (best 16×16 SAD) * 100.
// A threshold of 90 means B_PRED must be at least 10% better to be selected,
// accounting for the additional bits needed to encode 16 sub-block modes.
const bPredSADThreshold = 90

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

	// Select best luma prediction mode
	selectLumaMode(srcY, ctx, &mb)

	// Select best 8x8 chroma prediction mode (same for U and V)
	mb.chromaMode, _ = SelectBest8x8ChromaMode(srcU, ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU)

	// Process Y blocks based on prediction mode
	if mb.lumaMode == B_PRED {
		processYBlocksBPred(srcY, ctx, &mb, qf)
	} else {
		processYBlocks16x16(srcY, ctx, &mb, qf)
	}

	// Process chroma blocks
	processChromaPlane(srcU, ctx.chromaAboveU, ctx.chromaLeftU, ctx.chromaTopLeftU, mb.chromaMode, mb.uCoeffs[:], &mb.skip, qf)
	processChromaPlane(srcV, ctx.chromaAboveV, ctx.chromaLeftV, ctx.chromaTopLeftV, mb.chromaMode, mb.vCoeffs[:], &mb.skip, qf)

	if debugMB && mb.lumaMode != B_PRED {
		fmt.Printf(", skip=%v\n", mb.skip)
	}

	return mb
}

// selectLumaMode selects the best luma prediction mode (16x16 or B_PRED).
func selectLumaMode(srcY []byte, ctx *mbContext, mb *macroblock) {
	best16x16Mode, best16x16SAD := SelectBest16x16Mode(srcY, ctx.lumaAbove, ctx.lumaLeft, ctx.lumaTopLeft)
	bpredSAD, bModes := evaluateBPredMode(srcY, ctx)

	if bpredSAD*100 < best16x16SAD*bPredSADThreshold {
		mb.lumaMode = B_PRED
		mb.bModes = bModes
		if debugMB {
			fmt.Printf("MB: B_PRED (SAD=%d < 16x16 SAD=%d * %d%%)\n", bpredSAD, best16x16SAD, bPredSADThreshold)
		}
	} else {
		mb.lumaMode = best16x16Mode
		if debugMB {
			fmt.Printf("MB: %v (SAD=%d, bpred SAD=%d)", best16x16Mode, best16x16SAD, bpredSAD)
		}
	}
}

// processChromaPlane processes a single chroma plane (U or V).
// Uses processChroma4x4WithPred from inter.go for shared block processing.
func processChromaPlane(src, above, left []byte, topLeft byte, mode chromaMode, coeffs [][16]int16, skip *bool, qf QuantFactors) {
	var pred [64]byte
	Predict8x8Chroma(pred[:], above, left, topLeft, mode)

	for by := 0; by < 2; by++ {
		for bx := 0; bx < 2; bx++ {
			processChroma4x4WithPred(src, pred[:], by, bx, coeffs, skip, qf)
		}
	}
}

// hasNonZeroCoeffs returns true if any coefficient is non-zero.
func hasNonZeroCoeffs(coeffs []int16) bool {
	for _, c := range coeffs {
		if c != 0 {
			return true
		}
	}
	return false
}

// hasNonZeroACCoeffs returns true if any AC coefficient (index > 0) is non-zero.
func hasNonZeroACCoeffs(coeffs [16]int16) bool {
	for i := 1; i < 16; i++ {
		if coeffs[i] != 0 {
			return true
		}
	}
	return false
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

	above[0] = build4x4TopLeft(by, bx, ctx, recon)
	build4x4Above(above, by, bx, ctx, recon)
	build4x4Left(left, by, bx, ctx, recon)

	return above, left
}

// build4x4TopLeft determines the top-left corner pixel (P) for a 4×4 sub-block.
func build4x4TopLeft(by, bx int, ctx *mbContext, recon []byte) byte {
	if by == 0 && bx == 0 {
		return ctx.lumaTopLeft
	}
	if by == 0 {
		if ctx.lumaAbove != nil && bx > 0 {
			return ctx.lumaAbove[bx*4-1]
		}
		return 128
	}
	if bx == 0 {
		if ctx.lumaLeft != nil {
			return ctx.lumaLeft[by*4-1]
		}
		return 128
	}
	return recon[(by*4-1)*16+(bx*4-1)]
}

// build4x4Above fills the above row (A[0..7]) for a 4×4 sub-block prediction.
func build4x4Above(above []byte, by, bx int, ctx *mbContext, recon []byte) {
	if by == 0 {
		build4x4AboveFromMBContext(above, bx, ctx)
		return
	}
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
		for i := 0; i < 4; i++ {
			above[5+i] = above[4]
		}
	}
}

// build4x4AboveFromMBContext fills above pixels from macroblock context (first row).
func build4x4AboveFromMBContext(above []byte, bx int, ctx *mbContext) {
	if ctx.lumaAbove == nil {
		for i := 0; i < 8; i++ {
			above[1+i] = 127
		}
		return
	}
	for i := 0; i < 4; i++ {
		col := bx*4 + i
		if col < len(ctx.lumaAbove) {
			above[1+i] = ctx.lumaAbove[col]
		} else {
			above[1+i] = 128
		}
	}
	for i := 0; i < 4; i++ {
		col := bx*4 + 4 + i
		if col < len(ctx.lumaAbove) {
			above[5+i] = ctx.lumaAbove[col]
		} else {
			above[5+i] = above[4]
		}
	}
}

// build4x4Left fills the left column (L[0..3]) for a 4×4 sub-block prediction.
func build4x4Left(left []byte, by, bx int, ctx *mbContext, recon []byte) {
	if bx == 0 {
		build4x4LeftFromMBContext(left, by, ctx)
		return
	}
	for i := 0; i < 4; i++ {
		left[i] = recon[(by*4+i)*16+bx*4-1]
	}
}

// build4x4LeftFromMBContext fills left pixels from macroblock context (first column).
func build4x4LeftFromMBContext(left []byte, by int, ctx *mbContext) {
	if ctx.lumaLeft == nil {
		for i := 0; i < 4; i++ {
			left[i] = 129
		}
		return
	}
	for i := 0; i < 4; i++ {
		row := by*4 + i
		if row < len(ctx.lumaLeft) {
			left[i] = ctx.lumaLeft[row]
		} else {
			left[i] = 129
		}
	}
}

// processYBlocks16x16 processes luma blocks using 16×16 prediction mode.
// It generates a single prediction, computes residuals, applies DCT/WHT.
func processYBlocks16x16(srcY []byte, ctx *mbContext, mb *macroblock, qf QuantFactors) {
	var predY [256]byte
	Predict16x16(predY[:], ctx.lumaAbove, ctx.lumaLeft, ctx.lumaTopLeft, mb.lumaMode)

	dcValues := processLumaBlocksWithDC(srcY, predY[:], mb, qf)
	applyWHTAndFinalize(dcValues, mb, qf)
}

// processLumaBlocksWithDC processes 16 luma 4x4 blocks and returns DC values for WHT.
func processLumaBlocksWithDC(srcY, predY []byte, mb *macroblock, qf QuantFactors) [16]int16 {
	var dcValues [16]int16
	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx
			src4x4 := extract4x4Block(srcY, by, bx)
			pred4x4 := extract4x4Block(predY, by, bx)
			dcValues[blockIdx] = processLuma4x4Block(src4x4, pred4x4, mb, blockIdx, qf)
		}
	}
	return dcValues
}

// processLuma4x4Block processes a single 4x4 luma block, returning its DC value.
func processLuma4x4Block(src4x4, pred4x4 [16]byte, mb *macroblock, blockIdx int, qf QuantFactors) int16 {
	residual := ComputeResidual(src4x4[:], pred4x4[:])
	dctCoeffs := ForwardDCT4x4(residual[:])
	quantized := QuantizeBlock(dctCoeffs, qf.Y1DC, qf.Y1AC)
	mb.yCoeffs[blockIdx] = ToZigzag(quantized)

	if hasNonZeroACCoeffs(mb.yCoeffs[blockIdx]) {
		mb.skip = false
	}
	return dctCoeffs[0]
}

// applyWHTAndFinalize applies WHT to DC values and finalizes the macroblock.
func applyWHTAndFinalize(dcValues [16]int16, mb *macroblock, qf QuantFactors) {
	whtCoeffs := ForwardWHT4x4(dcValues)
	quantizedWHT := QuantizeBlock(whtCoeffs, qf.Y2DC, qf.Y2AC)
	mb.y2Coeffs = ToZigzag(quantizedWHT)

	for i := 0; i < 16; i++ {
		mb.yCoeffs[i][0] = 0
	}

	if hasNonZeroCoeffs(mb.y2Coeffs[:]) {
		mb.skip = false
	}
}

// processYBlocksBPred processes luma blocks using B_PRED (4×4) mode.
// Each sub-block uses its own prediction mode; no WHT is applied.
func processYBlocksBPred(srcY []byte, ctx *mbContext, mb *macroblock, qf QuantFactors) {
	var recon [256]byte

	for by := 0; by < 4; by++ {
		for bx := 0; bx < 4; bx++ {
			blockIdx := by*4 + bx
			src4x4 := extract4x4Block(srcY, by, bx)
			above, left := build4x4Context(by, bx, ctx, recon[:])
			process4x4BPredBlock(src4x4, above, left, mb, blockIdx, qf, recon[:], by, bx)
		}
	}
}

// extract4x4Block extracts a 4x4 block from a 16x16 source.
func extract4x4Block(srcY []byte, by, bx int) [16]byte {
	var src4x4 [16]byte
	for row := 0; row < 4; row++ {
		srcRow := (by*4 + row) * 16
		copy(src4x4[row*4:row*4+4], srcY[srcRow+bx*4:srcRow+bx*4+4])
	}
	return src4x4
}

// process4x4BPredBlock processes a single 4x4 block in B_PRED mode.
func process4x4BPredBlock(src4x4 [16]byte, above, left []byte, mb *macroblock, blockIdx int, qf QuantFactors, recon []byte, by, bx int) {
	var pred4x4 [16]byte
	Predict4x4(pred4x4[:], above, left, mb.bModes[blockIdx])

	residual := ComputeResidual(src4x4[:], pred4x4[:])
	dctCoeffs := ForwardDCT4x4(residual[:])
	quantized := QuantizeBlock(dctCoeffs, qf.Y1DC, qf.Y1AC)
	mb.yCoeffs[blockIdx] = ToZigzag(quantized)

	reconstruct4x4Block(quantized, pred4x4[:], recon, by, bx, qf)

	if hasNonZeroCoeffs(mb.yCoeffs[blockIdx][:]) {
		mb.skip = false
	}
}

// reconstruct4x4Block reconstructs a 4x4 block for B_PRED context.
func reconstruct4x4Block(quantized [16]int16, pred, recon []byte, by, bx int, qf QuantFactors) {
	dequantized := DequantizeBlock(quantized, qf.Y1DC, qf.Y1AC)
	invDCT := InverseDCT4x4(dequantized)
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			val := int(pred[row*4+col]) + int(invDCT[row*4+col])
			recon[(by*4+row)*16+bx*4+col] = clamp8(val)
		}
	}
}
