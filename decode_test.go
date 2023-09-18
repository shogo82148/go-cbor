package cbor

import (
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestDecodeState_isAvailable(t *testing.T) {
	d := &decodeState{
		data: []byte{0x00, 0x00},
		off:  1,
	}
	if !d.isAvailable(1) {
		t.Errorf("IsAvailable(1) = false, want true")
	}
	if d.isAvailable(2) {
		t.Errorf("IsAvailable(2) = true, want false")
	}

	// 1 + max.MaxInt overflows int
	if d.isAvailable(math.MaxInt) {
		t.Errorf("IsAvailable(math.MaxInt) = true, want false")
	}

	// int(math.MaxUint64) overflows int
	if d.isAvailable(math.MaxUint64) {
		t.Errorf("IsAvailable(math.MaxUint64) = true, want false")
	}
}

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
	{
		"-18446744073709551616",
		[]byte{0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		new(Integer),
		&Integer{Sign: true, Value: 18446744073709551615},
	},
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
	{
		"false",
		[]byte{0xf4},
		new(bool),
		ptr(false),
	},
	{
		"true",
		[]byte{0xf5},
		new(bool),
		ptr(true),
	},
	{
		"null",
		[]byte{0xf6},
		new(*int64),
		ptr((*int64)(nil)),
	},
	{
		"undefined",
		[]byte{0xf7},
		new(any),
		ptr(any(Undefined)),
	},
	{
		"simple(16)",
		[]byte{0xf0},
		new(Simple),
		ptr(Simple(16)),
	},
	{
		"simple(255)",
		[]byte{0xf8, 0xff},
		new(Simple),
		ptr(Simple(255)),
	},
	{
		"tag 0",
		[]byte{0xc0, 0x74, 0x32, 0x30, 0x31, 0x33, 0x2d, 0x30, 0x33, 0x2d, 0x32, 0x31, 0x54, 0x32, 0x30, 0x3a, 0x30, 0x34, 0x3a, 0x30, 0x30, 0x5a},
		new(Tag),
		&Tag{
			Number:  0,
			Content: "2013-03-21T20:04:00Z",
		},
	},
	{
		"tag 1 integer",
		[]byte{0xc1, 0x1a, 0x51, 0x4b, 0x67, 0xb0},
		new(Tag),
		&Tag{
			Number:  1,
			Content: int64(1363896240),
		},
	},
	{
		"tag 1 float",
		[]byte{0xc1, 0xfb, 0x41, 0xd4, 0x52, 0xd9, 0xec, 0x20, 0x00, 0x00},
		new(Tag),
		&Tag{
			Number:  1,
			Content: float64(1363896240.5),
		},
	},
	{
		"tag 23",
		[]byte{0xd7, 0x44, 0x01, 0x02, 0x03, 0x04},
		new(Tag),
		&Tag{
			Number:  23,
			Content: []byte{0x01, 0x02, 0x03, 0x04},
		},
	},
	{
		"tag 24",
		[]byte{0xd8, 0x18, 0x45, 0x64, 0x49, 0x45, 0x54, 0x46},
		new(Tag),
		&Tag{
			Number:  24,
			Content: []byte{0x64, 0x49, 0x45, 0x54, 0x46},
		},
	},
	{
		"tag 32",
		[]byte{0xd8, 0x20, 0x76, 0x68, 0x74, 0x74, 0x70, 0x3a, 0x2f, 0x2f, 0x77, 0x77, 0x77, 0x2e, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2e, 0x63, 0x6f, 0x6d},
		new(Tag),
		&Tag{
			Number:  32,
			Content: "http://www.example.com",
		},
	},
	{
		"zero-length byte string",
		[]byte{0x40},
		new([]byte),
		ptr([]byte{}),
	},
	{
		"byte string",
		[]byte{0x44, 0x01, 0x02, 0x03, 0x04},
		new([]byte),
		ptr([]byte{0x01, 0x02, 0x03, 0x04}),
	},
	{
		"utf8 string",
		[]byte{0x60},
		new(string),
		ptr(""),
	},
	{
		"utf8 string: \"IETF\"",
		[]byte{0x64, 0x49, 0x45, 0x54, 0x46},
		new(string),
		ptr("IETF"),
	},
	{
		`utf8 string: "\"\\"`,
		[]byte{0x62, 0x22, 0x5c},
		new(string),
		ptr("\"\\"),
	},
	{
		`utf8 string: "\u00fc"`,
		[]byte{0x62, 0xc3, 0xbc},
		new(string),
		ptr("\u00fc"),
	},
	{
		`utf8 string: "\u6c34"`,
		[]byte{0x63, 0xe6, 0xb0, 0xb4},
		new(string),
		ptr("\u6c34"),
	},
	{
		`utf8 string: "\ud800\udd51"`,
		[]byte{0x64, 0xf0, 0x90, 0x85, 0x91},
		new(string),
		ptr("\xf0\x90\x85\x91"),
	},
	{
		"zero-length array",
		[]byte{0x80},
		new([]int64),
		ptr([]int64{}),
	},
	{
		"array",
		[]byte{0x83, 0x01, 0x02, 0x03},
		new([]int64),
		ptr([]int64{1, 2, 3}),
	},
	{
		"array: [1, [2, 3], [4, 5]]",
		[]byte{0x83, 0x01, 0x82, 0x02, 0x03, 0x82, 0x04, 0x05},
		new([]any),
		ptr([]any{int64(1), []any{int64(2), int64(3)}, []any{int64(4), int64(5)}}),
	},
	{
		"25-elements array",
		[]byte{
			0x98, 0x19,
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
			0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14,
			0x15, 0x16, 0x17, 0x18, 0x18, 0x18, 0x19,
		},
		new([]int64),
		ptr([]int64{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
			11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
			21, 22, 23, 24, 25,
		}),
	},
	{
		"zero-length map",
		[]byte{0xa0},
		new(map[string]int64),
		ptr(map[string]int64{}),
	},
	{
		"map",
		[]byte{0xa2, 0x01, 0x02, 0x03, 0x04},
		new(map[int64]int64),
		ptr(map[int64]int64{1: 2, 3: 4}),
	},
	{
		"array in map",
		[]byte{0xa2, 0x61, 0x61, 0x01, 0x61, 0x62, 0x82, 0x02, 0x03},
		new(map[string]any),
		ptr(map[string]any{"a": int64(1), "b": []any{int64(2), int64(3)}}),
	},
	{
		"map in array",
		[]byte{0x82, 0x61, 0x61, 0xa1, 0x61, 0x62, 0x61, 0x63},
		new([]any),
		ptr([]any{"a", map[string]any{"b": "c"}}),
	},
	{
		"map abcde",
		[]byte{0xa5, 0x61, 0x61, 0x61, 0x41, 0x61, 0x62, 0x61, 0x42, 0x61, 0x63, 0x61, 0x43, 0x61, 0x64, 0x61, 0x44, 0x61, 0x65, 0x61, 0x45},
		new(map[string]string),
		ptr(map[string]string{
			"a": "A",
			"b": "B",
			"c": "C",
			"d": "D",
			"e": "E",
		}),
	},
	{
		"indefinite-length byte string",
		[]byte{0x5f, 0x42, 0x01, 0x02, 0x43, 0x03, 0x4, 0x05, 0xff},
		new([]byte),
		ptr([]byte{0x01, 0x02, 0x03, 0x04, 0x05}),
	},
	{
		"indefinite-length utf8 string",
		[]byte{0x7f, 0x65, 0x73, 0x74, 0x72, 0x65, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x67, 0xff},
		new(string),
		ptr("streaming"),
	},
	{
		"zero-length indefinite-length array",
		[]byte{0x9f, 0xff},
		new([]int64),
		ptr([]int64{}),
	},
	{
		"indefinite-length array 1",
		[]byte{0x9f, 0x01, 0x82, 0x02, 0x03, 0x9f, 0x04, 0x05, 0xff, 0xff},
		new([]any),
		ptr([]any{int64(1), []any{int64(2), int64(3)}, []any{int64(4), int64(5)}}),
	},
	{
		"indefinite-length array 2",
		[]byte{0x9f, 0x01, 0x82, 0x02, 0x03, 0x82, 0x04, 0x05, 0xff},
		new([]any),
		ptr([]any{int64(1), []any{int64(2), int64(3)}, []any{int64(4), int64(5)}}),
	},
	{
		"indefinite-length array 3",
		[]byte{0x83, 0x01, 0x82, 0x02, 0x03, 0x9f, 0x04, 0x05, 0xff},
		new([]any),
		ptr([]any{int64(1), []any{int64(2), int64(3)}, []any{int64(4), int64(5)}}),
	},
	{
		"indefinite-length array 4",
		[]byte{0x83, 0x01, 0x9f, 0x02, 0x03, 0xff, 0x82, 0x04, 0x05},
		new([]any),
		ptr([]any{int64(1), []any{int64(2), int64(3)}, []any{int64(4), int64(5)}}),
	},
	{
		"indefinite-length array 5",
		[]byte{0x9f, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x18, 0x18, 0x19, 0xff},
		new([]int),
		ptr([]int{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25,
		}),
	},
	{
		"array in indefinite-length map",
		[]byte{0xbf, 0x61, 0x61, 0x01, 0x61, 0x62, 0x9f, 0x02, 0x03, 0xff, 0xff},
		new(map[string]any),
		ptr(map[string]any{"a": int64(1), "b": []any{int64(2), int64(3)}}),
	},
	{
		"indefinite-length map in array",
		[]byte{0x82, 0x61, 0x61, 0xbf, 0x61, 0x62, 0x61, 0x63, 0xff},
		new([]any),
		ptr([]any{"a", map[string]any{"b": "c"}}),
	},
	{
		"indefinite-length map",
		[]byte{0xbf, 0x63, 0x46, 0x75, 0x6e, 0xf5, 0x63, 0x41, 0x6d, 0x74, 0x21, 0xff},
		new(map[string]any),
		ptr(map[string]any{"Fun": true, "Amt": int64(-2)}),
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
	{
		"decode empty byte string to any",
		[]byte{0x40},
		new(any),
		ptr(any([]byte{})),
	},
	{
		"decode indefinite-length empty byte string to any",
		[]byte{0x5f, 0xff},
		new(any),
		ptr(any([]byte{})),
	},
	{
		"decode byte string to any",
		[]byte{0x44, 0x01, 0x02, 0x03, 0x04},
		new(any),
		ptr(any([]byte{0x01, 0x02, 0x03, 0x04})),
	},
	{
		"decode utf8 string to any",
		[]byte{0x64, 0x49, 0x45, 0x54, 0x46},
		new(any),
		ptr(any("IETF")),
	},
	{
		"tag 0",
		[]byte{0xc0, 0x74, 0x32, 0x30, 0x31, 0x33, 0x2d, 0x30, 0x33, 0x2d, 0x32, 0x31, 0x54, 0x32, 0x30, 0x3a, 0x30, 0x34, 0x3a, 0x30, 0x30, 0x5a},
		new(Tag),
		&Tag{
			Number:  0,
			Content: "2013-03-21T20:04:00Z",
		},
	},
	{
		"tag 1 integer",
		[]byte{0xc1, 0x1a, 0x51, 0x4b, 0x67, 0xb0},
		new(any),
		ptr(any(Tag{
			Number:  1,
			Content: int64(1363896240),
		})),
	},
	{
		"tag 1 float",
		[]byte{0xc1, 0xfb, 0x41, 0xd4, 0x52, 0xd9, 0xec, 0x20, 0x00, 0x00},
		new(any),
		ptr(any(Tag{
			Number:  1,
			Content: float64(1363896240.5),
		})),
	},
	{
		"tag 23",
		[]byte{0xd7, 0x44, 0x01, 0x02, 0x03, 0x04},
		new(any),
		ptr(any(Tag{
			Number:  23,
			Content: []byte{0x01, 0x02, 0x03, 0x04},
		})),
	},
	{
		"tag 24",
		[]byte{0xd8, 0x18, 0x45, 0x64, 0x49, 0x45, 0x54, 0x46},
		new(any),
		ptr(any(Tag{
			Number:  24,
			Content: []byte{0x64, 0x49, 0x45, 0x54, 0x46},
		})),
	},
	{
		"tag 32",
		[]byte{0xd8, 0x20, 0x76, 0x68, 0x74, 0x74, 0x70, 0x3a, 0x2f, 0x2f, 0x77, 0x77, 0x77, 0x2e, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2e, 0x63, 0x6f, 0x6d},
		new(any),
		ptr(any(Tag{
			Number:  32,
			Content: "http://www.example.com",
		})),
	},
	{
		"simple(16)",
		[]byte{0xf0},
		new(any),
		ptr(any(Simple(16))),
	},
	{
		"simple(255)",
		[]byte{0xf8, 0xff},
		new(any),
		ptr(any(Simple(255))),
	},

	// decode to struct
	{
		"map to struct a",
		[]byte{0xa2, 0x61, 0x41, 0x01, 0x61, 0x42, 0x61, 0x32},
		new(FooA),
		&FooA{A: 1, B: "2"},
	},
	{
		"indefinite-length map to struct a",
		[]byte{0xbf, 0x61, 0x41, 0x01, 0x61, 0x42, 0x61, 0x32, 0xff},
		new(FooA),
		&FooA{A: 1, B: "2"},
	},
	{
		"map to struct b",
		[]byte{0xa2, 0x01, 0x18, 0x2a, 0x04, 0x43, 0x6b, 0x69, 0x74},
		new(FooB),
		&FooB{Alg: 42, Kit: []byte("kit")},
	},
	{
		"array to struct c",
		[]byte{0x82, 0x01, 0x61, 0x32},
		new(FooC),
		&FooC{A: 1, B: "2"},
	},
	{
		"short array to struct c",
		[]byte{0x81, 0x01},
		&FooC{A: 1, B: "2"},
		&FooC{A: 1},
	},
	{
		"long array to struct c",
		[]byte{0x83, 0x01, 0x61, 0x32, 0x02},
		&FooC{},
		&FooC{A: 1, B: "2"},
	},
	{
		"indefinite-length array to struct c",
		[]byte{0x9f, 0x01, 0x61, 0x32, 0xff},
		new(FooC),
		&FooC{A: 1, B: "2"},
	},
	{
		"short indefinite-length array to struct c",
		[]byte{0x9f, 0x01, 0xff},
		&FooC{A: 1, B: "2"},
		&FooC{A: 1},
	},
	{
		"long indefinite-length array to struct c",
		[]byte{0x9f, 0x01, 0x61, 0x32, 0x02, 0xff},
		new(FooC),
		&FooC{A: 1, B: "2"},
	},
}

func TestUnmarshal(t *testing.T) {
	for _, tt := range unmarshalTests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Unmarshal(tt.data, tt.ptr); err != nil {
				t.Errorf("Unmarshal() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, tt.ptr, cmpopts.EquateNaNs()); diff != "" {
				t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUnmarshal_Unmarshaler(t *testing.T) {
	for _, tt := range unmarshalTests {
		t.Run(tt.name, func(t *testing.T) {
			var got RawMessage
			if err := Unmarshal(tt.data, &got); err != nil {
				t.Errorf("Unmarshal() error = %v", err)
			}
			if diff := cmp.Diff(tt.data, []byte(got)); diff != "" {
				t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUnmarshal_BigInt(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		input := []byte{0xc2, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		var got *big.Int
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := newBigInt("18446744073709551616")
		if got.Cmp(want) != 0 {
			t.Errorf("Unmarshal() = %x, want %x", got, want)
		}
	})
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
			[]byte{0x3b, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // +-2^63-1
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

		// struct
		{
			"map to struct",
			[]byte{0xa1, 0x61, 0x41, 0x61, 0x30}, // {A: "0"}
			new(FooA),
			&UnmarshalTypeError{Value: "string", Type: typeOf[int](), Offset: 3, Struct: "FooA", Field: "A"},
		},
		{
			"indefinite-length map to struct",
			[]byte{0xbf, 0x61, 0x41, 0x61, 0x30, 0xff}, // {_ A: "0"}
			new(FooA),
			&UnmarshalTypeError{Value: "string", Type: typeOf[int](), Offset: 3, Struct: "FooA", Field: "A"},
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

func TestWellFormed_valid(t *testing.T) {
	for _, tt := range unmarshalTests {
		if !WellFormed(tt.data) {
			t.Errorf("Valid(%x) = false, want true", tt.data)
		}
	}
}

func TestWellFormed_invalid(t *testing.T) {
	for _, tt := range notWellFormed {
		if WellFormed(tt) {
			t.Errorf("Valid(%x) = true, want false", tt)
		}
	}
}

func BenchmarkMaliciousCBORData(b *testing.B) {
	var v any
	input := []byte{0x9B, 0x00, 0x00, 0x42, 0xFA, 0x42, 0xFA, 0x42, 0xFA, 0x42}
	for i := 0; i < b.N; i++ {
		if err := Unmarshal(input, v); err == nil {
			b.Error("want error, but not")
		}
	}
}
