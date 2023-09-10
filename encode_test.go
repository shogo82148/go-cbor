package cbor

import (
	"bytes"
	"math"
	"testing"
)

func TestMarshal(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want []byte
	}{
		// RFC 8949  Appendix A. Examples of Encoded CBOR Data Items
		{
			"integer zero",
			int(0),
			[]byte{0x00},
		},
		{
			"integer one",
			int(1),
			[]byte{0x01},
		},
		{
			"integer ten",
			int(10),
			[]byte{0x0a},
		},
		{
			"integer twenty-three",
			int(23),
			[]byte{0x17},
		},
		{
			"integer twenty-four",
			int(24),
			[]byte{0x18, 0x18},
		},
		{
			"integer twenty-five",
			int(25),
			[]byte{0x18, 0x19},
		},
		{
			"integer one hundred",
			int(100),
			[]byte{0x18, 0x64},
		},
		{
			"integer one thousand",
			int(1000),
			[]byte{0x19, 0x03, 0xe8},
		},
		{
			"integer one million",
			int(1000_000),
			[]byte{0x1a, 0x00, 0x0f, 0x42, 0x40},
		},
		{
			"1_000_000_000_000",
			int(1_000_000_000_000),
			[]byte{0x1b, 0x00, 0x00, 0x00, 0xe8, 0xd4, 0xa5, 0x10, 0x00},
		},
		{
			"maximum 64-bit unsigned integer",
			uint64(18446744073709551615),
			[]byte{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		},
		// TODO: 18446744073709551616
		// TODO: -18446744073709551616
		// TODO: -18446744073709551617
		{
			"negative one",
			int(-1),
			[]byte{0x20},
		},
		{
			"negative ten",
			int(-10),
			[]byte{0x29},
		},
		{
			"negative one hundred",
			int(-100),
			[]byte{0x38, 0x63},
		},
		{
			"negative one thousand",
			int(-1000),
			[]byte{0x39, 0x03, 0xe7},
		},
		{
			"positive float zero",
			math.Copysign(0, 1),
			[]byte{0xf9, 0x00, 0x00},
		},
		{
			"negative float zero",
			math.Copysign(0, -1),
			[]byte{0xf9, 0x80, 0x00},
		},
		{
			"positive float one",
			float64(1),
			[]byte{0xf9, 0x3c, 0x00},
		},
		{
			"1.1",
			float64(1.1),
			[]byte{0xfb, 0x3f, 0xf1, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a},
		},
		{
			"1.5",
			float64(1.5),
			[]byte{0xf9, 0x3e, 0x00},
		},
		{
			"65504.0",
			float64(65504.0),
			[]byte{0xf9, 0x7b, 0xff},
		},
		{
			"100000.0",
			float64(100000.0),
			[]byte{0xfa, 0x47, 0xc3, 0x50, 0x00},
		},
		{
			"3.4028234663852886e+38",
			float64(3.4028234663852886e+38),
			[]byte{0xfa, 0x7f, 0x7f, 0xff, 0xff},
		},
		{
			"1.0e+300",
			float64(1.0e+300),
			[]byte{0xfb, 0x7e, 0x37, 0xe4, 0x3c, 0x88, 0x00, 0x75, 0x9c},
		},
		{
			"5.960464477539063e-8",
			float64(5.960464477539063e-8),
			[]byte{0xf9, 0x00, 0x01},
		},
		{
			"0.00006103515625",
			float64(0.00006103515625),
			[]byte{0xf9, 0x04, 0x00},
		},
		{
			"-4.0",
			float64(-4.0),
			[]byte{0xf9, 0xc4, 0x00},
		},
		{
			"-4.1",
			float64(-4.1),
			[]byte{0xfb, 0xc0, 0x10, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66},
		},
		{
			"Infinity",
			math.Inf(1),
			[]byte{0xf9, 0x7c, 0x00},
		},
		{
			"NaN",
			math.NaN(),
			[]byte{0xf9, 0x7e, 0x00},
		},
		{
			"-Infinity",
			math.Inf(-1),
			[]byte{0xf9, 0xfc, 0x00},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.v)
			if err != nil {
				t.Errorf("Marshal() error = %v", err)
				return
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Marshal() got = %x, want %x", got, tt.want)
			}
		})
	}
}
