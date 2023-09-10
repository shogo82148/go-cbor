package cbor

import "math"

//go:generate sh -c "perl scripts/float_gen.pl | gofmt > float_gen_test.go"

func EncodeFloat64(f float64) []byte {
	f64 := math.Float64bits(f)
	sign := f64 >> 63
	exp := int((f64>>52)&0x7ff) - 1023
	frac := f64 & 0xfffffffffffff

	if exp == -1023 && frac == 0 {
		// 0.0 in float16
		return []byte{byte(sign << 7), 0x00}
	}
	if exp == 1024 {
		if frac == 0 {
			// inf in float16
			return []byte{byte(sign<<7 | 0x7c), 0x00}
		} else if frac&0x8000000000000 != 0 {
			// qNaN in float16
			return []byte{byte(sign<<7 | 0x7c | frac>>50), byte(frac >> 42)}
		}
	}

	// try converting to subnormal float16
	if -24 <= exp && exp < -14 {
		shift := -exp + 53 - 24 - 1
		if frac&((1<<shift)-1) == 0 {
			frac |= 1 << 52
			f16 := uint16(sign<<15 | frac>>shift)
			return []byte{byte(f16 >> 8), byte(f16)}
		}
	}

	// try converting to normal float16
	if -14 <= exp && exp <= 15 {
		if frac&((1<<42)-1) == 0 {
			f16 := uint16(sign<<15 | uint64(exp+15)<<10 | frac>>42)
			return []byte{byte(f16 >> 8), byte(f16)}
		}
	}

	// try converting to subnormal float32
	if -149 <= exp && exp < -126 {
		shift := -exp + 53 - 149 - 1
		if frac&((1<<shift)-1) == 0 {
			frac |= 1 << 52
			f32 := uint32(sign<<31 | frac>>shift)
			return []byte{byte(f32 >> 24), byte(f32 >> 16), byte(f32 >> 8), byte(f32)}
		}
	}

	// try converting to normal float32
	if -126 <= exp && exp <= 127 {
		if frac&((1<<29)-1) == 0 {
			f32 := uint32(sign<<31 | uint64(exp+127)<<23 | frac>>29)
			return []byte{byte(f32 >> 24), byte(f32 >> 16), byte(f32 >> 8), byte(f32)}
		}
	}

	return []byte{
		byte(f64 >> 56),
		byte(f64 >> 48),
		byte(f64 >> 40),
		byte(f64 >> 32),
		byte(f64 >> 24),
		byte(f64 >> 16),
		byte(f64 >> 8),
		byte(f64),
	}
}
