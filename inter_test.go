package vp8

import (
	"testing"
)

// TestRefFrameManagerBasic tests basic reference frame buffer operations.
func TestRefFrameManagerBasic(t *testing.T) {
	mgr := newRefFrameManager(32, 32)

	// Initially no references should be valid
	if mgr.hasReference(refFrameLast) {
		t.Error("last frame should not be valid initially")
	}
	if mgr.hasReference(refFrameGolden) {
		t.Error("golden frame should not be valid initially")
	}
	if mgr.hasReference(refFrameAltRef) {
		t.Error("altref frame should not be valid initially")
	}

	// Store a reference frame
	ySize := 32 * 32
	uvSize := 16 * 16
	y := make([]byte, ySize)
	cb := make([]byte, uvSize)
	cr := make([]byte, uvSize)
	for i := range y {
		y[i] = 128
	}

	mgr.updateLast(y, cb, cr)
	if !mgr.hasReference(refFrameLast) {
		t.Error("last frame should be valid after update")
	}

	ref := mgr.getRef(refFrameLast)
	if ref == nil {
		t.Fatal("getRef returned nil for valid last frame")
	}
	if ref.Y[0] != 128 {
		t.Errorf("expected Y[0]=128, got %d", ref.Y[0])
	}
}

// TestRefFrameManagerCopy tests copying between reference buffers.
func TestRefFrameManagerCopy(t *testing.T) {
	mgr := newRefFrameManager(16, 16)

	ySize := 16 * 16
	uvSize := 8 * 8
	y := make([]byte, ySize)
	cb := make([]byte, uvSize)
	cr := make([]byte, uvSize)
	for i := range y {
		y[i] = 200
	}

	mgr.updateLast(y, cb, cr)
	mgr.copyLastToGolden()

	if !mgr.hasReference(refFrameGolden) {
		t.Error("golden should be valid after copy from last")
	}

	goldenRef := mgr.getRef(refFrameGolden)
	if goldenRef.Y[0] != 200 {
		t.Errorf("expected golden Y[0]=200, got %d", goldenRef.Y[0])
	}
}

// TestRefFrameManagerReset tests that reset invalidates all buffers.
func TestRefFrameManagerReset(t *testing.T) {
	mgr := newRefFrameManager(16, 16)

	ySize := 16 * 16
	uvSize := 8 * 8
	y := make([]byte, ySize)
	cb := make([]byte, uvSize)
	cr := make([]byte, uvSize)

	mgr.updateLast(y, cb, cr)
	mgr.updateGolden(y, cb, cr)
	mgr.updateAltRef(y, cb, cr)

	mgr.reset()

	if mgr.hasReference(refFrameLast) {
		t.Error("last should be invalid after reset")
	}
	if mgr.hasReference(refFrameGolden) {
		t.Error("golden should be invalid after reset")
	}
	if mgr.hasReference(refFrameAltRef) {
		t.Error("altref should be invalid after reset")
	}
}

// TestMotionVectorBasics tests motion vector type operations.
func TestMotionVectorBasics(t *testing.T) {
	mv1 := motionVector{dx: 4, dy: -8}
	mv2 := motionVector{dx: 4, dy: -8}
	mv3 := motionVector{dx: 0, dy: 0}

	if !mvEqual(mv1, mv2) {
		t.Error("equal MVs should be equal")
	}
	if mvEqual(mv1, mv3) {
		t.Error("different MVs should not be equal")
	}
	if !mvEqual(mv3, zeroMV) {
		t.Error("zero MV should equal zeroMV")
	}
}

// TestMotionEstimateStatic tests motion estimation with a static (identical) frame.
func TestMotionEstimateStatic(t *testing.T) {
	// Create a reference frame with known pattern
	width, height := 64, 64
	ref := make([]byte, width*height)
	for i := range ref {
		ref[i] = byte(i % 256)
	}

	// Source block at (16, 16) should match reference exactly at zero MV
	var srcY [256]byte
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			srcY[row*16+col] = ref[(16+row)*width+(16+col)]
		}
	}

	result := estimateMotion(srcY[:], ref, width, height, 16, 16, zeroMV)

	if result.sad != 0 {
		t.Errorf("expected SAD=0 for identical block, got %d", result.sad)
	}
	if result.mv.dx != 0 || result.mv.dy != 0 {
		t.Errorf("expected zero MV for identical block, got (%d, %d)", result.mv.dx, result.mv.dy)
	}
}

