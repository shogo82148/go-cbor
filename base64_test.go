package cbor

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExpectedBase(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			"base64url byte string",
			ExpectedBase64URL{Content: []byte{0xf0, 0x9f, 0x8d, 0xa3, 0xf0, 0x9f, 0x8d, 0xba}},
			`"8J-No_Cfjbo"`,
		},
		{
			"base64 byte string",
			ExpectedBase64{Content: []byte{0xf0, 0x9f, 0x8d, 0xa3, 0xf0, 0x9f, 0x8d, 0xba}},
			`"8J+No/Cfjbo="`,
		},
		{
			"base16 byte string",
			ExpectedBase16{Content: []byte{0xf0, 0x9f, 0x8d, 0xa3, 0xf0, 0x9f, 0x8d, 0xba}},
			`"f09f8da3f09f8dba"`,
		},

		{
			"text string",
			ExpectedBase16{
				Content: map[string]any{
					"x": "🍣🍺",
				},
			},
			`{"x":"🍣🍺"}`,
		},

		{
			"map",
			ExpectedBase16{
				Content: map[string]any{
					"x": []byte{0x01, 0x02, 0x03, 0x04},
				},
			},
			`{"x":"01020304"}`,
		},

		{
			"array",
			ExpectedBase16{
				Content: []any{
					[]byte{0x01, 0x02, 0x03, 0x04},
				},
			},
			`["01020304"]`,
		},

		{
			"nested map",
			ExpectedBase16{
				Content: map[string]any{
					"base16": []byte{0x01, 0x02, 0x03, 0x04},
					"child": ExpectedBase64{Content: map[string]any{
						"base64": []byte{0x01, 0x02, 0x03, 0x04},
						"base64url": ExpectedBase64URL{
							Content: []byte{0x01, 0x02, 0x03, 0x04},
						},
						"base16": ExpectedBase16{
							Content: []byte{0x01, 0x02, 0x03, 0x04},
						},
					}},
				},
			},
			`{"base16":"01020304","child":{"base16":"01020304","base64":"AQIDBA==","base64url":"AQIDBA"}}`,
		},
	}

	for _, tt := range tests {
		got, err := json.Marshal(tt.input)
		if err != nil {
			t.Error(err)
			continue
		}
		if diff := cmp.Diff(tt.want, string(got)); diff != "" {
			t.Errorf("%s: Marshal() mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}
