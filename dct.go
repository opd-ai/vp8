package vp8

// This file implements the VP8 forward DCT and WHT transforms.
// Reference: RFC 6386 §14.1 (DCT) and §14.3 (WHT)
// Implementation based on libvpx vp8/encoder/dct.c and vp8/common/idctllm.c

// ForwardDCT4x4 performs the VP8 integer 4x4 forward DCT on a 4x4 residual block.
// The input is 16 signed 16-bit residual values (source - prediction).
// The output is 16 signed 16-bit DCT coefficients.
//
// Based on libvpx vp8_short_fdct4x4_c.
func ForwardDCT4x4(input []int16) [16]int16 {
	var temp [16]int16
	var output [16]int16

	// First pass: transform rows (with *8 scaling)
	for r := 0; r < 4; r++ {
		i := r * 4
		a1 := (int(input[i+0]) + int(input[i+3])) * 8
		b1 := (int(input[i+1]) + int(input[i+2])) * 8
		c1 := (int(input[i+1]) - int(input[i+2])) * 8
		d1 := (int(input[i+0]) - int(input[i+3])) * 8

		temp[i+0] = int16(a1 + b1)
		temp[i+2] = int16(a1 - b1)
		temp[i+1] = int16((c1*2217 + d1*5352 + 14500) >> 12)
		temp[i+3] = int16((d1*2217 - c1*5352 + 7500) >> 12)
	}

	// Second pass: transform columns
	for c := 0; c < 4; c++ {
		a1 := int(temp[c]) + int(temp[c+12])
		b1 := int(temp[c+4]) + int(temp[c+8])
		c1 := int(temp[c+4]) - int(temp[c+8])
		d1 := int(temp[c]) - int(temp[c+12])

		output[c] = int16((a1 + b1 + 7) >> 4)
		output[c+8] = int16((a1 - b1 + 7) >> 4)

		// Note: the (d1!=0) correction is for proper rounding
		correction := 0
		if d1 != 0 {
			correction = 1
		}
		output[c+4] = int16(((c1*2217 + d1*5352 + 12000) >> 16) + correction)
		output[c+12] = int16((d1*2217 - c1*5352 + 51000) >> 16)
	}

	return output
}

// ForwardWHT4x4 performs the VP8 4x4 Walsh-Hadamard Transform.
// This is used to transform the DC coefficients of the 16 Y subblocks
// into the Y2 block.
//
// Input: 16 DC values from the 16 Y subblocks
// Output: 16 WHT coefficients
//
// Reference: RFC 6386 §14.3
func ForwardWHT4x4(input [16]int16) [16]int16 {
	var temp [16]int16
	var output [16]int16

	// First pass: transform rows
	for r := 0; r < 4; r++ {
		i := r * 4
		a := int(input[i+0]) + int(input[i+2])
		d := int(input[i+1]) + int(input[i+3])
		c := int(input[i+0]) - int(input[i+2])
		b := int(input[i+1]) - int(input[i+3])

		temp[i+0] = int16(a + d)
		temp[i+1] = int16(c + b)
		temp[i+2] = int16(c - b)
		temp[i+3] = int16(a - d)
	}

	// Second pass: transform columns
	for c := 0; c < 4; c++ {
		a := int(temp[c]) + int(temp[c+8])
		d := int(temp[c+4]) + int(temp[c+12])
		cc := int(temp[c]) - int(temp[c+8])
		b := int(temp[c+4]) - int(temp[c+12])

		// Scale by 1/2 to match inverse WHT
		output[c] = int16((a + d) >> 1)
		output[c+4] = int16((cc + b) >> 1)
		output[c+8] = int16((cc - b) >> 1)
		output[c+12] = int16((a - d) >> 1)
	}

	return output
}

// QuantizeBlock quantizes a 4x4 block of DCT coefficients.
// The dcQ and acQ are the quantizer step sizes for DC and AC coefficients.
// Returns the quantized coefficients.
func QuantizeBlock(coeffs [16]int16, dcQ, acQ int16) [16]int16 {
	var out [16]int16

	// DC coefficient (index 0)
	if coeffs[0] >= 0 {
		out[0] = (coeffs[0] + dcQ/2) / dcQ
	} else {
		out[0] = (coeffs[0] - dcQ/2) / dcQ
	}

	// AC coefficients (indices 1-15)
	for i := 1; i < 16; i++ {
		if coeffs[i] >= 0 {
			out[i] = (coeffs[i] + acQ/2) / acQ
		} else {
			out[i] = (coeffs[i] - acQ/2) / acQ
		}
	}

	return out
}

// DequantizeBlock dequantizes a 4x4 block of quantized coefficients.
// This is the inverse of QuantizeBlock.
func DequantizeBlock(coeffs [16]int16, dcQ, acQ int16) [16]int16 {
	var out [16]int16

	out[0] = coeffs[0] * dcQ
	for i := 1; i < 16; i++ {
		out[i] = coeffs[i] * acQ
	}

	return out
}

// ComputeResidual computes the residual signal (source - prediction).
// Both src and pred are 4x4 blocks (16 bytes each).
// Returns 16 signed 16-bit residual values.
func ComputeResidual(src, pred []byte) [16]int16 {
	var residual [16]int16
	for i := 0; i < 16; i++ {
		residual[i] = int16(src[i]) - int16(pred[i])
	}
	return residual
}

