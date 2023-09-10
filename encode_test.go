package cbor

import (
	"bytes"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.v)
			if err != nil {
				t.Errorf("Marshal() error = %v", err)
				return
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Marshal() got = %v, want %v", got, tt.want)
			}
		})
	}
}
