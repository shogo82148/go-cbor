package cbor

import (
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestRawTag(t *testing.T) {
	t.Run("decode into RawTag", func(t *testing.T) {
		input := []byte{0xd9, 0xff, 0xff, 0x00}
		var got RawTag
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error: %v", err)
		}
		want := RawTag{0xffff, []byte{0x00}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("decode into any", func(t *testing.T) {
		input := []byte{0xd9, 0xff, 0xff, 0x00}
		var got any
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error: %v", err)
		}
		want := RawTag{0xffff, []byte{0x00}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("decode into any", func(t *testing.T) {
		input := []byte{0xd9, 0xff, 0xff, 0x00}
		var got someInterface
		err := Unmarshal(input, &got)
		if err == nil {
			t.Errorf("Unmarshal() error = nil, want error")
		}
	})

	t.Run("encode RawTag", func(t *testing.T) {
		input := RawTag{0xffff, []byte{0x00}}
		b, err := Marshal(input)
		if err != nil {
			t.Errorf("Marshal() error: %v", err)
		}
		want := []byte{0xd9, 0xff, 0xff, 0x00}
		if diff := cmp.Diff(want, b); diff != "" {
			t.Errorf("Marshal() mismatch (-want +got):\n%s", diff)
		}
	})
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

		testUnexpectedEnd(t, input)
	})

	t.Run("negative", func(t *testing.T) {
		input := []byte{0xc3, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		var got *big.Int
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := newBigInt("-18446744073709551617")
		if got.Cmp(want) != 0 {
			t.Errorf("Unmarshal() = %x, want %x", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("minus one", func(t *testing.T) {
		input := []byte{0xc3, 0x40}
		var got any
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := any(int64(-1))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Unmarshal() = %v, want %v", got, want)
		}
		testUnexpectedEnd(t, input)
	})
}

func TestUnmarshal_BigFloat(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		// RFC 8949 Section 3.4.4.
		input := []byte{
			0xc5, // Tag 5
			0x82, // Array of length 2
			0x20, // -1
			0x03, // 3
		}
		var got *big.Float
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := newBigFloat("1.5")
		if got.Cmp(want) != 0 {
			t.Errorf("Unmarshal() = %x, want %x", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("decode to any", func(t *testing.T) {
		// RFC 8949 Section 3.4.4.
		input := []byte{
			0xc5, // Tag 5
			0x82, // Array of length 2
			0x20, // -1
			0x03, // 3
		}
		var v any
		if err := Unmarshal(input, &v); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		got, ok := v.(float64)
		if !ok {
			t.Errorf("Unmarshal() = %T, want float64", v)
			return
		}
		want := 1.5
		if got != want {
			t.Errorf("Unmarshal() = %x, want %x", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("long length of array", func(t *testing.T) {
		input := []byte{
			0xc5, // Tag 5
			0x83, // Array of length 3
			0x20, // -1
			0x03, // 3
			0x04, // 4
		}
		var v any
		if err := Unmarshal(input, &v); err == nil {
			t.Errorf("Unmarshal() error = nil, want error")
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("short length of array", func(t *testing.T) {
		input := []byte{
			0xc5, // Tag 5
			0x80, // Array of length 0
		}
		var v any
		if err := Unmarshal(input, &v); err == nil {
			t.Errorf("Unmarshal() error = nil, want error")
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("invalid type of exponential", func(t *testing.T) {
		input := []byte{
			0xc5, // Tag 5
			0x82, // Array of length 2

			// bigint 18446744073709551616
			0xc2, 0x49,
			0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

			0x03, // 3
		}
		var v any
		if err := Unmarshal(input, &v); err == nil {
			t.Errorf("Unmarshal() error = nil, want error")
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("invalid type of mant", func(t *testing.T) {
		input := []byte{
			0xc5, // Tag 5
			0x82, // Array of length 2
			0x20, // -1
			0x80, // []
		}
		var v any
		if err := Unmarshal(input, &v); err == nil {
			t.Errorf("Unmarshal() error = nil, want error")
		}

		testUnexpectedEnd(t, input)
	})
}

func TestUnmarshal_Time(t *testing.T) {
	t.Run("rfc3339", func(t *testing.T) {
		input := []byte{0xc0, 0x74, 0x32, 0x30, 0x31, 0x33, 0x2d, 0x30, 0x33, 0x2d, 0x32, 0x31, 0x54, 0x32, 0x30, 0x3a, 0x30, 0x34, 0x3a, 0x30, 0x30, 0x5a}
		var got time.Time
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("Unmarshal() = %v, want %v", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("rfc3339 to any", func(t *testing.T) {
		input := []byte{0xc0, 0x74, 0x32, 0x30, 0x31, 0x33, 0x2d, 0x30, 0x33, 0x2d, 0x32, 0x31, 0x54, 0x32, 0x30, 0x3a, 0x30, 0x34, 0x3a, 0x30, 0x30, 0x5a}
		var got any
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		tt, ok := got.(time.Time)
		if !ok {
			t.Fatal("got is not a time.Time")
		}
		want := time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC)
		if !tt.Equal(want) {
			t.Errorf("Unmarshal() = %v, want %v", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("rfc3339 to interface", func(t *testing.T) {
		input := []byte{0xc0, 0x74, 0x32, 0x30, 0x31, 0x33, 0x2d, 0x30, 0x33, 0x2d, 0x32, 0x31, 0x54, 0x32, 0x30, 0x3a, 0x30, 0x34, 0x3a, 0x30, 0x30, 0x5a}
		var got interface {
			Unix() int64
		}
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		tt, ok := got.(time.Time)
		if !ok {
			t.Fatal("got is not a time.Time")
		}
		want := time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC)
		if !tt.Equal(want) {
			t.Errorf("Unmarshal() = %v, want %v", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("integer epoch", func(t *testing.T) {
		input := []byte{0xc1, 0x1a, 0x51, 0x4b, 0x67, 0xb0}
		var got time.Time
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := time.Unix(1363896240, 0)
		if !got.Equal(want) {
			t.Errorf("Unmarshal() = %v, want %v", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("float epoch", func(t *testing.T) {
		input := []byte{0xc1, 0xfb, 0x41, 0xd4, 0x52, 0xd9, 0xec, 0x20, 0x00, 0x00}
		var got time.Time
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := time.Unix(1363896240, 500000000)
		if !got.Equal(want) {
			t.Errorf("Unmarshal() = %v, want %v", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	// https://github.com/shogo82148/go-cbor/pull/67
	t.Run("float epoch type error", func(t *testing.T) {
		input := []byte{
			0xc1,                         // tag 1
			0x44, 0x30, 0x30, 0x30, 0x30, // []byte("0000")
		}
		var got time.Time
		err := Unmarshal(input, &got)
		se, ok := err.(*SemanticError)
		if !ok {
			t.Errorf("Unmarshal() error = %v, want SemanticError", err)
			return
		}
		if se.msg != "cbor: invalid epoch-based datetime" {
			t.Errorf("unexpected error message: %q", se.msg)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("null", func(t *testing.T) {
		input := []byte{0xf6}
		got := time.Now()
		err := Unmarshal(input, &got)
		if err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		if !got.IsZero() {
			t.Errorf("Unmarshal() = %v, want zero time", got)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("undefined", func(t *testing.T) {
		input := []byte{0xf7}
		got := time.Now()
		err := Unmarshal(input, &got)
		if err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		if !got.IsZero() {
			t.Errorf("Unmarshal() = %v, want zero time", got)
		}

		testUnexpectedEnd(t, input)
	})
}
