package vp8

import (
	"bytes"
	"math"
	"testing"

	"golang.org/x/image/vp8"
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
		name      string
		w, h, fps int
		wantErr   bool
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
	// Too small buffer
	_, err := NewYUV420Frame(make([]byte, 10), 640, 480)
	if err == nil {
		t.Error("expected error for undersized buffer")
	}
	// Zero dimensions
	_, err = NewYUV420Frame(make([]byte, 100), 0, 480)
	if err == nil {
		t.Error("expected error for zero width")
	}
	// Odd dimensions
	_, err = NewYUV420Frame(make([]byte, 641*480*3/2), 641, 480)
	if err == nil {
		t.Error("expected error for odd width")
	}
	// Exact size, valid dimensions
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

func TestQuantIndexToQp(t *testing.T) {
	tests := []struct {
		qi      int
		wantMin int16
	}{
		{0, 4},   // lower bound → minimum step size
		{1, 4},   // just above lower bound
		{127, 1}, // upper bound → maximum step size (≥ 1)
		{24, 4},  // typical default qi → positive step size
	}
	for _, tt := range tests {
		got := quantIndexToQp(tt.qi)
		if got < tt.wantMin {
			t.Errorf("quantIndexToQp(%d) = %d, want >= %d", tt.qi, got, tt.wantMin)
		}
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

func BenchmarkEncode320x240(b *testing.B) {
	enc, _ := NewEncoder(320, 240, 30)
	yuv := makeYUV420(320, 240, 128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := enc.Encode(yuv); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode1280x720(b *testing.B) {
	enc, _ := NewEncoder(1280, 720, 30)
	yuv := makeYUV420(1280, 720, 128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := enc.Encode(yuv); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode1920x1080(b *testing.B) {
	enc, _ := NewEncoder(1920, 1080, 30)
	yuv := makeYUV420(1920, 1080, 128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := enc.Encode(yuv); err != nil {
			b.Fatal(err)
		}
	}
}

// makeGradientYUV420 creates a synthetic gradient YUV420 frame.
// The luma plane has a horizontal gradient from left to right.
func makeGradientYUV420(width, height int) []byte {
	ySize := width * height
	uvSize := (width / 2) * (height / 2)
	buf := make([]byte, ySize+2*uvSize)
	// Luma: horizontal gradient 16-235 (studio swing)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := 16 + int(float64(x)/float64(width-1)*219)
			buf[y*width+x] = byte(val)
		}
	}
	// Chroma: neutral gray (128)
	for i := ySize; i < len(buf); i++ {
		buf[i] = 128
	}
	return buf
}

// TestDecodeVerification encodes a frame and verifies it decodes correctly
// with golang.org/x/image/vp8, validating WebRTC compatibility.
func TestDecodeVerification(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		makeYUV func(w, h int) []byte
		minPSNR float64 // minimum expected PSNR in dB
	}{
		{
			name:    "solid gray 64x64",
			width:   64,
			height:  64,
			makeYUV: func(w, h int) []byte { return makeYUV420(w, h, 128) },
			minPSNR: 20.0,
		},
		{
			name:    "gradient 64x64",
			width:   64,
			height:  64,
			makeYUV: makeGradientYUV420,
			minPSNR: 15.0, // gradients have more quantization error
		},
		{
			name:    "solid gray 320x240",
			width:   320,
			height:  240,
			makeYUV: func(w, h int) []byte { return makeYUV420(w, h, 128) },
			minPSNR: 20.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := NewEncoder(tt.width, tt.height, 30)
			if err != nil {
				t.Fatalf("NewEncoder: %v", err)
			}

			// Create source YUV frame
			srcYUV := tt.makeYUV(tt.width, tt.height)

			// Encode
			vp8Data, err := enc.Encode(srcYUV)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}

			// Decode with golang.org/x/image/vp8
			dec := vp8.NewDecoder()
			dec.Init(bytes.NewReader(vp8Data), len(vp8Data))

			fh, err := dec.DecodeFrameHeader()
			if err != nil {
				t.Fatalf("DecodeFrameHeader: %v", err)
			}

			// Verify dimensions match
			if fh.Width != tt.width || fh.Height != tt.height {
				t.Errorf("decoded dimensions %dx%d, want %dx%d",
					fh.Width, fh.Height, tt.width, tt.height)
			}

			// Verify key frame
			if !fh.KeyFrame {
				t.Error("expected key frame")
			}

			// Decode the frame
			decoded, err := dec.DecodeFrame()
			if err != nil {
				t.Fatalf("DecodeFrame: %v", err)
			}

			// Calculate PSNR between source and decoded luma
			psnr := calculatePSNR(srcYUV[:tt.width*tt.height], decoded.Y, tt.width, tt.height, decoded.YStride)
			t.Logf("PSNR: %.2f dB (min: %.2f)", psnr, tt.minPSNR)

			if psnr < tt.minPSNR {
				t.Errorf("PSNR %.2f dB below threshold %.2f dB", psnr, tt.minPSNR)
			}
		})
	}
}

// calculatePSNR computes Peak Signal-to-Noise Ratio between two images.
func calculatePSNR(src, dst []byte, width, height, dstStride int) float64 {
	var mse float64
	count := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcVal := float64(src[y*width+x])
			dstVal := float64(dst[y*dstStride+x])
			diff := srcVal - dstVal
			mse += diff * diff
			count++
		}
	}
	mse /= float64(count)
	if mse == 0 {
		return 100.0 // perfect match
	}
	return 10.0 * math.Log10(255.0*255.0/mse)
}

