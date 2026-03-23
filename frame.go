package vp8

// FrameType identifies whether a VP8 frame is a key frame or inter frame.
type FrameType uint8

const (
	// KeyFrame is a VP8 intra-coded frame (I-frame).
	KeyFrame FrameType = iota
	// InterFrame is a VP8 inter-coded frame (P-frame).
	// Not supported in this implementation.
	InterFrame
)

// Frame holds raw YUV420 video data for encoding.
type Frame struct {
	// Y is the luma plane. Length must be Width * Height.
	Y []byte
	// Cb is the Cb chroma plane. Length must be (Width/2) * (Height/2).
	Cb []byte
	// Cr is the Cr chroma plane. Length must be (Width/2) * (Height/2).
	Cr     []byte
	Width  int
	Height int
	Type   FrameType
}

// NewYUV420Frame creates a Frame from a packed YUV420 (I420) byte slice.
// The expected layout is: Y plane, then Cb plane, then Cr plane.
// Width and height must be positive even integers.
func NewYUV420Frame(yuv []byte, width, height int) (*Frame, error) {
	if width <= 0 || height <= 0 || width%2 != 0 || height%2 != 0 {
		return nil, errInvalidDimensions
	}
	ySize := width * height
	uvSize := (width / 2) * (height / 2)
	if len(yuv) < ySize+2*uvSize {
		return nil, errInvalidFrameSize
	}
	return &Frame{
		Y:      yuv[:ySize],
		Cb:     yuv[ySize : ySize+uvSize],
		Cr:     yuv[ySize+uvSize : ySize+2*uvSize],
		Width:  width,
		Height: height,
		Type:   KeyFrame,
	}, nil
}
