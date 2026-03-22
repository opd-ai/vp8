package vp8

import (
	"testing"
)

// makeYUV420 creates a synthetic YUV420 frame of the given dimensions.
// The luma plane is filled with the given value; chroma planes use 128.
func makeYUV420(width, height int, lumaVal byte) []byte {
	ySize := width * height
	uvSize := (width / 2) * (height / 2)
	buf := make([]byte, ySize+2*uvSize)
	for i := 0; i < ySize; i++ {
		buf[i] = lumaVal
	}
	for i := ySize; i < len(buf); i++ {
		buf[i] = 128
	}
	return buf
}

func TestNewEncoder(t *testing.T) {
	tests := []struct {
		name        string
		w, h, fps   int
		wantErr     bool
	}{
		{"valid 640x480 30fps", 640, 480, 30, false},
		{"valid 1280x720 30fps", 1280, 720, 30, false},
		{"zero width", 0, 480, 30, true},
		{"negative height", 640, -1, 30, true},
		{"odd width", 641, 480, 30, true},
		{"zero fps", 640, 480, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := NewEncoder(tt.w, tt.h, tt.fps)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewEncoder(%d,%d,%d) error = %v, wantErr %v",
					tt.w, tt.h, tt.fps, err, tt.wantErr)
			}
			if !tt.wantErr && enc == nil {
				t.Fatal("expected non-nil encoder")
			}
		})
	}
}

func TestEncodeBlackFrame(t *testing.T) {
	enc, err := NewEncoder(640, 480, 30)
	if err != nil {
		t.Fatal(err)
	}
	yuv := makeYUV420(640, 480, 16) // Y=16 is "black" in studio swing
	pkt, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(pkt) < 10 {
		t.Fatalf("encoded packet too short: %d bytes", len(pkt))
	}
	// Verify VP8 key-frame tag: bit 0 of first byte must be 0 (key frame).
	if pkt[0]&1 != 0 {
		t.Errorf("expected key frame (bit0=0), got 0x%02x", pkt[0])
	}
	// Verify VP8 start code bytes at offset 3.
	if pkt[3] != 0x9D || pkt[4] != 0x01 || pkt[5] != 0x2A {
		t.Errorf("unexpected start code: %02x %02x %02x", pkt[3], pkt[4], pkt[5])
	}
}

func TestEncodeWhiteFrame(t *testing.T) {
	enc, err := NewEncoder(320, 240, 15)
	if err != nil {
		t.Fatal(err)
	}
	yuv := makeYUV420(320, 240, 235) // Y=235 is "white" in studio swing
	pkt, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(pkt) == 0 {
		t.Fatal("got empty packet")
	}
}

func TestSetBitrate(t *testing.T) {
	enc, _ := NewEncoder(640, 480, 30)
	enc.SetBitrate(1_000_000)
	// qi should be in valid range
	if enc.qi < 0 || enc.qi > 127 {
		t.Errorf("qi out of range: %d", enc.qi)
	}
}

func TestForceKeyFrame(t *testing.T) {
	enc, _ := NewEncoder(640, 480, 30)
	enc.ForceKeyFrame() // should not panic
	yuv := makeYUV420(640, 480, 128)
	pkt, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode after ForceKeyFrame: %v", err)
	}
	if pkt[0]&1 != 0 {
		t.Errorf("expected key frame bit, got 0x%02x", pkt[0])
	}
}

func TestNewYUV420Frame(t *testing.T) {
	// Too small
	_, err := NewYUV420Frame(make([]byte, 10), 640, 480)
	if err == nil {
		t.Error("expected error for undersized buffer")
	}
	// Exact size
	yuv := make([]byte, 640*480*3/2)
	f, err := NewYUV420Frame(yuv, 640, 480)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Width != 640 || f.Height != 480 {
		t.Errorf("unexpected dimensions: %dx%d", f.Width, f.Height)
	}
}

func TestEncodeSmallFrame(t *testing.T) {
	enc, err := NewEncoder(16, 16, 30)
	if err != nil {
		t.Fatal(err)
	}
	yuv := makeYUV420(16, 16, 128)
	pkt, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode 16x16: %v", err)
	}
	if len(pkt) < 10 {
		t.Fatalf("packet too short: %d bytes", len(pkt))
	}
}

func BenchmarkEncode640x480(b *testing.B) {
	enc, _ := NewEncoder(640, 480, 30)
	yuv := makeYUV420(640, 480, 128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := enc.Encode(yuv); err != nil {
			b.Fatal(err)
		}
	}
}
