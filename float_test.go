package cbor

import (
	"bytes"
	"math"
	"testing"
)

//go:generate sh -c "perl scripts/float_gen.pl | gofmt > float_gen_test.go"

func TestFloat(t *testing.T) {
	tests := []struct {
		f64   float64
		bytes []byte
	}{
		// zeros
		{0.0, []byte{0x00, 0x00}},
		{math.Copysign(0, -1), []byte{0x80, 0x00}},

		// inf
		{math.Inf(1), []byte{0x7c, 0x00}},
		{math.Inf(-1), []byte{0xfc, 0x00}},

		// NaN
		{math.NaN(), []byte{0x7e, 0x00}},

		// subnormal float16
		{0x1p-24, []byte{0x00, 0x01}},     // smallest positive subnormal float16
		{0x1.ff8p-15, []byte{0x03, 0xff}}, // largest positive subnormal float16

		// normal float16
		{0x1p-14, []byte{0x04, 0x00}},     // smallest positive normal float16
		{0x1.554p-02, []byte{0x35, 0x55}}, // nearest float16 value to 1/3
		{0x1.ffcp-01, []byte{0x3b, 0xff}}, // largest float16 less than one
		{0x1p+00, []byte{0x3c, 0x00}},     // one
		{0x1.004p+00, []byte{0x3c, 0x01}}, // smallest float16 larger than one
		{0x1.ffcp+15, []byte{0x7b, 0xff}}, // largest normal float16

		// subnormal float32
		{0x1p-149, []byte{0x00, 0x00, 0x00, 0x01}},        // smallest positive subnormal float32
		{0x1.fffffcp-127, []byte{0x00, 0x7f, 0xff, 0xff}}, // largest positive subnormal float32

		// normal float32
		{0x1p-126, []byte{0x00, 0x80, 0x00, 0x00}},        // smallest positive normal float32
		{0x1.555556p-02, []byte{0x3e, 0xaa, 0xaa, 0xab}},  // nearest float32 value to 1/3
		{0x1.fffffep-01, []byte{0x3f, 0x7f, 0xff, 0xff}},  // largest float32 less than one
		{0x1.000002p+00, []byte{0x3f, 0x80, 0x00, 0x01}},  // smallest float32 larger than one
		{0x1.921fb6p+01, []byte{0x40, 0x49, 0x0f, 0xdb}},  // nearest float32 value to pi
		{0x1.fffffep+127, []byte{0x7f, 0x7f, 0xff, 0xff}}, // largest normal float32
	}
	for _, tt := range tests {
		got, err := Marshal(tt.f64)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			continue
		}
		got = got[1:] // skip major type
		if !bytes.Equal(got, tt.bytes) {
			t.Errorf("EncodeFloat64(%x) = %x, want %x", tt.f64, got, tt.bytes)
		}
	}
}

func TestFloat_Gen(t *testing.T) {
	for _, tt := range f64ToBytesTests {
		input := math.Float64frombits(tt.f64)
		if math.IsNaN(input) {
			continue
		}
		got, err := Marshal(input)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			continue
		}
		want := tt.bytes
		if math.IsNaN(input) {
			// support NaN payloads or signaling NaNs.
			// NaN is always encoded as 0x7e00
			want = []byte{0x7e, 0x00}
		}
		got = got[1:] // skip major type
		if !bytes.Equal(got, want) {
			t.Errorf("EncodeFloat64(%x) = %x, want %x", input, got, want)
		}
	}
}
