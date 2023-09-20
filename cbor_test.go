package cbor

import (
	"math"
	"math/big"
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
		{Integer{Sign: false, Value: math.MaxUint64}, "18446744073709551615"},
		{Integer{Sign: true, Value: 0}, "-1"},
		{Integer{Sign: true, Value: math.MaxUint64 - 1}, "-18446744073709551615"},
		{Integer{Sign: true, Value: math.MaxUint64}, "-18446744073709551616"},
	}

	for _, tt := range tests {
		got := tt.i.String()
		if got != tt.s {
			t.Errorf("Integer.String() = %v, want %v", got, tt.s)
		}
	}
}

func TestInteger_MarshalJSON(t *testing.T) {
	tests := []struct {
		i    Integer
		want string
	}{
		{Integer{Sign: false, Value: 0}, "0"},
		{Integer{Sign: false, Value: math.MaxUint64}, "18446744073709551615"},
		{Integer{Sign: true, Value: 0}, "-1"},
		{Integer{Sign: true, Value: math.MaxUint64 - 1}, "-18446744073709551615"},
		{Integer{Sign: true, Value: math.MaxUint64}, "-18446744073709551616"},
	}

	for _, tt := range tests {
		got, err := tt.i.MarshalJSON()
		if err != nil {
			t.Errorf("Integer.MarshalJSON() = %v, want nil", err)
		}
		if string(got) != tt.want {
			t.Errorf("Integer.MarshalJSON() = %v, want %v", string(got), tt.want)
		}
	}
}

func TestInteger_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		s    string
		want Integer
	}{
		{"0", Integer{Sign: false, Value: 0}},
		{"-0", Integer{Sign: false, Value: 0}},
		{"+0", Integer{Sign: false, Value: 0}},
		{"1", Integer{Sign: false, Value: 1}},
		{"+1", Integer{Sign: false, Value: 1}},
		{"18446744073709551615", Integer{Sign: false, Value: math.MaxUint64}},
		{"-1", Integer{Sign: true, Value: 0}},
		{"-18446744073709551615", Integer{Sign: true, Value: math.MaxUint64 - 1}},
		{"-18446744073709551616", Integer{Sign: true, Value: math.MaxUint64}},
	}

	for _, tt := range tests {
		var i Integer
		if err := i.UnmarshalJSON([]byte(tt.s)); err != nil {
			t.Errorf("Integer.UnmarshalJSON() = %v, want nil", err)
		}
		if i != tt.want {
			t.Errorf("Integer.UnmarshalJSON() = %v, want %v", i, tt.want)
		}
	}
}

func TestInteger_UnmarshalJSON_error(t *testing.T) {
	tests := []string{
		"",
		"a",
		"-a",
		"18446744073709551616",  // overflow
		"-18446744073709551617", // overflow
	}

	for _, tt := range tests {
		var i Integer
		err := i.UnmarshalJSON([]byte(tt))
		if err == nil {
			t.Errorf("Integer.UnmarshalJSON() = nil, want error")
		}
	}
}

func TestInteger_BigInt(t *testing.T) {
	tests := []struct {
		i    Integer
		want *big.Int
	}{
		{Integer{Sign: false, Value: 0}, newBigInt("0")},
		{Integer{Sign: false, Value: math.MaxUint64}, newBigInt("18446744073709551615")},
		{Integer{Sign: true, Value: 0}, newBigInt("-1")},
		{Integer{Sign: true, Value: math.MaxUint64 - 1}, newBigInt("-18446744073709551615")},
		{Integer{Sign: true, Value: math.MaxUint64}, newBigInt("-18446744073709551616")},
	}

	for _, tt := range tests {
		got := tt.i.BigInt()
		if got.Cmp(tt.want) != 0 {
			t.Errorf("Integer.BigInt() = %v, want %v", got, tt.want)
		}
	}
}
