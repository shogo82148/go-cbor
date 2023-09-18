package cbor

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var streamTest = []any{
	0.1,
	"hello",
	nil,
	true,
	false,
	[]any{"a", "b", "c"},
	map[string]any{"K": "Kelvin", "ß": "long s"},
	3.14,
}

var streamEncoded = [][]byte{
	{0xfb, 0x3f, 0xb9, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a},
	{0xfb, 0x3f, 0xb9, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a, 0x65, 0x68, 0x65, 0x6c, 0x6c, 0x6f},
	{0xfb, 0x3f, 0xb9, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a, 0x65, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0xf6},
	{0xfb, 0x3f, 0xb9, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a, 0x65, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0xf6, 0xf5},
	{0xfb, 0x3f, 0xb9, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a, 0x65, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0xf6, 0xf5, 0xf4},
	{0xfb, 0x3f, 0xb9, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a, 0x65, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0xf6, 0xf5, 0xf4, 0x83, 0x61, 0x61, 0x61, 0x62, 0x61, 0x63},
	{0xfb, 0x3f, 0xb9, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a, 0x65, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0xf6, 0xf5, 0xf4, 0x83, 0x61, 0x61, 0x61, 0x62, 0x61, 0x63, 0xa2, 0x61, 0x4b, 0x66, 0x4b, 0x65, 0x6c, 0x76, 0x69, 0x6e, 0x62, 0xc3, 0x9f, 0x66, 0x6c, 0x6f, 0x6e, 0x67, 0x20, 0x73},
	{0xfb, 0x3f, 0xb9, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a, 0x65, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0xf6, 0xf5, 0xf4, 0x83, 0x61, 0x61, 0x61, 0x62, 0x61, 0x63, 0xa2, 0x61, 0x4b, 0x66, 0x4b, 0x65, 0x6c, 0x76, 0x69, 0x6e, 0x62, 0xc3, 0x9f, 0x66, 0x6c, 0x6f, 0x6e, 0x67, 0x20, 0x73, 0xfb, 0x40, 0x09, 0x1e, 0xb8, 0x51, 0xeb, 0x85, 0x1f},
}

func TestEncoder(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	for i := 0; i < len(streamTest); i++ {
		if err := enc.Encode(streamTest[i]); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(buf.Bytes(), streamEncoded[i]); diff != "" {
			t.Errorf("Encode() mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestDecoder(t *testing.T) {
	for i := 0; i < len(streamEncoded); i++ {
		r := bytes.NewReader(streamEncoded[i])

		dec := NewDecoder(r)
		ret := []any{}
		for {
			var v any
			if err := dec.Decode(&v); err != nil {
				break
			}
			ret = append(ret, v)
		}
		if diff := cmp.Diff(streamTest[:i+1], ret); diff != "" {
			t.Errorf("Decode() mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestDecoder_UserAnyKey(t *testing.T) {
	input := []byte{0xa2, 0x01, 0x02, 0x03, 0x04}
	want := map[any]any{int64(1): int64(2), int64(3): int64(4)}

	r := bytes.NewReader(input)
	dec := NewDecoder(r)
	dec.d.useAnyKey = true
	var got any
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Decode() mismatch (-want +got):\n%s", diff)
	}
}