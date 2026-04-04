package vp8

// This file implements support for multiple DCT/residual partitions.
// Reference: RFC 6386 §9.5 and §9.7

// PartitionCount represents the number of DCT partitions.
// VP8 supports 1, 2, 4, or 8 partitions.
type PartitionCount int

const (
	OnePartition    PartitionCount = 0 // log2(1) = 0
	TwoPartitions   PartitionCount = 1 // log2(2) = 1
	FourPartitions  PartitionCount = 2 // log2(4) = 2
	EightPartitions PartitionCount = 3 // log2(8) = 3
)

// NumPartitions returns the actual number of partitions.
func (p PartitionCount) NumPartitions() int {
	return 1 << int(p)
}

// PartitionWriter manages writing to multiple DCT partitions.
// Each partition has its own boolean encoder.
type PartitionWriter struct {
	partitionCount PartitionCount
	encoders       []*boolEncoder
	coeffProbs     *[4][8][3][11]uint8
	histogram      *CoeffHistogram
}

// NewPartitionWriter creates a new partition writer with the specified
// number of partitions and coefficient probabilities.
func NewPartitionWriter(count PartitionCount, probs *[4][8][3][11]uint8) *PartitionWriter {
	return NewPartitionWriterWithHistogram(count, probs, nil)
}

// NewPartitionWriterWithHistogram creates a partition writer with optional histogram.
func NewPartitionWriterWithHistogram(count PartitionCount, probs *[4][8][3][11]uint8, probCfg *ProbConfig) *PartitionWriter {
	n := count.NumPartitions()
	encoders := make([]*boolEncoder, n)
	for i := range encoders {
		encoders[i] = newBoolEncoder()
	}
	var histogram *CoeffHistogram
	if probCfg != nil {
		histogram = probCfg.Histogram
	}
	return &PartitionWriter{
		partitionCount: count,
		encoders:       encoders,
		coeffProbs:     probs,
		histogram:      histogram,
	}
}

// GetTokenEncoder returns a token encoder for the specified macroblock row.
// Macroblocks are distributed across partitions by row:
//   - row % numPartitions determines which partition to use.
func (pw *PartitionWriter) GetTokenEncoder(mbRow int) *TokenEncoder {
	partIdx := mbRow % pw.partitionCount.NumPartitions()
	te := NewTokenEncoder(pw.encoders[partIdx], pw.coeffProbs)
	if pw.histogram != nil {
		te.SetHistogram(pw.histogram)
	}
	return te
}

// GetEncoder returns the boolean encoder for the specified partition.
func (pw *PartitionWriter) GetEncoder(partIdx int) *boolEncoder {
	return pw.encoders[partIdx]
}

// Finalize flushes all partition encoders and returns the partition data.
// Returns a slice of byte slices, one per partition.
func (pw *PartitionWriter) Finalize() [][]byte {
	parts := make([][]byte, len(pw.encoders))
	for i, enc := range pw.encoders {
		parts[i] = enc.flush()
	}
	return parts
}

// BuildPartitionSizes returns the partition size bytes for the frame.
// According to RFC 6386 §9.7, the sizes of all partitions except the last
// are written as 3-byte little-endian values between the first partition
// and the residual data.
func BuildPartitionSizes(partitions [][]byte) []byte {
	if len(partitions) <= 1 {
		return nil // No size bytes needed for single partition
	}

	// Write sizes for all partitions except the last
	sizes := make([]byte, (len(partitions)-1)*3)
	for i := 0; i < len(partitions)-1; i++ {
		size := uint32(len(partitions[i]))
		// 3-byte little-endian
		sizes[i*3] = byte(size)
		sizes[i*3+1] = byte(size >> 8)
		sizes[i*3+2] = byte(size >> 16)
	}
	return sizes
}

// ConcatPartitions concatenates all partition data into a single slice.
func ConcatPartitions(partitions [][]byte) []byte {
	total := 0
	for _, p := range partitions {
		total += len(p)
	}

	result := make([]byte, 0, total)
	for _, p := range partitions {
		result = append(result, p...)
	}
	return result
}

// EncodePartitionCount encodes the partition count into the frame header.
// This is a 2-bit field (log2 of partition count).
func EncodePartitionCount(enc *boolEncoder, count PartitionCount) {
	enc.putLiteral(uint32(count), 2)
}

// PartitionConfig holds configuration for partition-based encoding.
type PartitionConfig struct {
	Count      PartitionCount
	CoeffProbs *[4][8][3][11]uint8
}

// DefaultPartitionConfig returns a single-partition configuration with
// default coefficient probabilities.
func DefaultPartitionConfig() PartitionConfig {
	probs := DefaultCoeffProbs
	return PartitionConfig{
		Count:      OnePartition,
		CoeffProbs: &probs,
	}
}

// AssembleMultiPartitionFrame assembles a VP8 frame with multiple partitions.
// firstPart: the first partition (frame header + MB modes)
// residualParts: the residual data partitions
// Returns the assembled frame data starting after the frame tag/header.
func AssembleMultiPartitionFrame(firstPart []byte, residualParts [][]byte) []byte {
	partSizes := BuildPartitionSizes(residualParts)
	residualData := ConcatPartitions(residualParts)

	// Total size: first partition + size bytes + residual data
	total := len(firstPart) + len(partSizes) + len(residualData)
	result := make([]byte, 0, total)
	result = append(result, firstPart...)
	result = append(result, partSizes...)
	result = append(result, residualData...)

	return result
}
