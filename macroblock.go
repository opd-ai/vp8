package vp8

// macroblock holds the data for one 16x16 VP8 macroblock.
type macroblock struct {
	lumaMode   intraMode
	chromaMode chromaMode
	// skip indicates all quantized coefficients are zero.
	skip bool
	// dcValue is the DC coefficient value (after quantization).
	dcValue int16
}

// dct4x4 performs a 2D forward DCT on a 4x4 block (in-place, scaled).
// This is the integer approximation used by VP8.
func dct4x4(in [16]int16) [16]int16 {
	var tmp [16]int16
	// Row transform
	for i := 0; i < 4; i++ {
		a := int32(in[i*4+0] + in[i*4+3])
		b := int32(in[i*4+1] + in[i*4+2])
		c := int32(in[i*4+1] - in[i*4+2])
		d := int32(in[i*4+0] - in[i*4+3])
		tmp[i*4+0] = int16(a + b)
		tmp[i*4+1] = int16(c + d)
		tmp[i*4+2] = int16(a - b)
		tmp[i*4+3] = int16(d - c)
	}
	var out [16]int16
	// Column transform
	for j := 0; j < 4; j++ {
		a := int32(tmp[0*4+j] + tmp[3*4+j])
		b := int32(tmp[1*4+j] + tmp[2*4+j])
		c := int32(tmp[1*4+j] - tmp[2*4+j])
		d := int32(tmp[0*4+j] - tmp[3*4+j])
		out[0*4+j] = int16((a+b+3) >> 3)
		out[1*4+j] = int16((c+d+3) >> 3)
		out[2*4+j] = int16((a-b+3) >> 3)
		out[3*4+j] = int16((d-c+3) >> 3)
	}
	return out
}

// quantize divides each coefficient by the step size and rounds.
func quantize(coeff int16, step int16) int16 {
	if step <= 0 {
		return 0
	}
	sign := int16(1)
	if coeff < 0 {
		sign = -1
		coeff = -coeff
	}
	return sign * ((coeff + step/2) / step)
}

// processMacroblock builds a macroblock descriptor for a 16x16 block
// starting at (bx, by) within the frame. It computes residuals, applies
// DCT and quantization, and marks the block as skipped when all coefficients
// are zero.
func processMacroblock(f *Frame, bx, by int, qp int16) macroblock {
	if qp <= 0 {
		qp = 1
	}
	startY := by * 16
	startX := bx * 16

	// Compute the simple DC predictor value (128 at borders; otherwise
	// average of top row/left column). For this simplified encoder we
	// always use 128 as the DC prediction value.
	pred := byte(128)

	// Compute the 4x4 luma DC block (from 2x2 block grid of the MB).
	// We use a single representative DC value from each 4x4 block.
	allZero := true
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			var blk [16]int16
			for r := 0; r < 4; r++ {
				for c := 0; c < 4; c++ {
					yr := startY + row*4 + r
					xc := startX + col*4 + c
					if yr >= f.Height || xc >= f.Width {
						blk[r*4+c] = 0
					} else {
						blk[r*4+c] = int16(f.Y[yr*f.Width+xc]) - int16(pred)
					}
				}
			}
			dctOut := dct4x4(blk)
			for i := range dctOut {
				q := quantize(dctOut[i], qp)
				if q != 0 {
					allZero = false
				}
			}
		}
	}

	return macroblock{
		lumaMode:   DC_PRED,
		chromaMode: DC_PRED_CHROMA,
		skip:       allZero,
		dcValue:    0, // simplified: residuals not encoded
	}
}
