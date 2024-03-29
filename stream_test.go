package cbor

import (
	"bytes"
	"errors"
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

func TestDecoder_UseAnyKey(t *testing.T) {
	t.Run("number key", func(t *testing.T) {
		input := []byte{0xa2, 0x01, 0x02, 0x03, 0x04}
		want := map[any]any{int64(1): int64(2), int64(3): int64(4)}

		r := bytes.NewReader(input)
		dec := NewDecoder(r)
		dec.UseAnyKey()
		var got any
		if err := dec.Decode(&got); err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Decode() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("various type keys", func(t *testing.T) {
		input := []byte{
			0xa8,       // 8 items map
			0x0a, 0x01, // 10
			0x18, 0x64, 0x02, // 100
			0x20, 0x03, // -1
			0x61, 0x7a, 0x04, // "z"
			0x62, 0x61, 0x61, 0x05, // "aa"
			0x81, 0x18, 0x64, 0x06, // [100]
			0x81, 0x20, 0x07, // [-1]
			0xf4, 0x08, // false
		}
		want := map[any]any{
			int64(10):          int64(1),
			int64(100):         int64(2),
			int64(-1):          int64(3),
			"z":                int64(4),
			"aa":               int64(5),
			[1]any{int64(100)}: int64(6),
			[1]any{int64(-1)}:  int64(7),
			false:              int64(8),
		}

		r := bytes.NewReader(input)
		dec := NewDecoder(r)
		dec.UseAnyKey()
		var got any
		if err := dec.Decode(&got); err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Decode() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("byte strings keys", func(t *testing.T) {
		input := []byte{
			0xa1,                         // one element map
			0x44, 0x01, 0x02, 0x03, 0x04, // h'01020304'
			0x01, // 1
		}
		want := map[any]any{
			[4]byte{0x01, 0x02, 0x03, 0x04}: int64(1),
		}

		r := bytes.NewReader(input)
		dec := NewDecoder(r)
		dec.UseAnyKey()
		var got any
		if err := dec.Decode(&got); err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Decode() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("map map keys", func(t *testing.T) {
		input := []byte{
			0xa1,                         // one element map
			0xa1,                         // one element map
			0x44, 0x01, 0x02, 0x03, 0x04, // h'01020304'
			0x01, // 1
			0x02, // 2
		}

		r := bytes.NewReader(input)
		dec := NewDecoder(r)
		dec.UseAnyKey()
		var got any
		err := dec.Decode(&got)
		if err == nil {
			t.Error("Decode() should return error")
		}
		var se *SyntaxError
		if !errors.As(err, &se) {
			t.Errorf("Decode() should return SyntaxError, got %T", err)
		}
	})
}

func TestDecoder_UseInteger(t *testing.T) {
	t.Run("integer", func(t *testing.T) {
		input := []byte{0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
		want := Integer{Sign: true, Value: 18446744073709551615}

		r := bytes.NewReader(input)
		dec := NewDecoder(r)
		dec.UseInteger()
		var got any
		if err := dec.Decode(&got); err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Decode() mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestDecoder_SemanticError(t *testing.T) {
	t.Run("duplicated map key decoded to any", func(t *testing.T) {
		data := []byte{
			0xa2,             // map of length 2
			0x61, 0x30, 0x02, // "0": 2
			0x61, 0x30, 0x03, // "0": 3
		}
		r := bytes.NewReader(data)

		var v any
		dec := NewDecoder(r)
		dec.UseAnyKey()
		err := dec.Decode(&v)
		_, ok := err.(*SemanticError)
		if !ok {
			t.Errorf("Unmarshal() error = %v, want *SemanticError", err)
		}
	})

	t.Run("duplicated map key decoded to struct", func(t *testing.T) {
		data := []byte{
			0xa2,             // map of length 2
			0x61, 0x30, 0x02, // "0": 2
			0x61, 0x30, 0x03, // "0": 3
		}
		r := bytes.NewReader(data)

		var v struct {
			A int `cbor:"0"`
		}
		dec := NewDecoder(r)
		dec.UseAnyKey()
		err := dec.Decode(&v)
		_, ok := err.(*SemanticError)
		if !ok {
			t.Errorf("Unmarshal() error = %v, want *SemanticError", err)
		}
	})

	t.Run("duplicated indefinite-length map key decoded to any", func(t *testing.T) {
		data := []byte{
			0xbf,             // indefinite-length
			0x61, 0x30, 0x02, // "0": 2
			0x61, 0x30, 0x03, // "0": 3
			0xff, // break
		}
		r := bytes.NewReader(data)

		var v any
		dec := NewDecoder(r)
		dec.UseAnyKey()
		err := dec.Decode(&v)
		_, ok := err.(*SemanticError)
		if !ok {
			t.Errorf("Unmarshal() error = %v, want *SemanticError", err)
		}
	})
}
