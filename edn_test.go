package cbor

import (
	"testing"
)

func TestEncodeEDN(t *testing.T) {
	tests := []struct {
		in  RawMessage
		out string
	}{
		// positive integers
		{
			in:  RawMessage{0x00},
			out: "0",
		},
		{
			in:  RawMessage{0x17},
			out: "23",
		},
		{
			in:  RawMessage{0x18, 0x18},
			out: "24",
		},
		{
			in:  RawMessage{0x19, 0x01, 0x00},
			out: "256",
		},
		{
			in:  RawMessage{0x1a, 0x00, 0x01, 0x00, 0x00},
			out: "65536",
		},
		{
			in:  RawMessage{0x1b, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
			out: "4294967296",
		},
		{
			in:  RawMessage{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			out: "18446744073709551615",
		},

		// negative integers
		{
			in:  RawMessage{0x20},
			out: "-1",
		},
		{
			in:  RawMessage{0x37},
			out: "-24",
		},
		{
			in:  RawMessage{0x38, 0x18},
			out: "-25",
		},
		{
			in:  RawMessage{0x39, 0x01, 0x00},
			out: "-257",
		},
		{
			in:  RawMessage{0x3a, 0x00, 0x01, 0x00, 0x00},
			out: "-65537",
		},
		{
			in:  RawMessage{0x3b, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
			out: "-4294967297",
		},
		{
			in:  RawMessage{0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			out: "-18446744073709551616",
		},

		// {
		// 	in: RawMessage{
		// 		0xC0, // tag(0)
		// 		0x74, // text(20)
		// 		0x32, 0x30, 0x31, 0x33, 0x2D, 0x30, 0x33, 0x2D, 0x32, 0x31, 0x54, 0x32, 0x30, 0x3A, 0x30, 0x34, 0x3A, 0x30, 0x30, 0x5A,
		// 	},
		// 	out: `0("2013-03-21T20:04:00Z")`,
		// },
	}

	for _, tt := range tests {
		got, err := tt.in.EncodeEDN()
		if err != nil {
			t.Errorf("EncodeEDN() error = %v", err)
			continue
		}
		if string(got) != tt.out {
			t.Errorf("EncodeEDN() = %s, want %s", got, tt.out)
		}
	}
}
