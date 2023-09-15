package cbor

import (
	"math"
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
		{
			"positive float zero",
			[]byte{0xf9, 0x00, 0x00},
			new(float64),
			ptr(float64(0)),
		},
		{
			"negative float zero",
			[]byte{0xf9, 0x80, 0x00},
			new(float64),
			ptr(float64(0)),
		},
		{
			"positive float one",
			[]byte{0xf9, 0x3c, 0x00},
			new(float64),
			ptr(float64(1)),
		},
		{
			"1.1",
			[]byte{0xfb, 0x3f, 0xf1, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a},
			new(float64),
			ptr(float64(1.1)),
		},
		{
			"1.5",
			[]byte{0xf9, 0x3e, 0x00},
			new(float64),
			ptr(float64(1.5)),
		},
		{
			"65504.0",
			[]byte{0xf9, 0x7b, 0xff},
			new(float64),
			ptr(float64(65504.0)),
		},
		{
			"100000.0",
			[]byte{0xfa, 0x47, 0xc3, 0x50, 0x00},
			new(float64),
			ptr(float64(100000.0)),
		},
		{
			"3.4028234663852886e+38",
			[]byte{0xfa, 0x7f, 0x7f, 0xff, 0xff},
			new(float64),
			ptr(float64(3.4028234663852886e+38)),
		},
		{
			"1.0e+300",
			[]byte{0xfb, 0x7e, 0x37, 0xe4, 0x3c, 0x88, 0x00, 0x75, 0x9c},
			new(float64),
			ptr(float64(1.0e+300)),
		},
		{
			"5.960464477539063e-8",
			[]byte{0xf9, 0x00, 0x01},
			new(float64),
			ptr(float64(5.960464477539063e-8)),
		},
		{
			"0.00006103515625",
			[]byte{0xf9, 0x04, 0x00},
			new(float64),
			ptr(float64(0.00006103515625)),
		},
		{
			"-4.0",
			[]byte{0xf9, 0xc4, 0x00},
			new(float64),
			ptr(float64(-4.0)),
		},
		{
			"-4.1",
			[]byte{0xfb, 0xc0, 0x10, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66},
			new(float64),
			ptr(float64(-4.1)),
		},
		{
			"float16 Infinity",
			[]byte{0xf9, 0x7c, 0x00},
			new(float64),
			ptr(math.Inf(1)),
		},
		// skip NaNs. cmp.Diff can't compare NaNs.
		{
			"float16 -Infinity",
			[]byte{0xf9, 0xfc, 0x00},
			new(float64),
			ptr(math.Inf(-1)),
		},
		{
			"float32 Infinity",
			[]byte{0xfa, 0x7f, 0x80, 0x00, 0x00},
			new(float64),
			ptr(math.Inf(1)),
		},
		{
			"float32 -Infinity",
			[]byte{0xfa, 0xff, 0x80, 0x00, 0x00},
			new(float64),
			ptr(math.Inf(-1)),
		},
		{
			"float64 Infinity",
			[]byte{0xfb, 0x7f, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			new(float64),
			ptr(math.Inf(1)),
		},
		{
			"float64 -Infinity",
			[]byte{0xfb, 0xff, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			new(float64),
			ptr(math.Inf(-1)),
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

func TestUnmarshal_NaN(t *testing.T) {
	t.Run("float16 NaN", func(t *testing.T) {
		var f float64
		if err := Unmarshal([]byte{0xf9, 0x7e, 0x00}, &f); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		if !math.IsNaN(f) {
			t.Errorf("Unmarshal() got = %v, want NaN", f)
		}
	})
	t.Run("float32 NaN", func(t *testing.T) {
		var f float64
		if err := Unmarshal([]byte{0xfa, 0x7f, 0xc0, 0x00, 0x00}, &f); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		if !math.IsNaN(f) {
			t.Errorf("Unmarshal() got = %v, want NaN", f)
		}
	})
	t.Run("float64 NaN", func(t *testing.T) {
		var f float64
		if err := Unmarshal([]byte{0xfb, 0x7f, 0xf8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, &f); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		if !math.IsNaN(f) {
			t.Errorf("Unmarshal() got = %v, want NaN", f)
		}
	})
}
