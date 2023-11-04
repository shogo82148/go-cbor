package cbor

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestDecodeEDN(t *testing.T) {
	tests := []struct {
		in  string
		out RawMessage
	}{
		// positive integers
		{
			in:  "0",
			out: RawMessage{0x00},
		},
		{
			in:  "+1",
			out: RawMessage{0x01},
		},
		{
			in:  "-1",
			out: RawMessage{0x20},
		},

		// floats
		{
			in:  "0.0",
			out: RawMessage{0xf9, 0x00, 0x00},
		},
		{
			in:  "-Infinity",
			out: RawMessage{0xf9, 0xfc, 0x00},
		},
		{
			in:  "+Infinity",
			out: RawMessage{0xf9, 0x7c, 0x00},
		},

		// byte strings
		// from RFC 8610 Appendix G.1. and G.6.
		{
			in:  "h'48656c6c6f20776f726c64'",
			out: RawMessage{0x4b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  "h'48656c6c6f20776f726c64'",
			out: RawMessage{0x4b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  "h'4 86 56c 6c6f\n20776 f726c64'",
			out: RawMessage{0x4b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  "h'68656c6c6f20776f726c64'",
			out: RawMessage{0x4b, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in: "h'68 65 6c /doubled l!/ 6c 6f /hello/\n" +
				"20 /space/\n" +
				"77 6f 72 6c 64' /world/",
			out: RawMessage{0x4b, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},

		// from RFC 8610 Appendix G.4
		{
			in:  `'Hello world'`,
			out: RawMessage{0x4b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  `'Hello ' 'world'`,
			out: RawMessage{0x4b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  `'Hello ' h'776f726c64'`,
			out: RawMessage{0x4b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  `'Hello' h'20' 'world'`,
			out: RawMessage{0x4b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  ` '' h'48656c6c6f20776f726c64' '' b64''`,
			out: RawMessage{0x4b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  `h'4 86 56c 6c6f' h' 20776 f726c64'`,
			out: RawMessage{0x4b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},

		// from RFC 8949 Section 8.
		{
			in:  "h'12345678'",
			out: RawMessage{0x44, 0x12, 0x34, 0x56, 0x78},
		},
		{
			in:  "h32'CI2FM6A'",
			out: RawMessage{0x44, 0x12, 0x34, 0x56, 0x78},
		},
		{
			in:  "b64'EjRWeA'",
			out: RawMessage{0x44, 0x12, 0x34, 0x56, 0x78},
		},

		// text strings
		{
			in:  `""`,
			out: RawMessage{0x60},
		},

		// from RFC 8610 Appendix G.4
		{
			in:  `"Hello world"`,
			out: RawMessage{0x6b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  `"Hello " "world"`,
			out: RawMessage{0x6b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  `"Hello" h'20' "world"`,
			out: RawMessage{0x6b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
		{
			in:  `"" h'48656c6c6f20776f726c64' ""`,
			out: RawMessage{0x6b, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},

		// arrays
		{
			in:  "[]",
			out: RawMessage{0x80},
		},
		{
			in:  "[0]",
			out: RawMessage{0x81, 0x00},
		},
		{
			in:  "[_0 0]",
			out: RawMessage{0x98, 0x01, 0x00},
		},
		{
			in:  "[_1 0]",
			out: RawMessage{0x99, 0x00, 0x01, 0x00},
		},
		{
			in:  "[_2 0]",
			out: RawMessage{0x9a, 0x00, 0x00, 0x00, 0x01, 0x00},
		},
		{
			in:  "[_3 0]",
			out: RawMessage{0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00},
		},
		{
			in:  "[_7 0]",
			out: RawMessage{0x9f, 0x00, 0xff},
		},
		{
			in:  "[_ 0]",
			out: RawMessage{0x9f, 0x00, 0xff},
		},

		// numbers from RFC 8610 Appendix G.5.
		{
			in:  "4711",
			out: RawMessage{0x19, 0x12, 0x67},
		},
		{
			in:  "0x1267",
			out: RawMessage{0x19, 0x12, 0x67},
		},
		{
			in:  "0o11147",
			out: RawMessage{0x19, 0x12, 0x67},
		},
		{
			in:  "0b1001001100111",
			out: RawMessage{0x19, 0x12, 0x67},
		},
		{
			in:  "1.5",
			out: RawMessage{0xf9, 0x3e, 0x00},
		},
		{
			in:  "0x1.8p0",
			out: RawMessage{0xf9, 0x3e, 0x00},
		},
		{
			in:  "0x18p-4",
			out: RawMessage{0xf9, 0x3e, 0x00},
		},

		// RFC 8949 Appendix A.
		{
			in:  "0",
			out: RawMessage{0x00},
		},
		{
			in:  "1",
			out: RawMessage{0x01},
		},
		{
			in:  "10",
			out: RawMessage{0x0a},
		},
		{
			in:  "23",
			out: RawMessage{0x17},
		},
		{
			in:  "24",
			out: RawMessage{0x18, 0x18},
		},
		{
			in:  "25",
			out: RawMessage{0x18, 0x19},
		},
		{
			in:  "100",
			out: RawMessage{0x18, 0x64},
		},
		{
			in:  "1000",
			out: RawMessage{0x19, 0x03, 0xe8},
		},
		{
			in:  "1000000",
			out: RawMessage{0x1a, 0x00, 0x0f, 0x42, 0x40},
		},
		{
			in:  "1000000000000",
			out: RawMessage{0x1b, 0x00, 0x00, 0x00, 0xe8, 0xd4, 0xa5, 0x10, 0x00},
		},
		{
			in:  "18446744073709551615",
			out: RawMessage{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		},
		// TODO: fix this
		// {
		// 	in: "18446744073709551616",
		// 	out: RawMessage{0xc2, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		// },
		{
			in:  "-18446744073709551616",
			out: RawMessage{0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		},
		// TODO: fix this
		// {
		// 	in: "-18446744073709551617",
		// 	out: RawMessage{0xc3, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		// },
		{
			in:  "-1",
			out: RawMessage{0x20},
		},
		{
			in:  "-10",
			out: RawMessage{0x29},
		},
		{
			in:  "-100",
			out: RawMessage{0x38, 0x63},
		},
		{
			in:  "-1000",
			out: RawMessage{0x39, 0x03, 0xe7},
		},
		{
			in:  "0.0",
			out: RawMessage{0xf9, 0x00, 0x00},
		},
		{
			in:  "-0.0",
			out: RawMessage{0xf9, 0x80, 0x00},
		},
		{
			in:  "1.0",
			out: RawMessage{0xf9, 0x3c, 0x00},
		},
		{
			in:  "1.1",
			out: RawMessage{0xfb, 0x3f, 0xf1, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a},
		},
		{
			in:  "1.5",
			out: RawMessage{0xf9, 0x3e, 0x00},
		},
		{
			in:  "100000.0",
			out: RawMessage{0xfa, 0x47, 0xc3, 0x50, 0x00},
		},
		{
			in:  "3.4028234663852886e+38",
			out: RawMessage{0xfa, 0x7f, 0x7f, 0xff, 0xff},
		},
		{
			in:  "1e+300",
			out: RawMessage{0xfb, 0x7e, 0x37, 0xe4, 0x3c, 0x88, 0x00, 0x75, 0x9c},
		},
		{
			in:  "5.960464477539063e-8",
			out: RawMessage{0xf9, 0x00, 0x01},
		},
		{
			in:  "0.00006103515625",
			out: RawMessage{0xf9, 0x04, 0x00},
		},
		{
			in:  "-4.0",
			out: RawMessage{0xf9, 0xc4, 0x00},
		},
		{
			in:  "-4.1",
			out: RawMessage{0xfb, 0xc0, 0x10, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66},
		},
		{
			in:  "Infinity",
			out: RawMessage{0xf9, 0x7c, 0x00},
		},
		{
			in:  "NaN",
			out: RawMessage{0xf9, 0x7e, 0x00},
		},
		{
			in:  "-Infinity",
			out: RawMessage{0xf9, 0xfc, 0x00},
		},
		{
			in:  "false",
			out: RawMessage{0xf4},
		},
		{
			in:  "true",
			out: RawMessage{0xf5},
		},
		{
			in:  "null",
			out: RawMessage{0xf6},
		},
		{
			in:  "undefined",
			out: RawMessage{0xf7},
		},
		// {
		// 	in:  "simple(16)",
		// 	out: RawMessage{0xf0},
		// },
		// {
		// 	in:  "simple(255)",
		// 	out: RawMessage{0xf8, 0xff},
		// },
		// {
		// 	in:  "0(\"2013-03-21T20:04:00Z\")",
		// 	out: RawMessage{0xc0, 0x74, 0x32, 0x30, 0x31, 0x33, 0x2d, 0x30, 0x33, 0x2d, 0x32, 0x31, 0x54, 0x32, 0x30, 0x3a, 0x30, 0x34, 0x3a, 0x30, 0x30, 0x5a},
		// },
		// {
		// 	in:  "1(1363896240)",
		// 	out: RawMessage{0xc1, 0x1a, 0x51, 0x4b, 0x67, 0xb0},
		// },
		// {
		// 	in:  "1(1363896240.5)",
		// 	out: RawMessage{0xc1, 0xfb, 0x41, 0xd4, 0x52, 0xd9, 0xec, 0x20, 0x00, 0x00},
		// },
		// {
		// 	in:  "23(h'010203')",
		// 	out: RawMessage{0xd7, 0x43, 0x01, 0x02, 0x03},
		// },
		// {
		// 	in:  "32(\"http://www.example.com\")",
		// 	out: RawMessage{0xd8, 0x20, 0x76, 0x68, 0x74, 0x74, 0x70, 0x3a, 0x2f, 0x2f, 0x77, 0x77, 0x77, 0x2e, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2e, 0x63, 0x6f, 0x6d},
		// },
		{
			in:  "h''",
			out: RawMessage{0x40},
		},
		{
			in:  "h'010203'",
			out: RawMessage{0x43, 0x01, 0x02, 0x03},
		},
		{
			in:  `""`,
			out: RawMessage{0x60},
		},
		{
			in:  `"a"`,
			out: RawMessage{0x61, 0x61},
		},
		{
			in:  `"IETF"`,
			out: RawMessage{0x64, 0x49, 0x45, 0x54, 0x46},
		},
		{
			in:  `"\"\\"`,
			out: RawMessage{0x62, 0x22, 0x5c},
		},
		{
			in:  `"\u00fc"`,
			out: RawMessage{0x62, 0xc3, 0xbc},
		},
		{
			in:  `"\u6c34"`,
			out: RawMessage{0x63, 0xe6, 0xb0, 0xb4},
		},
		{
			in:  `"\ud800\udd51"`,
			out: RawMessage{0x64, 0xf0, 0x90, 0x85, 0x91},
		},
		{
			in:  `[]`,
			out: RawMessage{0x80},
		},
		{
			in:  `[1, 2, 3]`,
			out: RawMessage{0x83, 0x01, 0x02, 0x03},
		},
		{
			in:  `[1, [2, 3], [4, 5]]`,
			out: RawMessage{0x83, 0x01, 0x82, 0x02, 0x03, 0x82, 0x04, 0x05},
		},
		{
			in: `[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]`,
			out: RawMessage{
				0x98, 0x19,
				0x01,
				0x02,
				0x03,
				0x04,
				0x05,
				0x06,
				0x07,
				0x08,
				0x09,
				0x0a,
				0x0b,
				0x0c,
				0x0d,
				0x0e,
				0x0f,
				0x10,
				0x11,
				0x12,
				0x13,
				0x14,
				0x15,
				0x16,
				0x17,
				0x18, 0x18,
				0x18, 0x19,
			},
		},

		{
			in:  `[_ ]`,
			out: RawMessage{0x9f, 0xff},
		},

		// TODO: fix this
		// {
		// 	in: `[_ 1, [2, 3], [_ 4, 5]]`,
		// 	out: RawMessage{
		// 		0x9f,
		// 		0x01,
		// 		0x82, 0x02, 0x03,
		// 		0x9f, 0x04, 0x05, 0xff,
		// 		0xff,
		// 	},
		// },
		// {
		// 	in: `[_ 1, [2, 3], [4, 5]]`,
		// 	out: RawMessage{
		// 		0x9f,
		// 		0x01,
		// 		0x82, 0x02, 0x03,
		// 		0x82, 0x04, 0x05,
		// 		0xff,
		// 	},
		// },
		// {
		// 	in: `[1, [_ 2, 3], [4, 5]]`,
		// 	out: RawMessage{
		// 		0x83,
		// 		0x01,
		// 		0x9f, 0x02, 0x03, 0xff,
		// 		0x82, 0x04, 0x05,
		// 	},
		// },
		// {
		// 	in: `[_ 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]`,
		// 	out: RawMessage{
		// 		0x9f,
		// 		0x01,
		// 		0x02,
		// 		0x03,
		// 		0x04,
		// 		0x05,
		// 		0x06,
		// 		0x07,
		// 		0x08,
		// 		0x09,
		// 		0x0a,
		// 		0x0b,
		// 		0x0c,
		// 		0x0d,
		// 		0x0e,
		// 		0x0f,
		// 		0x10,
		// 		0x11,
		// 		0x12,
		// 		0x13,
		// 		0x14,
		// 		0x15,
		// 		0x16,
		// 		0x17,
		// 		0x18, 0x18,
		// 		0x18, 0x19,
		// 		0xff,
		// 	},
		// },
		// {
		// 	in: `{_ "a": 1, "b": [_ 2, 3]}`,
		// 	out: RawMessage{
		// 		0xbf,
		// 		0x61, 0x61, 0x01,
		// 		0x61, 0x62, 0x9f, 0x02, 0x03, 0xff,
		// 		0xff,
		// 	},
		// },
		// {
		// 	in: `["a", {_ "b": "c"}]`,
		// 	out: RawMessage{
		// 		0x82,
		// 		0x61, 0x61,
		// 		0xbf, 0x61, 0x62, 0x61, 0x63, 0xff,
		// 	},
		// },
		// {
		// 	in: `{_ "Fun": true, "Amt": -2}`,
		// 	out: RawMessage{
		// 		0xbf,
		// 		0x63, 0x46, 0x75, 0x6e, 0xf5,
		// 		0x63, 0x41, 0x6d, 0x74, 0x21,
		// 		0xff,
		// 	},
		// },
	}

	for _, tt := range tests {
		msg, err := DecodeEDN([]byte(tt.in))
		if err != nil {
			t.Errorf("DecodeEDN(%q) returned error %v", tt.in, err)
		}
		if !bytes.Equal(msg, tt.out) {
			t.Errorf("DecodeEDN(%q) = \n%s\nwant:\n%s", tt.in, hex.Dump(msg), hex.Dump(tt.out))
		}
	}
}

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

		// byte strings
		{
			in:  RawMessage{0x40},
			out: `h''`,
		},
		{
			in:  RawMessage{0x41, 0x01},
			out: `h'01'`,
		},
		{
			in:  RawMessage{0x58, 0x01, 0x01},
			out: `h'01'`,
		},
		{
			in:  RawMessage{0x59, 0x00, 0x01, 0x01},
			out: `h'01'`,
		},
		{
			in:  RawMessage{0x5a, 0x00, 0x00, 0x00, 0x01, 0x01},
			out: `h'01'`,
		},
		{
			in:  RawMessage{0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01},
			out: `h'01'`,
		},
		{
			in:  RawMessage{0x5f, 0xff},
			out: `''_`,
		},

		// utf-8 strings
		{
			in:  RawMessage{0x60},
			out: `""`,
		},
		{
			in:  RawMessage{0x61, 0x30},
			out: `"0"`,
		},
		{
			in:  RawMessage{0x78, 0x01, 0x30},
			out: `"0"`,
		},
		{
			in:  RawMessage{0x79, 0x00, 0x01, 0x30},
			out: `"0"`,
		},
		{
			in:  RawMessage{0x7a, 0x00, 0x00, 0x00, 0x01, 0x30},
			out: `"0"`,
		},
		{
			in:  RawMessage{0x7b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x30},
			out: `"0"`,
		},
		{
			in:  RawMessage{0x7f, 0xff},
			out: `""_`,
		},

		// arrays
		{
			in:  RawMessage{0x80},
			out: `[]`,
		},
		{
			in:  RawMessage{0x81, 0x00},
			out: `[0]`,
		},
		{
			in:  RawMessage{0x82, 0x00, 0x01},
			out: `[0, 1]`,
		},
		{
			in:  RawMessage{0x98, 0x02, 0x00, 0x01},
			out: `[0, 1]`,
		},
		{
			in:  RawMessage{0x99, 0x00, 0x02, 0x00, 0x01},
			out: `[0, 1]`,
		},
		{
			in:  RawMessage{0x9a, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01},
			out: `[0, 1]`,
		},
		{
			in:  RawMessage{0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01},
			out: `[0, 1]`,
		},

		// maps
		{
			in:  RawMessage{0xa0},
			out: `{}`,
		},
		{
			in:  RawMessage{0xa1, 0x00, 0x01},
			out: `{0: 1}`,
		},
		{
			in:  RawMessage{0xb8, 0x02, 0x00, 0x01, 0x02, 0x03},
			out: `{0: 1, 2: 3}`,
		},
		{
			in:  RawMessage{0xb9, 0x00, 0x02, 0x00, 0x01, 0x02, 0x03},
			out: `{0: 1, 2: 3}`,
		},
		{
			in:  RawMessage{0xba, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01, 0x02, 0x03},
			out: `{0: 1, 2: 3}`,
		},
		{
			in:  RawMessage{0xbb, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01, 0x02, 0x03},
			out: `{0: 1, 2: 3}`,
		},

		// tags
		{
			in: RawMessage{
				0xC0, // tag(0)
				0x74, // text(20)
				0x32, 0x30, 0x31, 0x33, 0x2D, 0x30, 0x33, 0x2D, 0x32, 0x31, 0x54, 0x32, 0x30, 0x3A, 0x30, 0x34, 0x3A, 0x30, 0x30, 0x5A,
			},
			out: `0("2013-03-21T20:04:00Z")`,
		},
		{
			in: RawMessage{
				0xc1,                   // tag(1)
				0x1a,                   // unsigned
				0x51, 0x4b, 0x67, 0xb0, // 1363896240
			},
			out: `1(1363896240)`,
		},

		// simple values
		{
			in:  RawMessage{0xe0},
			out: `simple(0)`,
		},
		{
			in:  RawMessage{0xf4},
			out: `false`,
		},
		{
			in:  RawMessage{0xf5},
			out: `true`,
		},
		{
			in:  RawMessage{0xf6},
			out: `null`,
		},
		{
			in:  RawMessage{0xf7},
			out: `undefined`,
		},

		// floats
		{
			in:  RawMessage{0xf9, 0x7c, 0x00},
			out: `Infinity`,
		},
		{
			in:  RawMessage{0xf9, 0x7e, 0x00},
			out: `NaN`,
		},
		{
			in:  RawMessage{0xf9, 0xfc, 0x00},
			out: `-Infinity`,
		},
		{
			in:  RawMessage{0xf9, 0x3e, 0x00},
			out: `1.5`,
		},
		{
			in:  RawMessage{0xf9, 0x00, 0x00},
			out: `0.0`,
		},
		{
			in:  RawMessage{0xf9, 0x7b, 0xff},
			out: `65504.0`,
		},
		{
			in:  RawMessage{0xfa, 0x47, 0xc3, 0x50, 0x00},
			out: `100000.0`,
		},
		{
			in:  RawMessage{0xfa, 0x7f, 0x7f, 0xff, 0xff},
			out: `3.4028234663852886e+38`,
		},

		// RFC 8949 Appendix A.
		{
			in:  RawMessage{0x00},
			out: `0`,
		},
		{
			in:  RawMessage{0x01},
			out: `1`,
		},
		{
			in:  RawMessage{0x0a},
			out: `10`,
		},
		{
			in:  RawMessage{0x17},
			out: `23`,
		},
		{
			in:  RawMessage{0x18, 0x18},
			out: `24`,
		},
		{
			in:  RawMessage{0x18, 0x19},
			out: `25`,
		},
		{
			in:  RawMessage{0x18, 0x64},
			out: `100`,
		},
		{
			in:  RawMessage{0x19, 0x03, 0xe8},
			out: `1000`,
		},
		{
			in:  RawMessage{0x1a, 0x00, 0x0f, 0x42, 0x40},
			out: `1000000`,
		},
		{
			in:  RawMessage{0x1b, 0x00, 0x00, 0x00, 0xe8, 0xd4, 0xa5, 0x10, 0x00},
			out: `1000000000000`,
		},
		{
			in:  RawMessage{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			out: `18446744073709551615`,
		},
		// TODO: fix this
		// {
		// 	in:  RawMessage{0xc2, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		// 	out: `18446744073709551616`,
		// },
		{
			in:  RawMessage{0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			out: `-18446744073709551616`,
		},
		// {
		// 	in:  RawMessage{0xc3, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		// 	out: `-18446744073709551617`,
		// },
		{
			in:  RawMessage{0x20},
			out: `-1`,
		},
		{
			in:  RawMessage{0x29},
			out: `-10`,
		},
		{
			in:  RawMessage{0x38, 0x63},
			out: `-100`,
		},
		{
			in:  RawMessage{0x39, 0x03, 0xe7},
			out: `-1000`,
		},
		{
			in:  RawMessage{0xf9, 0x00, 0x00},
			out: `0.0`,
		},
		{
			in:  RawMessage{0xf9, 0x80, 0x00},
			out: `-0.0`,
		},
		{
			in:  RawMessage{0xf9, 0x3c, 0x00},
			out: `1.0`,
		},
		{
			in:  RawMessage{0xfb, 0x3f, 0xf1, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a},
			out: `1.1`,
		},
		{
			in:  RawMessage{0xf9, 0x3e, 0x00},
			out: `1.5`,
		},
		{
			in:  RawMessage{0xfa, 0x47, 0xc3, 0x50, 0x00},
			out: `100000.0`,
		},
		{
			in:  RawMessage{0xfa, 0x7f, 0x7f, 0xff, 0xff},
			out: `3.4028234663852886e+38`,
		},
		{
			in:  RawMessage{0xfb, 0x7e, 0x37, 0xe4, 0x3c, 0x88, 0x00, 0x75, 0x9c},
			out: `1e+300`, // "1.0e+300" in RFC 8949 Appendix A, however "1e+300" is shorter.
		},
		{
			in:  RawMessage{0xf9, 0x00, 0x01},
			out: `5.960464477539063e-08`, // "5.960464477539063e-8" in RFC 8949 Appendix A, it is limitation of strconv package.
		},
		{
			in:  RawMessage{0xf9, 0x04, 0x00},
			out: `6.103515625e-05`, // "0.00006103515625" in RFC 8949 Appendix A, it is limitation of strconv package.
		},
		{
			in:  RawMessage{0xf9, 0xc4, 0x00},
			out: `-4.0`,
		},
		{
			in:  RawMessage{0xfb, 0xc0, 0x10, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66},
			out: `-4.1`,
		},
		{
			in:  RawMessage{0xf9, 0x7c, 0x00},
			out: `Infinity`,
		},
		{
			in:  RawMessage{0xf9, 0x7e, 0x00},
			out: `NaN`,
		},
		{
			in:  RawMessage{0xf9, 0xfc, 0x00},
			out: `-Infinity`,
		},
		{
			in:  RawMessage{0xfa, 0x7f, 0x80, 0x00, 0x00},
			out: `Infinity`,
		},
		{
			in:  RawMessage{0xfa, 0x7f, 0xc0, 0x00, 0x00},
			out: `NaN`,
		},
		{
			in:  RawMessage{0xfa, 0xff, 0x80, 0x00, 0x00},
			out: `-Infinity`,
		},
		{
			in:  RawMessage{0xfb, 0x7f, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			out: `Infinity`,
		},
		{
			in:  RawMessage{0xfb, 0x7f, 0xf8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			out: `NaN`,
		},
		{
			in:  RawMessage{0xfb, 0xff, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			out: `-Infinity`,
		},
		{
			in:  RawMessage{0xf4},
			out: `false`,
		},
		{
			in:  RawMessage{0xf5},
			out: `true`,
		},
		{
			in:  RawMessage{0xf6},
			out: `null`,
		},
		{
			in:  RawMessage{0xf7},
			out: `undefined`,
		},
		{
			in:  RawMessage{0xf0},
			out: `simple(16)`,
		},
		{
			in:  RawMessage{0xf8, 0xff},
			out: `simple(255)`,
		},
		{
			in: RawMessage{
				0xc0,
				0x74,
				0x32, 0x30, 0x31, 0x33, 0x2d, 0x30, 0x33, 0x2d, 0x32, 0x31, 0x54, 0x32, 0x30, 0x3a, 0x30, 0x34, 0x3a, 0x30, 0x30, 0x5a,
			},
			out: `0("2013-03-21T20:04:00Z")`,
		},
		{
			in:  RawMessage{0xc1, 0x1a, 0x51, 0x4b, 0x67, 0xb0},
			out: `1(1363896240)`,
		},
		{
			in:  RawMessage{0xc1, 0xfb, 0x41, 0xd4, 0x52, 0xd9, 0xec, 0x20, 0x00, 0x00},
			out: `1(1.3638962405e+09)`, // "1(1363896240.5)" in RFC 8949 Appendix A, it is limitation of strconv package.
		},
		{
			in:  RawMessage{0xd7, 0x44, 0x01, 0x02, 0x03, 0x04},
			out: `23(h'01020304')`,
		},
		{
			in: RawMessage{
				0xd8, 0x20,
				0x76, 0x68, 0x74, 0x74, 0x70, 0x3a, 0x2f, 0x2f, 0x77, 0x77, 0x77, 0x2e, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2e, 0x63, 0x6f, 0x6d},
			out: `32("http://www.example.com")`,
		},
		{
			in:  RawMessage{0x40},
			out: `h''`,
		},
		{
			in:  RawMessage{0x44, 0x01, 0x02, 0x03, 0x04},
			out: `h'01020304'`,
		},
		{
			in:  RawMessage{0x60},
			out: `""`,
		},
		{
			in:  RawMessage{0x61, 0x61},
			out: `"a"`,
		},
		{
			in:  RawMessage{0x64, 0x49, 0x45, 0x54, 0x46},
			out: `"IETF"`,
		},
		{
			in:  RawMessage{0x62, 0x22, 0x5c},
			out: `"\"\\"`,
		},
		{
			in:  RawMessage{0x62, 0xc3, 0xbc},
			out: `"ü"`, // "\u00fc" in RFC 8949 Appendix A, it is limitation of encoding/json package.
		},
		{
			in:  RawMessage{0x63, 0xe6, 0xb0, 0xb4},
			out: `"水"`, // "\u6c34" in RFC 8949 Appendix A, it is limitation of encoding/json package.
		},
		{
			in:  RawMessage{0x64, 0xf0, 0x90, 0x85, 0x91},
			out: `"𐅑"`, // "\ud800\udd51" in RFC 8949 Appendix A, it is limitation of encoding/json package.
		},
		{
			in:  RawMessage{0x80},
			out: `[]`,
		},
		{
			in:  RawMessage{0x83, 0x01, 0x02, 0x03},
			out: `[1, 2, 3]`,
		},
		{
			in:  RawMessage{0x83, 0x01, 0x82, 0x02, 0x03, 0x82, 0x04, 0x05},
			out: `[1, [2, 3], [4, 5]]`,
		},
		{
			in: RawMessage{
				0x98, 0x19,
				0x01,
				0x02,
				0x03,
				0x04,
				0x05,
				0x06,
				0x07,
				0x08,
				0x09,
				0x0a,
				0x0b,
				0x0c,
				0x0d,
				0x0e,
				0x0f,
				0x10,
				0x11,
				0x12,
				0x13,
				0x14,
				0x15,
				0x16,
				0x17,
				0x18, 0x18,
				0x18, 0x19,
			},
			out: `[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]`,
		},
		{
			in:  RawMessage{0xa0},
			out: `{}`,
		},
		{
			in:  RawMessage{0xa2, 0x01, 0x02, 0x03, 0x04},
			out: `{1: 2, 3: 4}`,
		},
		{
			in:  RawMessage{0xa2, 0x61, 0x61, 0x01, 0x61, 0x62, 0x82, 0x02, 0x03},
			out: `{"a": 1, "b": [2, 3]}`,
		},
		{
			in:  RawMessage{0x82, 0x61, 0x61, 0xa1, 0x61, 0x62, 0x61, 0x63},
			out: `["a", {"b": "c"}]`,
		},
		{
			in: RawMessage{
				0xa5,
				0x61, 0x61, 0x61, 0x41,
				0x61, 0x62, 0x61, 0x42,
				0x61, 0x63, 0x61, 0x43,
				0x61, 0x64, 0x61, 0x44,
				0x61, 0x65, 0x61, 0x45,
			},
			out: `{"a": "A", "b": "B", "c": "C", "d": "D", "e": "E"}`,
		},
		{
			in: RawMessage{
				0x5f,
				0x42, 0x01, 0x02,
				0x43, 0x03, 0x04, 0x05,
				0xff,
			},
			out: `(_ h'0102', h'030405')`,
		},
		{
			in: RawMessage{
				0x7f,
				0x65, 0x73, 0x74, 0x72, 0x65, 0x61,
				0x64, 0x6d, 0x69, 0x6e, 0x67,
				0xff,
			},
			out: `(_ "strea", "ming")`,
		},
		{
			in:  RawMessage{0x9f, 0xff},
			out: `[_ ]`,
		},
		{
			in: RawMessage{
				0x9f,
				0x01,
				0x82, 0x02, 0x03,
				0x9f, 0x04, 0x05, 0xff,
				0xff,
			},
			out: `[_ 1, [2, 3], [_ 4, 5]]`,
		},
		{
			in: RawMessage{
				0x9f,
				0x01,
				0x82, 0x02, 0x03,
				0x82, 0x04, 0x05,
				0xff,
			},
			out: `[_ 1, [2, 3], [4, 5]]`,
		},
		{
			in: RawMessage{
				0x83,
				0x01,
				0x82, 0x02, 0x03,
				0x9f, 0x04, 0x05,
				0xff,
			},
			out: `[1, [2, 3], [_ 4, 5]]`,
		},
		{
			in: RawMessage{
				0x83,
				0x01,
				0x9f, 0x02, 0x03, 0xff,
				0x82, 0x04, 0x05,
			},
			out: `[1, [_ 2, 3], [4, 5]]`,
		},
		{
			in: RawMessage{
				0x9f,
				0x01,
				0x02,
				0x03,
				0x04,
				0x05,
				0x06,
				0x07,
				0x08,
				0x09,
				0x0a,
				0x0b,
				0x0c,
				0x0d,
				0x0e,
				0x0f,
				0x10,
				0x11,
				0x12,
				0x13,
				0x14,
				0x15,
				0x16,
				0x17,
				0x18, 0x18,
				0x18, 0x19,
				0xff,
			},
			out: `[_ 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]`,
		},
		{
			in: RawMessage{
				0xbf,
				0x61, 0x61, 0x01,
				0x61, 0x62, 0x9f, 0x02, 0x03, 0xff,
				0xff,
			},
			out: `{_ "a": 1, "b": [_ 2, 3]}`,
		},
		{
			in: RawMessage{
				0x82,
				0x61, 0x61,
				0xbf, 0x61, 0x62, 0x61, 0x63, 0xff,
			},
			out: `["a", {_ "b": "c"}]`,
		},
		{
			in: RawMessage{
				0xbf,
				0x63, 0x46, 0x75, 0x6e, 0xf5,
				0x63, 0x41, 0x6d, 0x74, 0x21,
				0xff,
			},
			out: `{_ "Fun": true, "Amt": -2}`,
		},
	}

	for _, tt := range tests {
		got, err := tt.in.EncodeEDN()
		if err != nil {
			t.Errorf("EncodeEDN() error = %v", err)
			continue
		}
		if string(got) != tt.out {
			t.Errorf("EncodeEDN(%x) = %s, want %s", []byte(tt.in), got, tt.out)
		}
	}
}