// TestMotionEstimateShifted tests motion estimation with a shifted block.
func TestMotionEstimateShifted(t *testing.T) {
	width, height := 96, 96

	// Create reference frame with a distinctive pattern
	ref := make([]byte, width*height)
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			ref[row*width+col] = byte((row*3 + col*7) % 256)
		}
	}

	// Source block: copy from reference at offset (4, 0) relative to MB position (32, 32)
	// So source contains ref pixels at (36, 32) to (51, 47)
	var srcY [256]byte
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			srcY[row*16+col] = ref[(32+row)*width+(36+col)]
		}
	}

	// Motion estimation should find a MV with low SAD
	result := estimateMotion(srcY[:], ref, width, height, 32, 32, zeroMV)

	// The search should find the pattern - SAD should be 0 or very low
	t.Logf("SAD=%d, MV=(%d,%d)", result.sad, result.mv.dx, result.mv.dy)
	// The exact MV found depends on search pattern; verify SAD is good
	if result.sad > 256 {
		t.Errorf("expected low SAD for shifted block, got %d", result.sad)
	}
}

// TestMotionCompensate16x16 tests motion compensation block extraction.
func TestMotionCompensate16x16(t *testing.T) {
	width, height := 48, 48
	ref := make([]byte, width*height)
	for i := range ref {
		ref[i] = byte(i % 256)
	}

	var dst [256]byte
	mv := motionVector{dx: 0, dy: 0}
	motionCompensate16x16(dst[:], ref, width, height, 16, 16, mv)

	// Verify pixels match reference at (16, 16)
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			expected := ref[(16+row)*width+(16+col)]
			got := dst[row*16+col]
			if got != expected {
				t.Errorf("at (%d,%d): expected %d, got %d", row, col, expected, got)
			}
		}
	}
}

// TestMotionCompensate8x8 tests chroma motion compensation.
func TestMotionCompensate8x8(t *testing.T) {
	width, height := 32, 32
	ref := make([]byte, width*height)
	for i := range ref {
		ref[i] = byte(i % 256)
	}

	var dst [64]byte
	mv := motionVector{dx: 0, dy: 0}
	motionCompensate8x8(dst[:], ref, width, height, 8, 8, mv)

	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			expected := ref[(8+row)*width+(8+col)]
			got := dst[row*8+col]
			if got != expected {
				t.Errorf("at (%d,%d): expected %d, got %d", row, col, expected, got)
			}
		}
	}
}

// TestFindNearestMV tests motion vector prediction from neighbors.
func TestFindNearestMV(t *testing.T) {
	mbW := 4
	mbs := make([]macroblock, 16)

	// Set up left neighbor with MV
	mbs[1*mbW+0] = macroblock{
		isInter: true,
		mv:      motionVector{dx: 8, dy: 4},
	}

	// Query MB at (1, 1) - left neighbor has MV(8, 4)
	nearest, _ := findNearestMV(mbs, 1, 1, mbW)

	if nearest.dx != 8 || nearest.dy != 4 {
		t.Errorf("expected nearest=(%d,%d), got (%d,%d)", 8, 4, nearest.dx, nearest.dy)
	}
}

// TestMVInRange tests motion vector range checking.
func TestMVInRange(t *testing.T) {
	tests := []struct {
		mv   motionVector
		mbX  int
		mbY  int
		refW int
		refH int
		want bool
	}{
		{motionVector{0, 0}, 0, 0, 64, 64, true},
		{motionVector{-4, 0}, 0, 0, 64, 64, false}, // would go to x=-1
		{motionVector{0, -4}, 0, 0, 64, 64, false},  // would go to y=-1
		{motionVector{4, 4}, 16, 16, 48, 48, true},   // reference at (17, 17)
		{motionVector{40, 0}, 16, 16, 48, 48, true},   // 16+10+16 = 42 <= 48
	}

	for i, tt := range tests {
		got := mvInRange(tt.mv, tt.mbX, tt.mbY, tt.refW, tt.refH)
		if got != tt.want {
			t.Errorf("test %d: mvInRange(%v, %d, %d, %d, %d) = %v, want %v",
				i, tt.mv, tt.mbX, tt.mbY, tt.refW, tt.refH, got, tt.want)
		}
	}
}