// ComputeResidual16x16 computes the residual for a 16x16 block.
// Returns 256 signed 16-bit residual values.
func ComputeResidual16x16(src, pred []byte) [256]int16 {
	var residual [256]int16
	for i := 0; i < 256; i++ {
		residual[i] = int16(src[i]) - int16(pred[i])
	}
	return residual
}

// InverseDCT4x4 performs the VP8 integer 4x4 inverse DCT.
// This reconstructs the spatial residual from the DCT coefficients.
// The output values are added to the prediction to get the final pixels.
//
// Based on libvpx vp8_short_idct4x4llm_c (without the add-to-pred step).
// Reference: RFC 6386 §14.4
func InverseDCT4x4(input [16]int16) [16]int16 {
	var temp [16]int16
	var output [16]int16

	// VP8 inverse DCT constants
	// cospi8sqrt2minus1 = (sqrt(2) * cos(pi/8) - 1) * 65536 = 20091
	// sinpi8sqrt2 = sqrt(2) * sin(pi/8) * 65536 = 35468
	const cospi8sqrt2minus1 = 20091
	const sinpi8sqrt2 = 35468

	// First pass: transform columns
	for c := 0; c < 4; c++ {
		a1 := int(input[c]) + int(input[c+8])
		b1 := int(input[c]) - int(input[c+8])

		temp1 := (int(input[c+4]) * sinpi8sqrt2) >> 16
		temp2 := int(input[c+12]) + ((int(input[c+12]) * cospi8sqrt2minus1) >> 16)
		c1 := temp1 - temp2

		temp1 = int(input[c+4]) + ((int(input[c+4]) * cospi8sqrt2minus1) >> 16)
		temp2 = (int(input[c+12]) * sinpi8sqrt2) >> 16
		d1 := temp1 + temp2

		temp[c] = int16(a1 + d1)
		temp[c+12] = int16(a1 - d1)
		temp[c+4] = int16(b1 + c1)
		temp[c+8] = int16(b1 - c1)
	}

	// Second pass: transform rows
	for r := 0; r < 4; r++ {
		i := r * 4
		a1 := int(temp[i]) + int(temp[i+2])
		b1 := int(temp[i]) - int(temp[i+2])

		temp1 := (int(temp[i+1]) * sinpi8sqrt2) >> 16
		temp2 := int(temp[i+3]) + ((int(temp[i+3]) * cospi8sqrt2minus1) >> 16)
		c1 := temp1 - temp2

		temp1 = int(temp[i+1]) + ((int(temp[i+1]) * cospi8sqrt2minus1) >> 16)
		temp2 = (int(temp[i+3]) * sinpi8sqrt2) >> 16
		d1 := temp1 + temp2

		output[i+0] = int16((a1 + d1 + 4) >> 3)
		output[i+3] = int16((a1 - d1 + 4) >> 3)
		output[i+1] = int16((b1 + c1 + 4) >> 3)
		output[i+2] = int16((b1 - c1 + 4) >> 3)
	}

	return output
}

// InverseWHT4x4 performs the VP8 4x4 inverse Walsh-Hadamard Transform.
// Reference: RFC 6386 §14.3
func InverseWHT4x4(input [16]int16) [16]int16 {
	var temp [16]int16
	var output [16]int16

	// First pass: transform columns
	for c := 0; c < 4; c++ {
		a := int(input[c]) + int(input[c+12])
		b := int(input[c+4]) + int(input[c+8])
		cc := int(input[c+4]) - int(input[c+8])
		d := int(input[c]) - int(input[c+12])

		temp[c] = int16(a + b)
		temp[c+4] = int16(cc + d)
		temp[c+8] = int16(a - b)
		temp[c+12] = int16(d - cc)
	}

	// Second pass: transform rows
	for r := 0; r < 4; r++ {
		i := r * 4
		a := int(temp[i]) + int(temp[i+3])
		b := int(temp[i+1]) + int(temp[i+2])
		cc := int(temp[i+1]) - int(temp[i+2])
		d := int(temp[i]) - int(temp[i+3])

		output[i] = int16((a + b + 3) >> 3)
		output[i+1] = int16((cc + d + 3) >> 3)
		output[i+2] = int16((a - b + 3) >> 3)
		output[i+3] = int16((d - cc + 3) >> 3)
	}

	return output
}

// BlockHasNonZeroCoeffs returns true if any coefficient in the block is non-zero.
func BlockHasNonZeroCoeffs(coeffs [16]int16) bool {
	for _, c := range coeffs {
		if c != 0 {
			return true
		}
	}
	return false
}

// CountNonZeroCoeffs returns the number of non-zero coefficients in a block.
func CountNonZeroCoeffs(coeffs [16]int16) int {
	count := 0
	for _, c := range coeffs {
		if c != 0 {
			count++
		}
	}
	return count
}

// ZigzagOrder defines the zigzag scan order for 4x4 blocks.
// This maps from zigzag index to raster index.
var ZigzagOrder = [16]int{
	0, 1, 4, 8,
	5, 2, 3, 6,
	9, 12, 13, 10,
	7, 11, 14, 15,
}

// ToZigzag reorders coefficients from raster order to zigzag order.
func ToZigzag(raster [16]int16) [16]int16 {
	var zigzag [16]int16
	for zi, ri := range ZigzagOrder {
		zigzag[zi] = raster[ri]
	}
	return zigzag
}

// FromZigzag reorders coefficients from zigzag order to raster order.
func FromZigzag(zigzag [16]int16) [16]int16 {
	var raster [16]int16
	for zi, ri := range ZigzagOrder {
		raster[ri] = zigzag[zi]
	}
	return raster
}
