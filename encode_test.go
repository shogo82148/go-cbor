package cbor

import (
	"bytes"
	"math"
	"math/big"
	"net/url"
	"testing"
	"time"
)

func TestMarshal_Cycles(t *testing.T) {
	// Pointer cycle
	type PointerCycle struct {
		Ptr *PointerCycle
	}
	var pointerCycle = &PointerCycle{}
	pointerCycle.Ptr = pointerCycle

	// map cycle
	mapCycle := map[string]any{}
	mapCycle["a"] = map[string]any{"b": mapCycle}

	// slice cycle
	sliceCycle := []any{nil}
	sliceCycle[0] = []any{sliceCycle}

	for _, v := range []any{pointerCycle, mapCycle, sliceCycle} {
		_, err := Marshal(v)
		_, ok := err.(*UnsupportedValueError)
		if !ok {
			t.Errorf("expected error, got nil")
		}
	}
}

func TestMarshal_UnsupportedType(t *testing.T) {
	tests := []any{
		func() {},
		chan int(nil),
		complex(1, 2),
	}

	for _, v := range tests {
		_, err := Marshal(v)
		_, ok := err.(*UnsupportedTypeError)
		if !ok {
			t.Errorf("expected error, got nil")
		}
	}
}

func newBigInt(s string) *big.Int {
	i := new(big.Int)
	if _, ok := i.SetString(s, 0); !ok {
		panic("failed to parse big.Int: " + s)
	}
	return i
}

