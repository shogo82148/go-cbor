package cbor

import (
	"math"
	"math/big"
	"net/netip"
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
	t.Run("encode into *big.Int", func(t *testing.T) {
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

	t.Run("decode MaxInt64", func(t *testing.T) {
		input := []byte{0x1b, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
		var got any
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := int64(math.MaxInt64)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("(-want/+got): %s", diff)
		}
		testUnexpectedEnd(t, input)
	})

	t.Run("decode MaxInt64+1", func(t *testing.T) {
		input := []byte{0x1b, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		var got any
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := newBigInt("9223372036854775808")
		if got, ok := got.(*big.Int); !ok || got.Cmp(want) != 0 {
			t.Errorf("Unmarshal() = %x, want %x", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("decode MaxUint64", func(t *testing.T) {
		input := []byte{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
		var got any
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := newBigInt("18446744073709551615")
		if got, ok := got.(*big.Int); !ok || got.Cmp(want) != 0 {
			t.Errorf("Unmarshal() = %x, want %x", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("decode MinInt64", func(t *testing.T) {
		input := []byte{0x3b, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
		var got any
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := any(int64(math.MinInt64))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Unmarshal() = %v, want %v", got, want)
		}
		testUnexpectedEnd(t, input)
	})

	t.Run("decode MinInt64-1", func(t *testing.T) {
		input := []byte{0x3b, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		var got any
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := newBigInt("-9223372036854775809")
		if got, ok := got.(*big.Int); !ok || got.Cmp(want) != 0 {
			t.Errorf("Unmarshal() = %x, want %x", got, want)
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

	t.Run("issue 107 decode", func(t *testing.T) {
		input := []byte{
			0xc5,             // Tag 5
			0x82,             // Array of length 2
			0x39, 0x30, 0x30, // -12337
			0x30, // -17
		}
		var got *big.Float
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := newBigFloat("-0x11p-12337")
		if got.Cmp(want) != 0 {
			t.Errorf("Unmarshal() = %x, want %x", got, want)
		}

		testUnexpectedEnd(t, input)
	})

	t.Run("issue 107 encode", func(t *testing.T) {
		input := newBigFloat("-0x11p-12337")
		got, err := Marshal(input)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
		}
		want := []byte{
			0xc5,             // Tag 5
			0x82,             // Array of length 2
			0x39, 0x30, 0x30, // -12337
			0x30, // -17
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("(-want/+got): %s", diff)
		}
	})

	t.Run("issue 109 decode", func(t *testing.T) {
		input := []byte{
			0xc5, // Tag 5
			0x82, // Array of length 2
			0x30, // -17
			0xc2, // Tag 2
			0x49, // Byte string of length 9
			0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30,
		}
		var got *big.Float
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := newBigFloatWithPrec("0x303030303030303030p-17", 9*8)
		if got.Cmp(want) != 0 {
			t.Errorf("Unmarshal() = %.18x, want %.18x", got, want)
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

func TestUnmarshal_EncodedData(t *testing.T) {
	t.Run("decode undefined", func(t *testing.T) {
		input := []byte{0xd8, 0x18, 0xf7}
		var got EncodedData
		err := Unmarshal(input, &got)
		se, ok := err.(*SemanticError)
		if !ok {
			t.Errorf("Unmarshal() error = %v, want SemanticError", err)
			return
		}
		if se.msg != "cbor: invalid encoded data" {
			t.Errorf("unexpected error message: %q", se.msg)
		}
	})
}

func TestUnmarshal_IPv4(t *testing.T) {
	t.Run("decode address to any", func(t *testing.T) {
		input := []byte{
			0xd8, 0x34, // tag 52
			0x44, 0xc0, 0x00, 0x02, 0x01, // []byte{192, 0, 2, 1}
		}
		var got any
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := netip.AddrFrom4([4]byte{192, 0, 2, 1})
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Unmarshal() = %v, want %v", got, want)
		}
	})

	t.Run("decode address to netip.Addr", func(t *testing.T) {
		input := []byte{
			0xd8, 0x34, // tag 52
			0x44, 0xc0, 0x00, 0x02, 0x01, // []byte{192, 0, 2, 1}
		}
		var got netip.Addr
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := netip.AddrFrom4([4]byte{192, 0, 2, 1})
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Unmarshal() = %v, want %v", got, want)
		}
	})

	t.Run("decode address to int", func(t *testing.T) {
		input := []byte{
			0xd8, 0x34, // tag 52
			0x44, 0xc0, 0x00, 0x02, 0x01, // []byte{192, 0, 2, 1}
		}
		var got int
		err := Unmarshal(input, &got)
		if err == nil {
			t.Fatal("Unmarshal() error = nil, want error")
		}
		_, ok := err.(*UnmarshalTypeError)
		if !ok {
			t.Fatal("Unmarshal() error is not UnmarshalTypeError")
		}
	})

	t.Run("decode prefix to any", func(t *testing.T) {
		input := []byte{
			0xd8, 0x34, // tag 52
			0x82,       // array of length 2
			0x18, 0x18, // 24
			0x43, 0xc0, 0x00, 0x02, // []byte{192, 0, 2}
		}
		var got any
		if err := Unmarshal(input, &got); err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}
		want := netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 0, 2, 0}), 24)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Unmarshal() = %v, want %v", got, want)
		}
	})

	t.Run("prefix ends with zero", func(t *testing.T) {
		input := []byte{
			0xd8, 0x34, // tag 52
			0x82,       // array of length 2
			0x18, 0x18, // 24
			0x43, 0xc0, 0x00, 0x00, // []byte{192, 0, 0}
		}
		var got any
		err := Unmarshal(input, &got)
		if err == nil {
			t.Fatal("Unmarshal() error = nil, want error")
		}
		_, ok := err.(*SemanticError)
		if !ok {
			t.Fatal("Unmarshal() error is not SemanticError")
		}
	})
}