// TestSetQuantizerDeltas verifies that quantizer deltas are correctly
// stored in the encoder and that frames with deltas decode correctly.
func TestSetQuantizerDeltas(t *testing.T) {
	enc, err := NewEncoder(64, 64, 30)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	// Set non-zero deltas
	enc.SetQuantizerDeltas(2, -3, 4, -2, 1)

	// Verify deltas are stored
	if enc.y1DCDelta != 2 {
		t.Errorf("y1DCDelta = %d, want 2", enc.y1DCDelta)
	}
	if enc.y2DCDelta != -3 {
		t.Errorf("y2DCDelta = %d, want -3", enc.y2DCDelta)
	}
	if enc.y2ACDelta != 4 {
		t.Errorf("y2ACDelta = %d, want 4", enc.y2ACDelta)
	}
	if enc.uvDCDelta != -2 {
		t.Errorf("uvDCDelta = %d, want -2", enc.uvDCDelta)
	}
	if enc.uvACDelta != 1 {
		t.Errorf("uvACDelta = %d, want 1", enc.uvACDelta)
	}

	// Encode a frame with deltas
	yuv := makeYUV420(64, 64, 128)
	vp8Data, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Verify the frame decodes correctly
	dec := vp8.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))

	fh, err := dec.DecodeFrameHeader()
	if err != nil {
		t.Fatalf("DecodeFrameHeader: %v", err)
	}
	if fh.Width != 64 || fh.Height != 64 {
		t.Errorf("decoded dimensions %dx%d, want 64x64", fh.Width, fh.Height)
	}

	// Frame should decode without error
	_, err = dec.DecodeFrame()
	if err != nil {
		t.Fatalf("DecodeFrame with deltas: %v", err)
	}
}

// TestMultiPartitionEncode verifies that frames with multiple partitions
// decode correctly with golang.org/x/image/vp8.
func TestMultiPartitionEncode(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		partitions PartitionCount
	}{
		{"single partition 64x64", 64, 64, OnePartition},
		{"two partitions 64x64", 64, 64, TwoPartitions},
		{"four partitions 64x64", 64, 64, FourPartitions},
		{"eight partitions 128x128", 128, 128, EightPartitions},
		{"two partitions 320x240", 320, 240, TwoPartitions},
		{"four partitions 320x240", 320, 240, FourPartitions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := NewEncoder(tt.width, tt.height, 30)
			if err != nil {
				t.Fatalf("NewEncoder: %v", err)
			}

			// Set partition count
			enc.SetPartitionCount(tt.partitions)

			// Create source YUV frame
			srcYUV := makeYUV420(tt.width, tt.height, 128)

			// Encode
			vp8Data, err := enc.Encode(srcYUV)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}

			// Verify key frame marker
			if vp8Data[0]&1 != 0 {
				t.Errorf("expected key frame (bit0=0), got 0x%02x", vp8Data[0])
			}

			// Decode with golang.org/x/image/vp8
			dec := vp8.NewDecoder()
			dec.Init(bytes.NewReader(vp8Data), len(vp8Data))

			fh, err := dec.DecodeFrameHeader()
			if err != nil {
				t.Fatalf("DecodeFrameHeader: %v", err)
			}

			// Verify dimensions match
			if fh.Width != tt.width || fh.Height != tt.height {
				t.Errorf("decoded dimensions %dx%d, want %dx%d",
					fh.Width, fh.Height, tt.width, tt.height)
			}

			// Decode the frame
			decoded, err := dec.DecodeFrame()
			if err != nil {
				t.Fatalf("DecodeFrame: %v", err)
			}

			// Verify decoded frame is reasonable (PSNR > 20 dB for solid gray)
			psnr := calculatePSNR(srcYUV[:tt.width*tt.height], decoded.Y, tt.width, tt.height, decoded.YStride)
			if psnr < 20.0 {
				t.Errorf("PSNR %.2f dB below threshold 20.0 dB", psnr)
			}
		})
	}
}
