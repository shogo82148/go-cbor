package cbor

import (
	"testing"
)

func TestEncodeEDN(t *testing.T) {
	tests := []struct {
		in  RawMessage
		out string
	}{
		{
			in:  RawMessage{0x00},
			out: "0",
		},
		{
			in:  RawMessage{0x18, 0x18},
			out: "24",
		},
		{
			in:  RawMessage{0x19, 0x01, 0x00},
			out: "256",
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