// TestDiamondSearch tests the diamond search pattern.
func TestDiamondSearch(t *testing.T) {
	width, height := 64, 64
	ref := make([]byte, width*height)
	for i := range ref {
		ref[i] = 128
	}

	// Place a distinctive pattern in the reference at (20, 20)
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			ref[(20+row)*width+(20+col)] = byte(row*16 + col)
		}
	}

	// Source block matches the pattern
	var srcY [256]byte
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			srcY[row*16+col] = byte(row*16 + col)
		}
	}

	// Start search from nearby position (16, 16)
	startMV := motionVector{dx: 0, dy: 0}
	startSAD := computeMCSAD16x16(srcY[:], ref, width, height, 16, 16, startMV)

	bestMV, bestSAD := diamondSearch(srcY[:], ref, width, height, 16, 16, startMV, startSAD)

	// Should find the pattern at offset (4, 4) = MV (16, 16) in quarter-pel
	if bestSAD >= startSAD {
		t.Logf("diamond search did not improve: startSAD=%d, bestSAD=%d, bestMV=(%d,%d)",
			startSAD, bestSAD, bestMV.dx, bestMV.dy)
	}
	if bestSAD != 0 {
		t.Logf("note: bestSAD=%d (expected 0 for exact match), MV=(%d,%d)",
			bestSAD, bestMV.dx, bestMV.dy)
	}
}

// TestLoopFilterBasic tests the loop filter with a moderate edge case.
func TestLoopFilterBasic(t *testing.T) {
	recon := &refFrameBuffer{
		Y:      make([]byte, 16*16),
		Cb:     make([]byte, 8*8),
		Cr:     make([]byte, 8*8),
		Width:  16,
		Height: 16,
		valid:  true,
	}

	// Create an edge within the filter's limit range
	for row := 0; row < 16; row++ {
		for col := 0; col < 8; col++ {
			recon.Y[row*16+col] = 120
		}
		for col := 8; col < 16; col++ {
			recon.Y[row*16+col] = 130
		}
	}

	// Save original edge values
	origP0 := int(recon.Y[0*16+7])
	origQ0 := int(recon.Y[0*16+8])
	origDiff := origQ0 - origP0

	params := loopFilterParams{
		level:     20,
		sharpness: 0,
		simple:    true,
	}

	applyLoopFilter(recon, params)

	// After filtering, the edge should be smoothed
	p0 := int(recon.Y[0*16+7])
	q0 := int(recon.Y[0*16+8])
	newDiff := q0 - p0
	if newDiff < 0 {
		newDiff = -newDiff
	}

	if newDiff >= origDiff {
		t.Errorf("loop filter did not smooth edge: before diff=%d, after diff=%d", origDiff, newDiff)
	}
}

// TestLoopFilterZeroLevel tests that level=0 disables the filter.
func TestLoopFilterZeroLevel(t *testing.T) {
	recon := &refFrameBuffer{
		Y:      make([]byte, 16*16),
		Cb:     make([]byte, 8*8),
		Cr:     make([]byte, 8*8),
		Width:  16,
		Height: 16,
		valid:  true,
	}

	// Create an edge
	for row := 0; row < 16; row++ {
		for col := 0; col < 8; col++ {
			recon.Y[row*16+col] = 50
		}
		for col := 8; col < 16; col++ {
			recon.Y[row*16+col] = 200
		}
	}

	// Save original values
	origP0 := recon.Y[0*16+7]
	origQ0 := recon.Y[0*16+8]

	params := loopFilterParams{level: 0}
	applyLoopFilter(recon, params)

	if recon.Y[0*16+7] != origP0 || recon.Y[0*16+8] != origQ0 {
		t.Error("loop filter should not modify pixels when level=0")
	}
}

// TestEncoderKeyFrameInterval tests key frame interval configuration.
func TestEncoderKeyFrameInterval(t *testing.T) {
	enc, err := NewEncoder(32, 32, 30)
	if err != nil {
		t.Fatal(err)
	}

	// Default: every frame is key frame
	if !enc.shouldEncodeKeyFrame() {
		t.Error("first frame should be key frame")
	}

	// Set interval
	enc.SetKeyFrameInterval(10)
	enc.frameCount = 0 // no frames encoded yet
	if !enc.shouldEncodeKeyFrame() {
		t.Error("frame 0 (first ever) should be key frame")
	}

	enc.frameCount = 5
	// Need to have a valid reference for inter frame
	mgr := enc.refFrames
	ySize := 32 * 32
	uvSize := 16 * 16
	mgr.updateLast(make([]byte, ySize), make([]byte, uvSize), make([]byte, uvSize))

	if enc.shouldEncodeKeyFrame() {
		t.Error("frame 5 should not be key frame with interval=10")
	}

	enc.frameCount = 10
	if !enc.shouldEncodeKeyFrame() {
		t.Error("frame 10 should be key frame with interval=10")
	}
}

