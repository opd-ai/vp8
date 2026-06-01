package vp8

import (
	"testing"
)

// TestPropagateCarry verifies the carry propagation logic (AUDIT LOW finding).
// Input [5, 0xff, 0xff] should produce [6, 0x00, 0x00].
func TestPropagateCarry(t *testing.T) {
	tests := []struct {
		input []byte
		want  []byte
	}{
		{[]byte{5, 0xff, 0xff}, []byte{6, 0x00, 0x00}},
		{[]byte{0xff, 0xff, 0xff}, []byte{0x00, 0x00, 0x00}}, // all-0xff: carry off end
		{[]byte{10}, []byte{11}},
		{[]byte{0x7f, 0xff}, []byte{0x80, 0x00}},
		{[]byte{}, []byte{}}, // empty buffer: no-op
	}

	for _, tt := range tests {
		enc := &boolEncoder{buf: append([]byte(nil), tt.input...)}
		enc.propagateCarry()
		if len(enc.buf) != len(tt.want) {
			t.Errorf("input %v: len %d, want %d", tt.input, len(enc.buf), len(tt.want))
			continue
		}
		for i, b := range enc.buf {
			if b != tt.want[i] {
				t.Errorf("input %v: buf[%d] = %d, want %d", tt.input, i, b, tt.want[i])
			}
		}
	}
}