func newBigFloat(s string) *big.Float {
	f := new(big.Float)
	if _, ok := f.SetString(s); !ok {
		panic("failed to parse big.Float: " + s)
	}
	return f
}

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
			int64(1_000_000_000_000),
			[]byte{0x1b, 0x00, 0x00, 0x00, 0xe8, 0xd4, 0xa5, 0x10, 0x00},
		},
		{
			"maximum 64-bit unsigned integer",
			uint64(18446744073709551615),
			[]byte{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		},
		{
			"bigint: 18446744073709551616",
			newBigInt("18446744073709551616"),
			[]byte{0xc2, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			"-18446744073709551616",
			Integer{Sign: true, Value: 18446744073709551615},
			[]byte{0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		},
		{
			"bigint: -18446744073709551617",
			newBigInt("-18446744073709551617"),
			[]byte{0xc3, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
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
		{
			"false",
			false,
			[]byte{0xf4},
		},
		{
			"true",
			true,
			[]byte{0xf5},
		},
		{
			"null",
			nil,
			[]byte{0xf6},
		},
		{
			"undefined",
			Undefined,
			[]byte{0xf7},
		},
		{
			"simple value 16",
			Simple(16),
			[]byte{0xf0},
		},
		{
			"simple value 255",
			Simple(255),
			[]byte{0xf8, 0xff},
		},
		{
			"tag 0",
			Tag{
				Number:  0,
				Content: "2013-03-21T20:04:00Z",
			},
			[]byte{0xc0, 0x74, 0x32, 0x30, 0x31, 0x33, 0x2d, 0x30, 0x33, 0x2d, 0x32, 0x31, 0x54, 0x32, 0x30, 0x3a, 0x30, 0x34, 0x3a, 0x30, 0x30, 0x5a},
		},
		{
			"tag 1 integer",
			Tag{
				Number:  1,
				Content: int64(1363896240),
			},
			[]byte{0xc1, 0x1a, 0x51, 0x4b, 0x67, 0xb0},
		},
		{
			"tag 1 float",
			Tag{
				Number:  1,
				Content: float64(1363896240.5),
			},
			[]byte{0xc1, 0xfb, 0x41, 0xd4, 0x52, 0xd9, 0xec, 0x20, 0x00, 0x00},
		},
		{
			"tag 23",
			Tag{
				Number:  23,
				Content: []byte{0x01, 0x02, 0x03, 0x04},
			},
			[]byte{0xd7, 0x44, 0x01, 0x02, 0x03, 0x04},
		},
		{
			"tag 24",
			Tag{
				Number:  24,
				Content: []byte{0x64, 0x49, 0x45, 0x54, 0x46},
			},
			[]byte{0xd8, 0x18, 0x45, 0x64, 0x49, 0x45, 0x54, 0x46},
		},
		{
			"tag 32",
			Tag{
				Number:  32,
				Content: "http://www.example.com",
			},
			[]byte{0xd8, 0x20, 0x76, 0x68, 0x74, 0x74, 0x70, 0x3a, 0x2f, 0x2f, 0x77, 0x77, 0x77, 0x2e, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2e, 0x63, 0x6f, 0x6d},
		},
		{
			"byte string",
			[]byte{},
			[]byte{0x40},
		},
		{
			"byte string: h'01020304'",
			[]byte{0x01, 0x02, 0x03, 0x04},
			[]byte{0x44, 0x01, 0x02, 0x03, 0x04},
		},
		{
			"unicode string",
			"",
			[]byte{0x60},
		},
		{
			"unicode string: \"IETF\"",
			"IETF",
			[]byte{0x64, 0x49, 0x45, 0x54, 0x46},
		},
		{
			"unicode string: \"\"\"\\\"",
			"\"\\",
			[]byte{0x62, 0x22, 0x5c},
		},
		{
			"unicode string: \"\u00fc\"",
			"\u00fc",
			[]byte{0x62, 0xc3, 0xbc},
		},
		{
			"unicode string: \"\u6c34\"",
			"\u6c34",
			[]byte{0x63, 0xe6, 0xb0, 0xb4},
		},
		{
			"array",
			[]any{},
			[]byte{0x80},
		},
		{
			"array: [1, 2, 3]",
			[]int{1, 2, 3},
			[]byte{0x83, 0x01, 0x02, 0x03},
		},
		{
			"array: [1, [2, 3], [4, 5]]",
			[]any{1, []int{2, 3}, []int{4, 5}},
			[]byte{0x83, 0x01, 0x82, 0x02, 0x03, 0x82, 0x04, 0x05},
		},
		{
			"array that have 25 elements",
			[]int{
				1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
				21, 22, 23, 24, 25,
			},
			[]byte{
				0x98, 0x19, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
				0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16,
				0x17, 0x18, 0x18, 0x18, 0x19,
			},
		},
		{
			"empty map",
			map[string]any{},
			[]byte{0xa0},
		},
		{
			"map",
			map[int]int{
				1: 2,
				3: 4,
			},
			[]byte{0xa2, 0x01, 0x02, 0x03, 0x04},
		},
		{
			"map: {\"a\": 1, \"b\": [2, 3]}",
			map[string]any{
				"a": 1,
				"b": []int{2, 3},
			},
			[]byte{0xa2, 0x61, 0x61, 0x01, 0x61, 0x62, 0x82, 0x02, 0x03},
		},
		{
			"map abcde",
			map[string]string{
				"a": "A",
				"b": "B",
				"c": "C",
				"d": "D",
				"e": "E",
			},
			[]byte{
				0xa5,
				0x61, 0x61, 0x61, 0x41,
				0x61, 0x62, 0x61, 0x42,
				0x61, 0x63, 0x61, 0x43,
				0x61, 0x64, 0x61, 0x44,
				0x61, 0x65, 0x61, 0x45,
			},
		},

		// nils
		{
			"any type nil",
			any(nil),
			[]byte{0xf6},
		},
		{
			"map type nil",
			map[string]any(nil),
			[]byte{0xf6},
		},
		{
			"slice type nil",
			[]any(nil),
			[]byte{0xf6},
		},

		// integer types
		{
			"int8",
			int8(0),
			[]byte{0x00},
		},
		{
			"int16",
			int16(0),
			[]byte{0x00},
		},
		{
			"int32",
			int32(0),
			[]byte{0x00},
		},
		{
			"int64",
			int64(0),
			[]byte{0x00},
		},
		{
			"uint8",
			uint8(0),
			[]byte{0x00},
		},
		{
			"uint16",
			uint16(0),
			[]byte{0x00},
		},
		{
			"uint32",
			uint32(0),
			[]byte{0x00},
		},
		{
			"uint64",
			uint64(0),
			[]byte{0x00},
		},
		{
			"uintptr",
			uintptr(0),
			[]byte{0x00},
		},
		{
			"int",
			int(0),
			[]byte{0x00},
		},
		{
			"uint",
			uint(0),
			[]byte{0x00},
		},
		{
			"Integer",
			Integer{Sign: false, Value: 0},
			[]byte{0x00},
		},

		// float
		{
			"float32",
			float32(0),
			[]byte{0xf9, 0x00, 0x00},
		},

		// bigint
		{
			"bigint zero",
			newBigInt("0"),
			[]byte{0x00},
		},
		{
			"bigint one",
			newBigInt("1"),
			[]byte{0x01},
		},
		{
			"bigint ten",
			newBigInt("10"),
			[]byte{0x0a},
		},
		{
			"bigint twenty-three",
			newBigInt("23"),
			[]byte{0x17},
		},
		{
			"bigint twenty-four",
			newBigInt("24"),
			[]byte{0x18, 0x18},
		},
		{
			"bigint twenty-five",
			newBigInt("25"),
			[]byte{0x18, 0x19},
		},
		{
			"bigint one hundred",
			newBigInt("100"),
			[]byte{0x18, 0x64},
		},
		{
			"bigint one thousand",
			newBigInt("1000"),
			[]byte{0x19, 0x03, 0xe8},
		},
		{
			"bigint one million",
			newBigInt("1000000"),
			[]byte{0x1a, 0x00, 0x0f, 0x42, 0x40},
		},
		{
			"bigint 1_000_000_000_000",
			newBigInt("1000000000000"),
			[]byte{0x1b, 0x00, 0x00, 0x00, 0xe8, 0xd4, 0xa5, 0x10, 0x00},
		},
		{
			"bigint maximum 64-bit unsigned integer",
			newBigInt("18446744073709551615"),
			[]byte{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		},
		{
			"bigint2: 18446744073709551616",
			newBigInt("18446744073709551616"),
			[]byte{0xc2, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			"bigint -18446744073709551616",
			newBigInt("-18446744073709551616"),
			[]byte{0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		},
		{
			"bigint2: -18446744073709551617",
			newBigInt("-18446744073709551617"),
			[]byte{0xc3, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			"bigint negative one",
			newBigInt("-1"),
			[]byte{0x20},
		},
		{
			"bigint negative ten",
			newBigInt("-10"),
			[]byte{0x29},
		},
		{
			"bigint negative one hundred",
			newBigInt("-100"),
			[]byte{0x38, 0x63},
		},
		{
			"bigint negative one thousand",
			newBigInt("-1000"),
			[]byte{0x39, 0x03, 0xe7},
		},

		// bigfloat
		{
			"Bigfloat 1.5",
			newBigFloat("1.5"),
			[]byte{0xf9, 0x3e, 0x00},
		},
		{
			"Bigfloat 0.1",
			newBigFloat("0.1"),
			[]byte{0xc5, 0x82, 0x18, 0x3c, 0x1b, 0xcc, 0xcc, 0xcc, 0xcc, 0xcc, 0xcc, 0xcc, 0xcd},
		},

		// marshaler
		{
			"marshaler",
			RawMessage{0x01},
			[]byte{0x01},
		},

		// map key sort order
		{
			"RFC 8949 Section 4.2.1",
			map[any]any{
				10:          1,
				100:         2,
				-1:          3,
				"z":         4,
				"aa":        5,
				[1]int{100}: 6,
				[1]int{-1}:  7,
				false:       8,
			},
			[]byte{
				0xa8,       // 8 items map
				0x0a, 0x01, // 10
				0x18, 0x64, 0x02, // 100
				0x20, 0x03, // -1
				0x61, 0x7a, 0x04, // "z"
				0x62, 0x61, 0x61, 0x05, // "aa"
				0x81, 0x18, 0x64, 0x06, // [100]
				0x81, 0x20, 0x07, // [-1]
				0xf4, 0x08, // false
			},
		},

		{
			"simple value in map",
			map[string]any{
				"0": Simple(2),
			},
			[]byte{0xa1, 0x61, 0x30, 0xe2},
		},

		// datetime
		{
			"datetime",
			time.Unix(1363896240, 500_000_000).UTC(),
			[]byte{0xc1, 0xfb, 0x41, 0xd4, 0x52, 0xd9, 0xec, 0x20, 0x00, 0x00},
		},

		// known tag types
		{
			"uri",
			&url.URL{
				Scheme: "http",
				Host:   "www.example.com",
			},
			[]byte{0xd8, 0x20, 0x76, 0x68, 0x74, 0x74, 0x70, 0x3a, 0x2f, 0x2f, 0x77, 0x77, 0x77, 0x2e, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2e, 0x63, 0x6f, 0x6d},
		},
		{
			"base64 string",
			Base64String("8J+No/Cfjbo="),
			[]byte{0xd8, 0x22, 0x6c, 0x38, 0x4a, 0x2b, 0x4e, 0x6f, 0x2f, 0x43, 0x66, 0x6a, 0x62, 0x6f, 0x3d},
		},
		{
			"base64url string",
			Base64URLString("8J-No_Cfjbo"),
			[]byte{0xd8, 0x21, 0x6b, 0x38, 0x4a, 0x2d, 0x4e, 0x6f, 0x5f, 0x43, 0x66, 0x6a, 0x62, 0x6f},
		},

		// struct
		{
			"struct a",
			&FooA{A: 1, B: "2"},
			[]byte{0xa2, 0x61, 0x41, 0x01, 0x61, 0x42, 0x61, 0x32},
		},
		{
			"struct b",
			&FooB{Alg: 42, Kit: []byte("kit")},
			[]byte{0xa2, 0x01, 0x18, 0x2a, 0x04, 0x43, 0x6b, 0x69, 0x74},
		},
		{
			"struct b, omitempty",
			&FooB{Alg: 42},
			[]byte{0xa1, 0x01, 0x18, 0x2a},
		},
		{
			"struct c",
			&FooC{A: 1, B: "2"},
			[]byte{0x82, 0x01, 0x61, 0x32},
		},

		// invalid runes
		{
			"invalid runes 1",
			"\uFDDD",
			[]byte{0x63, 0xef, 0xb7, 0x9d},
		},
		{
			"invalid runes 2",
			"a\xffb",
			[]byte{0x65, 0x61, 0xef, 0xbf, 0xbd, 0x62},
		},
		{
			"invalid runes 3",
			"a\xffb\uFFFD",
			[]byte{0x68, 0x61, 0xef, 0xbf, 0xbd, 0x62, 0xef, 0xbf, 0xbd},
		},
		{
			"invalid runes 4",
			"a☺\xffb☺\xC0\xAFc☺\xff",
			[]byte{
				0x75, 0x61, 0xe2, 0x98, 0xba, 0xef, 0xbf, 0xbd,
				0x62, 0xe2, 0x98, 0xba, 0xef, 0xbf, 0xbd, 0x63,
				0xe2, 0x98, 0xba, 0xef, 0xbf, 0xbd,
			},
		},
		{
			"invalid runes 5",
			"\xC0\xAF",
			[]byte{0x63, 0xef, 0xbf, 0xbd},
		},
		{
			"invalid runes 6",
			"\xE0\x80\xAF",
			[]byte{0x63, 0xef, 0xbf, 0xbd},
		},
		{
			"invalid runes 7",
			"\xed\xa0\x80",
			[]byte{0x63, 0xef, 0xbf, 0xbd},
		},
		{
			"invalid runes 8",
			"\xed\xbf\xbf",
			[]byte{0x63, 0xef, 0xbf, 0xbd},
		},
		{
			"invalid runes 9",
			"\xF0\x80\x80\xaf",
			[]byte{0x63, 0xef, 0xbf, 0xbd},
		},
		{
			"invalid runes 10",
			"\xF8\x80\x80\x80\xAF",
			[]byte{0x63, 0xef, 0xbf, 0xbd},
		},
		{
			"invalid runes 11",
			"\xFC\x80\x80\x80\x80\xAF",
			[]byte{0x63, 0xef, 0xbf, 0xbd},
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

func BenchmarkMarshal_Uint64(b *testing.B) {
	r := newXorshift64()
	for i := 0; i < b.N; i++ {
		Marshal(r.Uint64())
	}
}

func BenchmarkMarshal_Int64(b *testing.B) {
	r := newXorshift64()
	for i := 0; i < b.N; i++ {
		Marshal(int64(r.Uint64()))
	}
}