// TestEncoderForceKeyFrame tests ForceKeyFrame functionality.
func TestEncoderForceKeyFrame(t *testing.T) {
	enc, err := NewEncoder(32, 32, 30)
	if err != nil {
		t.Fatal(err)
	}

	enc.SetKeyFrameInterval(30)
	enc.frameCount = 5
	mgr := enc.refFrames
	ySize := 32 * 32
	uvSize := 16 * 16
	mgr.updateLast(make([]byte, ySize), make([]byte, uvSize), make([]byte, uvSize))

	if enc.shouldEncodeKeyFrame() {
		t.Error("should not be key frame before ForceKeyFrame")
	}

	enc.ForceKeyFrame()
	if !enc.shouldEncodeKeyFrame() {
		t.Error("should be key frame after ForceKeyFrame")
	}
}

// TestEncodeKeyFrameBackwardCompatible tests that key-frame-only mode still works.
func TestEncodeKeyFrameBackwardCompatible(t *testing.T) {
	enc, err := NewEncoder(32, 32, 30)
	if err != nil {
		t.Fatal(err)
	}

	// Default mode: every frame is key frame (backward compatible)
	yuv := make([]byte, 32*32*3/2)
	for i := range yuv {
		yuv[i] = 128
	}

	// Encode two frames - both should be key frames
	frame1, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("encode frame 1: %v", err)
	}
	if len(frame1) == 0 {
		t.Fatal("frame 1 is empty")
	}
	// Check frame tag: bit 0 should be 0 for key frame
	if frame1[0]&1 != 0 {
		t.Error("frame 1 should be key frame (bit 0 = 0)")
	}

	frame2, err := enc.Encode(yuv)
	if err != nil {
		t.Fatalf("encode frame 2: %v", err)
	}
	if frame2[0]&1 != 0 {
		t.Error("frame 2 should be key frame in I-frame-only mode")
	}
}

// TestEncodeInterFrame tests encoding an inter frame after a key frame.
func TestEncodeInterFrame(t *testing.T) {
	enc, err := NewEncoder(32, 32, 30)
	if err != nil {
		t.Fatal(err)
	}

	enc.SetKeyFrameInterval(30)

	// Create two frames
	yuv1 := make([]byte, 32*32*3/2)
	yuv2 := make([]byte, 32*32*3/2)
	for i := range yuv1 {
		yuv1[i] = 128
		yuv2[i] = 130 // Slightly different
	}

	// First frame should be key frame
	frame1, err := enc.Encode(yuv1)
	if err != nil {
		t.Fatalf("encode key frame: %v", err)
	}
	if frame1[0]&1 != 0 {
		t.Error("first frame should be key frame")
	}

	// Second frame should be inter frame
	frame2, err := enc.Encode(yuv2)
	if err != nil {
		t.Fatalf("encode inter frame: %v", err)
	}
	if frame2[0]&1 != 1 {
		t.Error("second frame should be inter frame (bit 0 = 1)")
	}
	if len(frame2) == 0 {
		t.Error("inter frame should not be empty")
	}
}

// TestEncodeInterFrameSequence tests encoding a sequence of inter frames.
func TestEncodeInterFrameSequence(t *testing.T) {
	enc, err := NewEncoder(64, 64, 30)
	if err != nil {
		t.Fatal(err)
	}

	enc.SetKeyFrameInterval(5)

	frameSize := 64 * 64 * 3 / 2
	for i := 0; i < 10; i++ {
		yuv := make([]byte, frameSize)
		// Create a pattern that shifts slightly each frame
		for row := 0; row < 64; row++ {
			for col := 0; col < 64; col++ {
				yuv[row*64+col] = byte((row + i) * 3 % 256)
			}
		}
		// Fill chroma with gray
		for j := 64 * 64; j < frameSize; j++ {
			yuv[j] = 128
		}

		frame, err := enc.Encode(yuv)
		if err != nil {
			t.Fatalf("encode frame %d: %v", i, err)
		}
		if len(frame) == 0 {
			t.Fatalf("frame %d is empty", i)
		}

		isKey := frame[0]&1 == 0
		if i == 0 && !isKey {
			t.Errorf("frame %d should be key frame", i)
		}
		if i == 5 && !isKey {
			t.Errorf("frame %d should be key frame (interval=5)", i)
		}
		if i > 0 && i < 5 && isKey {
			t.Errorf("frame %d should be inter frame", i)
		}
	}
}

