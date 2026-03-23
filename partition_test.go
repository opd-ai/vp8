package vp8

import (
	"testing"
)

func TestPartitionCountNumPartitions(t *testing.T) {
	tests := []struct {
		count    PartitionCount
		expected int
	}{
		{OnePartition, 1},
		{TwoPartitions, 2},
		{FourPartitions, 4},
		{EightPartitions, 8},
	}

	for _, tt := range tests {
		got := tt.count.NumPartitions()
		if got != tt.expected {
			t.Errorf("PartitionCount(%d).NumPartitions() = %d, want %d",
				tt.count, got, tt.expected)
		}
	}
}

func TestNewPartitionWriter(t *testing.T) {
	probs := DefaultCoeffProbs

	tests := []PartitionCount{OnePartition, TwoPartitions, FourPartitions, EightPartitions}

	for _, count := range tests {
		pw := NewPartitionWriter(count, &probs)

		if len(pw.encoders) != count.NumPartitions() {
			t.Errorf("NewPartitionWriter(%d) created %d encoders, want %d",
				count, len(pw.encoders), count.NumPartitions())
		}

		if pw.coeffProbs != &probs {
			t.Error("Coefficient probs not properly set")
		}
	}
}

func TestPartitionWriterGetTokenEncoder(t *testing.T) {
	probs := DefaultCoeffProbs
	pw := NewPartitionWriter(FourPartitions, &probs)

	// Test that different rows get different partitions
	for row := 0; row < 8; row++ {
		te := pw.GetTokenEncoder(row)
		if te == nil {
			t.Errorf("GetTokenEncoder(%d) returned nil", row)
		}
	}
}

func TestPartitionWriterRowDistribution(t *testing.T) {
	probs := DefaultCoeffProbs
	pw := NewPartitionWriter(FourPartitions, &probs)

	// Rows should be distributed round-robin across partitions
	// Row 0 -> partition 0
	// Row 1 -> partition 1
	// Row 2 -> partition 2
	// Row 3 -> partition 3
	// Row 4 -> partition 0
	// etc.

	// Encode something unique to each row to verify distribution
	for row := 0; row < 8; row++ {
		te := pw.GetTokenEncoder(row)
		coeffs := [16]int16{int16(row + 1), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		te.EncodeBlock(coeffs, BlockTypeACY, 0)
	}

	parts := pw.Finalize()

	// All 4 partitions should have data
	for i, p := range parts {
		if len(p) == 0 {
			t.Errorf("Partition %d should have data", i)
		}
	}
}

func TestBuildPartitionSizes(t *testing.T) {
	// Single partition - no sizes needed
	parts1 := [][]byte{{1, 2, 3}}
	sizes1 := BuildPartitionSizes(parts1)
	if sizes1 != nil {
		t.Error("Single partition should return nil sizes")
	}

	// Two partitions - one size (for first partition)
	parts2 := [][]byte{
		{1, 2, 3},       // size 3
		{4, 5, 6, 7, 8}, // size 5 (not encoded)
	}
	sizes2 := BuildPartitionSizes(parts2)
	if len(sizes2) != 3 {
		t.Errorf("Two partitions should have 3 size bytes, got %d", len(sizes2))
	}
	// Check little-endian encoding of size 3
	if sizes2[0] != 3 || sizes2[1] != 0 || sizes2[2] != 0 {
		t.Errorf("Size bytes = %v, want [3, 0, 0]", sizes2)
	}

	// Four partitions - three sizes
	parts4 := [][]byte{
		make([]byte, 256),   // size 256
		make([]byte, 1),     // size 1
		make([]byte, 65536), // size 65536
		make([]byte, 100),   // not encoded
	}
	sizes4 := BuildPartitionSizes(parts4)
	if len(sizes4) != 9 {
		t.Errorf("Four partitions should have 9 size bytes, got %d", len(sizes4))
	}
	// First size: 256 = 0x000100
	if sizes4[0] != 0 || sizes4[1] != 1 || sizes4[2] != 0 {
		t.Errorf("First size = [%d, %d, %d], want [0, 1, 0]", sizes4[0], sizes4[1], sizes4[2])
	}
}

func TestConcatPartitions(t *testing.T) {
	parts := [][]byte{
		{1, 2, 3},
		{4, 5},
		{6, 7, 8, 9},
	}

	result := ConcatPartitions(parts)
	expected := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}

	if len(result) != len(expected) {
		t.Fatalf("ConcatPartitions length = %d, want %d", len(result), len(expected))
	}

	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %d, want %d", i, result[i], v)
		}
	}
}

func TestEncodePartitionCount(t *testing.T) {
	tests := []PartitionCount{OnePartition, TwoPartitions, FourPartitions, EightPartitions}

	for _, count := range tests {
		enc := newBoolEncoder()
		EncodePartitionCount(enc, count)
		data := enc.flush()
		if len(data) == 0 {
			t.Errorf("EncodePartitionCount(%d) should produce output", count)
		}
	}
}

func TestDefaultPartitionConfig(t *testing.T) {
	cfg := DefaultPartitionConfig()

	if cfg.Count != OnePartition {
		t.Errorf("Default count = %d, want OnePartition", cfg.Count)
	}

	if cfg.CoeffProbs == nil {
		t.Error("Default CoeffProbs should not be nil")
	}
}

func TestAssembleMultiPartitionFrame(t *testing.T) {
	firstPart := []byte{1, 2, 3}
	residualParts := [][]byte{
		{10, 11, 12},
		{20, 21},
	}

	result := AssembleMultiPartitionFrame(firstPart, residualParts)

	// Expected: firstPart (3) + size of first residual (3) + residuals (3+2)
	// Total: 3 + 3 + 5 = 11
	expectedLen := 11
	if len(result) != expectedLen {
		t.Errorf("Assembled frame length = %d, want %d", len(result), expectedLen)
	}

	// First 3 bytes should be firstPart
	for i := 0; i < 3; i++ {
		if result[i] != firstPart[i] {
			t.Errorf("result[%d] = %d, want %d (firstPart)", i, result[i], firstPart[i])
		}
	}

	// Next 3 bytes should be size of first residual (3)
	if result[3] != 3 || result[4] != 0 || result[5] != 0 {
		t.Errorf("Size bytes = [%d, %d, %d], want [3, 0, 0]", result[3], result[4], result[5])
	}

	// Then the residual data
	if result[6] != 10 || result[7] != 11 || result[8] != 12 {
		t.Error("First residual data mismatch")
	}
	if result[9] != 20 || result[10] != 21 {
		t.Error("Second residual data mismatch")
	}
}
