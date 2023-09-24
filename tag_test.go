package cbor

import (
	"math/big"
	"reflect"
	"testing"
)

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
