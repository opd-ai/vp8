package vp8

import (
	"bytes"
	"fmt"
	"testing"

	vp8dec "golang.org/x/image/vp8"
)

func makeHGradient(w, h int) []byte {
	ySize := w * h
	uvSize := (w / 2) * (h / 2)
	buf := make([]byte, ySize+2*uvSize)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			val := 16 + int(float64(x)/float64(w-1)*219)
			buf[y*w+x] = byte(val)
		}
	}
	for i := ySize; i < len(buf); i++ {
		buf[i] = 128
	}
	return buf
}

func TestTraceHGradient32(t *testing.T) {
	debugMB = true
	defer func() { debugMB = false }()

	width, height := 32, 32
	srcYUV := makeHGradient(width, height)

	// Print first MB's Y values
	fmt.Println("=== First MB Y values (first 4 rows) ===")
	for row := 0; row < 4; row++ {
		for col := 0; col < 16; col++ {
			fmt.Printf("%3d ", srcYUV[row*32+col])
		}
		fmt.Println()
	}

	enc, err := NewEncoder(width, height, 30)
	if err != nil {
		t.Fatalf("NewEncoder error: %v\n", err)
	}

	fmt.Println("\n=== Encoding horizontal gradient 32x32 ===")
	vp8Data, err := enc.Encode(srcYUV)
	if err != nil {
		t.Fatalf("Encode error: %v\n", err)
	}

	fmt.Printf("\n32x32: %d bytes\n", len(vp8Data))

	// Print data hex
	fmt.Printf("Data: ")
	for i, b := range vp8Data {
		fmt.Printf("%02x ", b)
		if (i+1)%20 == 0 {
			fmt.Println()
			fmt.Print("      ")
		}
	}
	fmt.Println()

	dec := vp8dec.NewDecoder()
	dec.Init(bytes.NewReader(vp8Data), len(vp8Data))
	_, _ = dec.DecodeFrameHeader()
	_, err = dec.DecodeFrame()
	if err != nil {
		t.Logf("DecodeFrame: %v", err)
	} else {
		t.Log("DECODE SUCCESS")
	}
}
