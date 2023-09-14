package cbor

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func ptr[T any](v T) *T {
	return &v
}

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		ptr  any
		want any
	}{
		// RFC 8949  Appendix A. Examples of Encoded CBOR Data Items
		{
			"integer zero",
			[]byte{0x00},
			new(int64),
			ptr(int64(0)),
		},
		{
			"integer one",
			[]byte{0x01},
			new(int64),
			ptr(int64(1)),
		},
		{
			"integer ten",
			[]byte{0x0a},
			new(int64),
			ptr(int64(10)),
		},
		{
			"integer twenty-three",
			[]byte{0x17},
			new(int64),
			ptr(int64(23)),
		},
		{
			"integer twenty-four",
			[]byte{0x18, 0x18},
			new(int64),
			ptr(int64(24)),
		},
		{
			"integer twenty-five",
			[]byte{0x18, 0x19},
			new(int64),
			ptr(int64(25)),
		},
		{
			"integer one hundred",
			[]byte{0x18, 0x64},
			new(int64),
			ptr(int64(100)),
		},
		{
			"integer one thousand",
			[]byte{0x19, 0x03, 0xe8},
			new(int64),
			ptr(int64(1000)),
		},
		{
			"integer one million",
			[]byte{0x1a, 0x00, 0x0f, 0x42, 0x40},
			new(int64),
			ptr(int64(1_000_000)),
		},
		{
			"1_000_000_000_000",
			[]byte{0x1b, 0x00, 0x00, 0x00, 0xe8, 0xd4, 0xa5, 0x10, 0x00},
			new(int64),
			ptr(int64(1_000_000_000_000)),
		},
		{
			"maximum 64-bit unsigned integer",
			[]byte{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			new(uint64),
			ptr(uint64(18446744073709551615)),
		},
		// TODO: 18446744073709551616
		// TODO: -18446744073709551616
		// TODO: -18446744073709551617
		{
			"negative one",
			[]byte{0x20},
			new(int64),
			ptr(int64(-1)),
		},
		{
			"negative ten",
			[]byte{0x29},
			new(int64),
			ptr(int64(-10)),
		},
		{
			"negative one hundred",
			[]byte{0x38, 0x63},
			new(int64),
			ptr(int64(-100)),
		},
		{
			"negative one thousand",
			[]byte{0x39, 0x03, 0xe7},
			new(int64),
			ptr(int64(-1000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Unmarshal(tt.data, tt.ptr); err != nil {
				t.Errorf("Unmarshal() error = %v", err)
			}
			if diff := cmp.Diff(tt.ptr, tt.want); diff != "" {
				t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
