package cbor

import (
	"math"
	"testing"
)

// xorshift64 is a pseudo random number generator.
// https://en.wikipedia.org/wiki/Xorshift
type xorshift64 uint64

func newXorshift64() *xorshift64 {
	x := xorshift64(42)
	return &x
}

func (x *xorshift64) Uint64() uint64 {
	a := *x
	a ^= a << 13
	a ^= a >> 7
	a ^= a << 17
	*x = a
	return uint64(a)
}

func TestInteger_Int64(t *testing.T) {
	t.Run("0", func(t *testing.T) {
		i := Integer{Sign: false, Value: 0}
		if v, err := i.Int64(); err != nil || v != 0 {
			t.Errorf("Integer.Int64() = %v, %v, want 0, nil", v, err)
		}
	})

	t.Run("max int64", func(t *testing.T) {
		i := Integer{Sign: false, Value: math.MaxInt64}
		if v, err := i.Int64(); err != nil || v != math.MaxInt64 {
			t.Errorf("Integer.Int64() = %v, %v, want 0, nil", v, err)
		}
	})

	t.Run("max int64 + 1", func(t *testing.T) {
		i := Integer{Sign: false, Value: math.MaxInt64 + 1}
		if v, err := i.Int64(); err == nil {
			t.Errorf("Integer.Int64() = %v, %v, want 0, nil", v, err)
		}
	})

	t.Run("min int64", func(t *testing.T) {
		i := Integer{Sign: true, Value: math.MaxInt64}
		if v, err := i.Int64(); err != nil || v != math.MinInt64 {
			t.Errorf("Integer.Int64() = %v, %v, want 0, nil", v, err)
		}
	})

	t.Run("min int64", func(t *testing.T) {
		i := Integer{Sign: true, Value: math.MaxInt64 + 1}
		if v, err := i.Int64(); err == nil {
			t.Errorf("Integer.Int64() = %v, %v, want 0, nil", v, err)
		}
	})
}

func TestInteger_Uint64(t *testing.T) {
	t.Run("0", func(t *testing.T) {
		i := Integer{Sign: false, Value: 0}
		if v, err := i.Uint64(); err != nil || v != 0 {
			t.Errorf("Integer.Int64() = %v, %v, want 0, nil", v, err)
		}
	})

	t.Run("negative", func(t *testing.T) {
		i := Integer{Sign: true, Value: 0}
		if v, err := i.Uint64(); err == nil {
			t.Errorf("Integer.Int64() = %v, %v, want 0, nil", v, err)
		}
	})

	t.Run("max uint64", func(t *testing.T) {
		i := Integer{Sign: false, Value: math.MaxUint64}
		if v, err := i.Uint64(); err != nil || v != math.MaxUint64 {
			t.Errorf("Integer.Int64() = %v, %v, want 0, nil", v, err)
		}
	})
}

func TestInteger_String(t *testing.T) {
	tests := []struct {
		i Integer
		s string
	}{
		{Integer{Sign: false, Value: 0}, "0"},
		{Integer{Sign: false, Value: math.MaxInt64}, "9223372036854775807"},
		{Integer{Sign: true, Value: 0}, "-1"},
		{Integer{Sign: true, Value: math.MaxInt64 - 1}, "-9223372036854775807"},
		{Integer{Sign: true, Value: math.MaxInt64}, "-9223372036854775808"},
	}

	for _, tt := range tests {
		got := tt.i.String()
		if got != tt.s {
			t.Errorf("Integer.String() = %v, want %v", got, tt.s)
		}
	}
}
