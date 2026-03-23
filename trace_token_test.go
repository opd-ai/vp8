package vp8

import (
	"fmt"
	"testing"
)

func TestTraceTokenDecode(t *testing.T) {
	width, height := 32, 16

	ySize := width * height
	uvSize := ((width + 1) / 2) * ((height + 1) / 2)
	yuv := make([]byte, ySize+2*uvSize)

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			yuv[y*width+x] = 0
		}
		for x := 16; x < 32; x++ {
			yuv[y*width+x] = 255
		}
	}
	for i := ySize; i < ySize+2*uvSize; i++ {
		yuv[i] = 128
	}

	enc, _ := NewEncoder(width, height, 30)
	frame, _ := enc.Encode(yuv)

	firstPartLen := uint32(frame[0])>>5 | uint32(frame[1])<<3 | uint32(frame[2])<<11
	secondPart := frame[10+firstPartLen:]

	fmt.Printf("Second partition bytes: %x\n\n", secondPart)

	dec := newCorrectPartition(secondPart)

	// Y2 plane=1, band=0 (for coeff 0), ctx=0
	probs := DefaultCoeffProbs[1][0][0][:]
	fmt.Printf("Y2 band0 ctx0 probs: %v\n", probs)

	// First: p[1] = zero vs non-zero (no p[0] for first coeff)
	fmt.Printf("\nInitial state: r=%d, nBits=%d, bits=%04x, rangeM1=%d\n",
		dec.r, dec.nBits, dec.bits, dec.rangeM1)

	b1 := dec.readBit(probs[1])
	fmt.Printf("p[1]=%d: %v (non-zero=%v)\n", probs[1], b1, b1)
	fmt.Printf("State: r=%d, nBits=%d, bits=%04x, rangeM1=%d\n",
		dec.r, dec.nBits, dec.bits, dec.rangeM1)

	if !b1 {
		fmt.Println("Token is ZERO (dct_0)")
	} else {
		// p[2]: 1 vs larger
		b2 := dec.readBit(probs[2])
		fmt.Printf("p[2]=%d: %v (one=%v)\n", probs[2], b2, !b2)
		fmt.Printf("State: r=%d, nBits=%d, bits=%04x, rangeM1=%d\n",
			dec.r, dec.nBits, dec.bits, dec.rangeM1)

		if !b2 {
			// Token is dct_1
			sign := dec.readBit(128)
			fmt.Printf("Token = 1, sign=%v, value=%d\n", sign, map[bool]int{true: -1, false: 1}[sign])
		} else {
			// p[3]: 2 vs larger
			b3 := dec.readBit(probs[3])
			fmt.Printf("p[3]=%d: %v (two=%v)\n", probs[3], b3, !b3)
			fmt.Printf("State: r=%d, nBits=%d, bits=%04x, rangeM1=%d\n",
				dec.r, dec.nBits, dec.bits, dec.rangeM1)

			if !b3 {
				// Token is dct_2
				sign := dec.readBit(128)
				fmt.Printf("Token = 2, sign=%v, value=%d\n", sign, map[bool]int{true: -2, false: 2}[sign])
			} else {
				// p[4]: 3-4 vs larger (5-6 or cat)
				b4 := dec.readBit(probs[4])
				fmt.Printf("p[4]=%d: %v (dct_3_4=%v)\n", probs[4], b4, !b4)
				fmt.Printf("State: r=%d, nBits=%d, bits=%04x, rangeM1=%d\n",
					dec.r, dec.nBits, dec.bits, dec.rangeM1)

				if !b4 {
					// p[5]: 3 vs 4
					b5 := dec.readBit(probs[5])
					fmt.Printf("p[5]=%d: %v (three=%v, four=%v)\n", probs[5], b5, !b5, b5)
					v := 3
					if b5 {
						v = 4
					}
					sign := dec.readBit(128)
					val := v
					if sign {
						val = -v
					}
					fmt.Printf("Token = %d, sign=%v, value=%d\n", v, sign, val)
				} else {
					// p[6]: 5-6 vs cat
					b6 := dec.readBit(probs[6])
					fmt.Printf("p[6]=%d: %v (dct_5_6=%v, cat=%v)\n", probs[6], b6, !b6, b6)
					fmt.Printf("State: r=%d, nBits=%d, bits=%04x, rangeM1=%d\n",
						dec.r, dec.nBits, dec.bits, dec.rangeM1)

					if !b6 {
						// p[7]: 5 vs 6
						b7 := dec.readBit(probs[7])
						v := 5
						if b7 {
							v = 6
						}
						sign := dec.readBit(128)
						val := v
						if sign {
							val = -v
						}
						fmt.Printf("Token = %d, sign=%v, value=%d\n", v, sign, val)
					} else {
						// p[8]: cat1-2 vs cat3+
						b8 := dec.readBit(probs[8])
						fmt.Printf("p[8]=%d: %v (cat1-2=%v, cat3+=%v)\n", probs[8], b8, !b8, b8)
						fmt.Printf("State: r=%d, nBits=%d, bits=%04x, rangeM1=%d\n",
							dec.r, dec.nBits, dec.bits, dec.rangeM1)

						if !b8 {
							// p[9]: cat1 vs cat2
							b9 := dec.readBit(probs[9])
							fmt.Printf("p[9]=%d: %v (cat1=%v, cat2=%v)\n", probs[9], b9, !b9, b9)

							if !b9 {
								// cat1: 7 + 1 extra bit
								extra := dec.readBit(catProbs[0][0])
								v := 7
								if extra {
									v = 8
								}
								sign := dec.readBit(128)
								val := v
								if sign {
									val = -v
								}
								fmt.Printf("cat1: extra=%v, Token = %d, sign=%v, value=%d\n", extra, v, sign, val)
							} else {
								// cat2: 9 + 2 extra bits
								e0 := dec.readBit(catProbs[1][0])
								e1 := dec.readBit(catProbs[1][1])
								extra := 0
								if e0 {
									extra |= 2
								}
								if e1 {
									extra |= 1
								}
								v := 9 + extra
								sign := dec.readBit(128)
								val := v
								if sign {
									val = -v
								}
								fmt.Printf("cat2: extra bits=%d%d=%d, Token = %d, sign=%v, value=%d\n",
									map[bool]int{true: 1, false: 0}[e0],
									map[bool]int{true: 1, false: 0}[e1],
									extra, v, sign, val)
							}
						} else {
							// p[10]: cat3-4 vs cat5-6
							b10 := dec.readBit(probs[10])
							fmt.Printf("p[10]=%d: %v (cat3-4=%v, cat5-6=%v)\n", probs[10], b10, !b10, b10)
							fmt.Printf("State: r=%d, nBits=%d, bits=%04x, rangeM1=%d\n",
								dec.r, dec.nBits, dec.bits, dec.rangeM1)

							if !b10 {
								// cat3 or cat4
								// There's another prob to split these
								bc := dec.readBit(180) // This prob needs verification
								fmt.Printf("cat3/cat4 split (180): %v\n", bc)

								if !bc {
									// cat3: 13-20 (3 extra bits)
									extra := 0
									for i, p := range catProbs[2] {
										b := dec.readBit(p)
										fmt.Printf("  cat3 extra[%d] p=%d: %v\n", i, p, b)
										extra <<= 1
										if b {
											extra |= 1
										}
									}
									v := 13 + extra
									sign := dec.readBit(128)
									val := v
									if sign {
										val = -v
									}
									fmt.Printf("cat3: extra=%d, Token = %d, sign=%v, value=%d\n", extra, v, sign, val)
								} else {
									// cat4: 21-36 (4 extra bits)
									extra := 0
									for i, p := range catProbs[3] {
										b := dec.readBit(p)
										fmt.Printf("  cat4 extra[%d] p=%d: %v\n", i, p, b)
										extra <<= 1
										if b {
											extra |= 1
										}
									}
									v := 25 + extra
									sign := dec.readBit(128)
									val := v
									if sign {
										val = -v
									}
									fmt.Printf("cat4: extra=%d, Token = %d, sign=%v, value=%d\n", extra, v, sign, val)
								}
							} else {
								// cat5 or cat6
								bc := dec.readBit(254) // This prob needs verification
								fmt.Printf("cat5/cat6 split (254): %v\n", bc)

								if !bc {
									// cat5: 37-68 (5 extra bits)
									extra := 0
									for i, p := range catProbs[4] {
										b := dec.readBit(p)
										fmt.Printf("  cat5 extra[%d] p=%d: %v\n", i, p, b)
										extra <<= 1
										if b {
											extra |= 1
										}
									}
									v := 35 + extra
									sign := dec.readBit(128)
									val := v
									if sign {
										val = -v
									}
									fmt.Printf("cat5: extra=%d, Token = %d, sign=%v, value=%d\n", extra, v, sign, val)
								} else {
									// cat6: 67+ (11 extra bits)
									extra := 0
									for i, p := range catProbs[5] {
										b := dec.readBit(p)
										fmt.Printf("  cat6 extra[%d] p=%d: %v\n", i, p, b)
										extra <<= 1
										if b {
											extra |= 1
										}
									}
									v := 67 + extra
									sign := dec.readBit(128)
									val := v
									if sign {
										val = -v
									}
									fmt.Printf("cat6: extra=%d, Token = %d, sign=%v, value=%d\n", extra, v, sign, val)
								}
							}
						}
					}
				}
			}
		}
	}

	fmt.Printf("\nExpected: -177 for MB0 Y2[0]\n")
}
