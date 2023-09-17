package cbor

import (
	"math"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func ptr[T any](v T) *T {
	return &v
}

type someInterface interface {
	SomeMethod()
}

var unmarshalTests = []struct {
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
	{
		"float16 NaN",
		[]byte{0xf9, 0x7e, 0x00},
		new(float64),
		ptr(math.NaN()),
	},
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
		"float32 NaN",
		[]byte{0xfa, 0x7f, 0xc0, 0x00, 0x00},
		new(float64),
		ptr(math.NaN()),
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
		"float64 NaN",
		[]byte{0xfb, 0x7f, 0xf8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		new(float64),
		ptr(math.NaN()),
	},
	{
		"float64 -Infinity",
		[]byte{0xfb, 0xff, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		new(float64),
		ptr(math.Inf(-1)),
	},

	// decode to any
	{
		"decode unsigned int to any",
		[]byte{0x00},
		new(any),
		ptr(any(int64(0))),
	},
	{
		"decode signed int to any",
		[]byte{0x20},
		new(any),
		ptr(any(int64(-1))),
	},
	{
		"decode float to any",
		[]byte{0xf9, 0x00, 0x00},
		new(any),
		ptr(any(float64(0))),
	},

	// Unmarshaler
	{
		"Unmarshaler",
		[]byte{0x00},
		new(RawMessage),
		&RawMessage{0x00},
	},
}

func TestUnmarshal(t *testing.T) {
	for _, tt := range unmarshalTests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Unmarshal(tt.data, tt.ptr); err != nil {
				t.Errorf("Unmarshal() error = %v", err)
			}
			if diff := cmp.Diff(tt.ptr, tt.want, cmpopts.EquateNaNs()); diff != "" {
				t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func TestUnmarshal_Error(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		ptr  any
		err  *UnmarshalTypeError
	}{
		// positive integers
		{
			"int8 positive overflow",
			[]byte{0x18, 0x80}, // +128
			new(int8),
			&UnmarshalTypeError{Value: "integer", Type: typeOf[int8](), Offset: 0},
		},
		{
			"uint8 positive overflow",
			[]byte{0x19, 0x01, 0x00}, // +256
			new(uint8),
			&UnmarshalTypeError{Value: "integer", Type: typeOf[uint8](), Offset: 0},
		},
		{
			"int64 positive overflow",
			[]byte{0x1b, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // +2^63
			new(int64),
			&UnmarshalTypeError{Value: "integer", Type: typeOf[int64](), Offset: 0},
		},
		{
			"int64 positive overflow(any)",
			[]byte{0x1b, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // +2^63
			new(any),
			&UnmarshalTypeError{Value: "integer", Type: typeOf[any](), Offset: 0},
		},
		{
			"converting positive integer to float",
			[]byte{0x00},
			new(float64),
			&UnmarshalTypeError{Value: "integer", Type: typeOf[float64](), Offset: 0},
		},
		{
			"converting positive integer to some interface",
			[]byte{0x00},
			new(someInterface),
			&UnmarshalTypeError{Value: "integer", Type: typeOf[someInterface](), Offset: 0},
		},

		// negative integers
		{
			"int8 negative overflow",
			[]byte{0x38, 0x80}, // -129
			new(int8),
			&UnmarshalTypeError{Value: "integer", Type: typeOf[int8](), Offset: 0},
		},
		{
			"int64 negative overflow",
			[]byte{0x2b, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // +-2^63-1
			new(uint8),
			&UnmarshalTypeError{Value: "integer", Type: typeOf[uint8](), Offset: 0},
		},
		{
			"converting negative integer to float",
			[]byte{0x20},
			new(float64),
			&UnmarshalTypeError{Value: "integer", Type: typeOf[float64](), Offset: 0},
		},
		{
			"converting negative integer to some interface",
			[]byte{0x20},
			new(someInterface),
			&UnmarshalTypeError{Value: "integer", Type: typeOf[someInterface](), Offset: 0},
		},

		// floats
		{
			"float32 overflow",
			[]byte{0xfb, 0x7f, 0xef, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, // 1.7976931348623157e+308
			new(float32),
			&UnmarshalTypeError{Value: "float", Type: typeOf[float32](), Offset: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Unmarshal(tt.data, tt.ptr)
			e, ok := err.(*UnmarshalTypeError)
			if !ok {
				t.Errorf("Unmarshal() error = %v, want *UnmarshalTypeError", err)
				return
			}
			if e.Value != tt.err.Value {
				t.Errorf("unexpected Value: got %v, want %v", e.Value, tt.err.Value)
			}
			if e.Type != tt.err.Type {
				t.Errorf("unexpected Type: got %v, want %v", e.Type, tt.err.Type)
			}
			if e.Offset != tt.err.Offset {
				t.Errorf("unexpected Offset: got %v, want %v", e.Offset, tt.err.Offset)
			}
			if e.Struct != tt.err.Struct {
				t.Errorf("unexpected Struct: got %v, want %v", e.Struct, tt.err.Struct)
			}
			if e.Field != tt.err.Field {
				t.Errorf("unexpected Field: got %v, want %v", e.Field, tt.err.Field)
			}
		})
	}
}

// RFC 8949 Appendix F
var notWellFormed = [][]byte{
	// End of input in a head
	{0x18},
	{0x19},
	{0x1a},
	{0x1b},
	{0x19, 0x01},
	{0x1a, 0x01, 0x02},
	{0x1b, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
	{0x38},
	{0x58},
	{0x78},
	{0x98},
	{0x9a, 0x01, 0xff, 0x00},
	{0xb8},
	{0xd8},
	{0xf8},
	{0xf9, 0x00},
	{0xfa, 0x00, 0x00},
	{0xfb, 0x00, 0x00, 0x00},

	// Definite-length strings with short data
	{0x41},
	{0x61},
	{0x5a, 0xff, 0xff, 0xff, 0xff, 0x00},
	{0x5b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x02, 0x03},
	{0x7a, 0xff, 0xff, 0xff, 0xff, 0x00},
	{0x7b, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x02, 0x03},

	// Definite-length maps and arrays not closed with enough items
	{0x81},
	{0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81},
	{0x82, 0x00},
	{0xa1},
	{0xa2, 0x01, 0x02},
	{0xa1, 0x00},
	{0xa2, 0x00, 0x00, 0x00},

	// Tag number not followed by tag content
	{0xc0},

	// Indefinite-length strings not closed by a "break" stop code
	{0x5f, 0x41, 0x00},
	{0x7f, 0x61, 0x00},

	// Indefinite-length maps and arrays not closed by a "break" stop code
	{0x9f},
	{0x9f, 0x01, 0x02},
	{0xbf},
	{0xbf, 0x01, 0x02, 0x01, 0x02},
	{0x81, 0x9f},
	{0x9f, 0x80, 0x00},
	{0x9f, 0x9f, 0x9f, 0x9f, 0x9f, 0xff, 0xff, 0xff, 0xff},
	{0x9f, 0x81, 0x9f, 0x81, 0x9f, 0x9f, 0xff, 0xff, 0xff},

	// Subkind1: Reserved additional information values
	{0x1c},
	{0x1d},
	{0x1e},
	{0x3c},
	{0x3d},
	{0x3e},
	{0x5c},
	{0x5d},
	{0x5e},
	{0x7c},
	{0x7d},
	{0x7e},
	{0x9c},
	{0x9d},
	{0x9e},
	{0xbc},
	{0xbd},
	{0xbe},
	{0xdc},
	{0xdd},
	{0xde},
	{0xfc},
	{0xfd},
	{0xfe},

	// Subkind2: Reserved two-byte encodings of simple values
	{0xf8, 0x00},
	{0xf8, 0x01},
	{0xf8, 0x18},
	{0xf8, 0x1f},

	// Subkind3: Indefinite-length string chunks not of the correct type
	{0x5f, 0x00, 0xff},
	{0x5f, 0x21, 0xff},
	{0x5f, 0x61, 0x00, 0xff},
	{0x5f, 0x80, 0xff},
	{0x5f, 0xa0, 0xff},
	{0x5f, 0xc0, 0x00, 0xff},
	{0x5f, 0xe0, 0xff},
	{0x7f, 0x41, 0x00, 0xff},

	// Subkind4: Break occurring on its own outside of an indefinite-length item: 0xff,
	{0x81, 0xff},
	{0x82, 0x00, 0xff},
	{0xa1, 0xff},
	{0xa1, 0xff, 0x00},
	{0xa1, 0x00, 0xff},
	{0xa2, 0x00, 0x00, 0xff},
	{0x9f, 0x81, 0xff},
	{0x9f, 0x82, 0x9f, 0x81, 0x9f, 0x9f, 0xff, 0xff, 0xff, 0xff},
	{0xbf, 0x00, 0xff},
	{0xbf, 0x00, 0x00, 0x00, 0xff},

	// Subkind5: Major type 0, 1, 6 with additional information 31:
	{0x1f},
	{0x3f},
	{0xdf},
}

func TestValid_invalid(t *testing.T) {
	for _, tt := range notWellFormed {
		if Valid(tt) {
			t.Errorf("Valid(%x) = true, want false", tt)
		}
	}
}

func FuzzUnmarshal(f *testing.F) {
	for _, tt := range unmarshalTests {
		f.Add(tt.data)
	}
	for _, tt := range notWellFormed {
		f.Add(tt)
	}

	f.Fuzz(func(t *testing.T, a []byte) {
		var v any
		if err := Unmarshal(a, &v); err != nil {
			return
		}

		b, err := Marshal(v)
		if err != nil {
			t.Error(err)
		}

		var w any
		if err := Unmarshal(b, &w); err != nil {
			t.Error(err)
		}
		if diff := cmp.Diff(v, w, cmpopts.EquateNaNs()); diff != "" {
			t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
		}

		c, err := Marshal(w)
		if err != nil {
			t.Error(err)
		}
		if diff := cmp.Diff(b, c, cmpopts.EquateNaNs()); diff != "" {
			t.Errorf("Marshal() mismatch (-want +got):\n%s", diff)
		}
	})
}