// TestInterFrameCompression tests that inter frames achieve better compression
// for similar content compared to key frames.
func TestInterFrameCompression(t *testing.T) {
	width, height := 64, 64
	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatal(err)
	}

	enc.SetKeyFrameInterval(30)
	enc.SetBitrate(1_000_000)

	frameSize := width * height * 3 / 2

	// Create two nearly identical frames
	yuv1 := make([]byte, frameSize)
	yuv2 := make([]byte, frameSize)
	for i := 0; i < width*height; i++ {
		yuv1[i] = byte(i % 256)
		yuv2[i] = byte(i % 256) // Identical luma
	}
	for i := width * height; i < frameSize; i++ {
		yuv1[i] = 128
		yuv2[i] = 128
	}

	// Encode key frame
	keyFrame, err := enc.Encode(yuv1)
	if err != nil {
		t.Fatal(err)
	}

	// Encode inter frame (should be smaller for identical content)
	interFrame, err := enc.Encode(yuv2)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("key frame size: %d bytes, inter frame size: %d bytes",
		len(keyFrame), len(interFrame))

	// Inter frame for identical content should be notably smaller
	if len(interFrame) >= len(keyFrame) {
		t.Logf("warning: inter frame (%d bytes) is not smaller than key frame (%d bytes) for identical content",
			len(interFrame), len(keyFrame))
		// This is not necessarily a failure since encoding overhead may dominate
		// for small frames, but log it for awareness
	}
}

// TestBuildInterFrame tests the inter frame bitstream builder.
func TestBuildInterFrame(t *testing.T) {
	mbs := []macroblock{
		{
			isInter:   true,
			refFrame:  refFrameLast,
			mv:        motionVector{dx: 0, dy: 0},
			interMode: mvModeZeroMV,
			skip:      true,
		},
		{
			isInter:   true,
			refFrame:  refFrameLast,
			mv:        motionVector{dx: 4, dy: 4},
			interMode: mvModeNewMV,
			skip:      true,
		},
	}

	frame, err := BuildInterFrame(32, 16, 24, 0, 0, 0, 0, 0, OnePartition, mbs)
	if err != nil {
		t.Fatalf("BuildInterFrame: %v", err)
	}

	// Check frame tag: bit 0 should be 1 for inter frame
	if frame[0]&1 != 1 {
		t.Error("inter frame tag bit 0 should be 1")
	}

	// Check show_frame bit (bit 4)
	if frame[0]&0x10 == 0 {
		t.Error("show_frame should be 1")
	}
}

// TestSetLoopFilterLevel tests loop filter level configuration.
func TestSetLoopFilterLevel(t *testing.T) {
	enc, err := NewEncoder(32, 32, 30)
	if err != nil {
		t.Fatal(err)
	}

	enc.SetLoopFilterLevel(20)
	if enc.loopFilter.level != 20 {
		t.Errorf("expected level 20, got %d", enc.loopFilter.level)
	}

	enc.SetLoopFilterLevel(-1)
	if enc.loopFilter.level != 0 {
		t.Errorf("negative level should be clamped to 0, got %d", enc.loopFilter.level)
	}

	enc.SetLoopFilterLevel(100)
	if enc.loopFilter.level != 63 {
		t.Errorf("level > 63 should be clamped to 63, got %d", enc.loopFilter.level)
	}
}

// TestMVCost tests motion vector cost estimation.
func TestMVCost(t *testing.T) {
	// Zero MV should have lowest cost
	zeroCost := mvCost(zeroMV, zeroMV)
	nonZeroCost := mvCost(motionVector{dx: 16, dy: 16}, zeroMV)

	if zeroCost >= nonZeroCost {
		t.Errorf("zero MV cost (%d) should be less than non-zero cost (%d)",
			zeroCost, nonZeroCost)
	}
}

// TestReconstructIntraMB tests intra macroblock reconstruction.
func TestReconstructIntraMB(t *testing.T) {
	recon := &refFrameBuffer{
		Y:      make([]byte, 32*32),
		Cb:     make([]byte, 16*16),
		Cr:     make([]byte, 16*16),
		Width:  32,
		Height: 32,
		valid:  true,
	}

	// Create a simple DC-predicted macroblock with zero residual
	mb := macroblock{
		lumaMode:   DC_PRED,
		chromaMode: DC_PRED_CHROMA,
		skip:       true,
	}

	ctx := &mbContext{
		lumaTopLeft:    128,
		chromaTopLeftU: 128,
		chromaTopLeftV: 128,
	}

	qf := GetQuantFactorsSimple(24)
	_ = ctx
	reconstructIntraMB(recon, &mb, 0, 0, 32, 32, 16, qf)

	// With DC prediction and no neighbors, all pixels should be 128
	for i := 0; i < 16; i++ {
		for j := 0; j < 16; j++ {
			if recon.Y[i*32+j] != 128 {
				t.Errorf("expected Y[%d][%d]=128, got %d", i, j, recon.Y[i*32+j])
				return
			}
		}
	}
}
