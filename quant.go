package vp8

// RFC 6386 §14.1 — Dequantization tables
// These tables map quantizer index (0–127) to dequantization step sizes.

// dcQLookup maps quantizer index to DC coefficient dequantization factor.
// Used directly for Y1 DC coefficients.
var dcQLookup = [128]int16{
	4, 5, 6, 7, 8, 9, 10, 10, 11, 12, 13, 14, 15,
	16, 17, 17, 18, 19, 20, 20, 21, 21, 22, 22, 23, 23,
	24, 25, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35,
	36, 37, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 46,
	47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59,
	60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72,
	73, 74, 75, 76, 76, 77, 78, 79, 80, 81, 82, 83, 84,
	85, 86, 87, 88, 89, 91, 93, 95, 96, 98, 100, 101, 102,
	104, 106, 108, 110, 112, 114, 116, 118, 122, 124, 126, 128, 130,
	132, 134, 136, 138, 140, 143, 145, 148, 151, 154, 157,
}

// acQLookup maps quantizer index to AC coefficient dequantization factor.
// Used directly for Y1 AC coefficients.
var acQLookup = [128]int16{
	4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
	17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
	30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42,
	43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55,
	56, 57, 58, 60, 62, 64, 66, 68, 70, 72, 74, 76, 78,
	80, 82, 84, 86, 88, 90, 92, 94, 96, 98, 100, 102, 104,
	106, 108, 110, 112, 114, 116, 119, 122, 125, 128, 131, 134, 137,
	140, 143, 146, 149, 152, 155, 158, 161, 164, 167, 170, 173, 177,
	181, 185, 189, 193, 197, 201, 205, 209, 213, 217, 221, 225, 229,
	234, 239, 245, 249, 254, 259, 264, 269, 274, 279, 284,
}

// QuantFactors holds the six dequantization factors for a given quantizer index.
// These are used to dequantize DCT/WHT coefficients before inverse transform.
type QuantFactors struct {
	Y1DC int16 // Y1 (luma 4x4 blocks) DC coefficient factor
	Y1AC int16 // Y1 AC coefficients factor
	Y2DC int16 // Y2 (DC-of-DC WHT block) DC coefficient factor
	Y2AC int16 // Y2 AC coefficients factor
	UVDC int16 // Chroma DC coefficient factor
	UVAC int16 // Chroma AC coefficients factor
}

// clampQI clamps a quantizer index to the valid range [0, 127].
func clampQI(qi int) int {
	if qi < 0 {
		return 0
	}
	if qi > 127 {
		return 127
	}
	return qi
}

// GetQuantFactors returns the six dequantization factors for a given base
// quantizer index and per-plane delta values. The deltas allow adjusting
// quantization strength for different coefficient types.
//
// Parameters:
//   - qi: base quantizer index [0, 127]
//   - y1DCDelta: delta for Y1 DC coefficients (added to qi)
//   - y2DCDelta: delta for Y2 DC coefficients
//   - y2ACDelta: delta for Y2 AC coefficients
//   - uvDCDelta: delta for UV DC coefficients
//   - uvACDelta: delta for UV AC coefficients
//
// Reference: RFC 6386 §9.6 and §14.1
func GetQuantFactors(qi, y1DCDelta, y2DCDelta, y2ACDelta, uvDCDelta, uvACDelta int) QuantFactors {
	// Y1 DC: use base qi with y1DCDelta
	y1dcQI := clampQI(qi + y1DCDelta)
	y1dc := dcQLookup[y1dcQI]

	// Y1 AC: use base qi directly (no delta in frame header for Y1 AC)
	y1acQI := clampQI(qi)
	y1ac := acQLookup[y1acQI]

	// Y2 DC: scale by 2, minimum 8
	y2dcQI := clampQI(qi + y2DCDelta)
	y2dc := dcQLookup[y2dcQI] * 2
	if y2dc < 8 {
		y2dc = 8
	}

	// Y2 AC: scale by 155/100, minimum 8
	y2acQI := clampQI(qi + y2ACDelta)
	y2ac := int16((int(acQLookup[y2acQI]) * 155) / 100)
	if y2ac < 8 {
		y2ac = 8
	}

	// UV DC: clamp to maximum 132
	uvdcQI := clampQI(qi + uvDCDelta)
	uvdc := dcQLookup[uvdcQI]
	if uvdc > 132 {
		uvdc = 132
	}

	// UV AC: use AC table directly
	uvacQI := clampQI(qi + uvACDelta)
	uvac := acQLookup[uvacQI]

	return QuantFactors{
		Y1DC: y1dc,
		Y1AC: y1ac,
		Y2DC: y2dc,
		Y2AC: y2ac,
		UVDC: uvdc,
		UVAC: uvac,
	}
}

// GetQuantFactorsSimple returns quantization factors using only the base
// quantizer index with no deltas. This is a convenience function for the
// common case where no per-plane adjustments are needed.
func GetQuantFactorsSimple(qi int) QuantFactors {
	return GetQuantFactors(qi, 0, 0, 0, 0, 0)
}

// quantIndexToQp returns an approximate quantizer step size for a given
// quantizer index (0..127). This follows the VP8 dequantization table.
//
// Deprecated: Use GetQuantFactors or GetQuantFactorsSimple for accurate
// per-plane dequantization factors as specified in RFC 6386 §14.1.
func quantIndexToQp(qi int) int16 {
	// Use the Y1 AC lookup table for backward compatibility
	return acQLookup[clampQI(qi)]
}
