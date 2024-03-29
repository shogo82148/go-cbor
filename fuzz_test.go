package cbor

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"reflect"
	"testing"
	"time"
	"unicode/utf8"

	gocmp "github.com/google/go-cmp/cmp"
)

func FuzzUnmarshal(f *testing.F) {
	for _, tt := range unmarshalTests {
		f.Add(tt.data)
	}
	for _, tt := range notWellFormed {
		f.Add(tt)
	}

	types := []func() any{
		func() any { return new(any) },
		func() any { return new(map[string]any) },
		func() any { return new(map[any]any) },
		func() any { return new([]any) },
	}

	f.Fuzz(func(t *testing.T, a []byte) {
		for _, newv := range types {
			v := newv()
			if err := Unmarshal(a, v); err != nil {
				return
			}

			b, err := Marshal(v)
			if err != nil {
				t.Error(err)
			}

			w := newv()
			if err := Unmarshal(b, w); err != nil {
				t.Error(err)
			}
			if !deepEqualLite(v, w) {
				v0, err := json.Marshal(v)
				if err != nil {
					v0 = []byte("ERR! " + err.Error())
				}
				w0, err := json.Marshal(w)
				if err != nil {
					w0 = []byte("ERR! " + err.Error())
				}
				t.Errorf("%x -> %x: Unmarshal() mismatch: %s != %s", a, b, v0, w0)
			}

			c, err := Marshal(w)
			if err != nil {
				t.Error(err)
			}
			if diff := gocmp.Diff(b, c); diff != "" {
				t.Errorf("%x -> %x: Marshal() mismatch (-want +got):\n%s", a, b, diff)
			}
		}
	})
}

func FuzzDecode(f *testing.F) {
	for _, tt := range unmarshalTests {
		f.Add(tt.data)
	}
	for _, tt := range notWellFormed {
		f.Add(tt)
	}

	types := []func() any{
		func() any { return new(any) },
		func() any { return new(map[string]any) },
		func() any { return new(map[any]any) },
		func() any { return new([]any) },
	}
	opts := []func(dec *Decoder){
		func(dec *Decoder) {},
		func(dec *Decoder) { dec.UseAnyKey() },
		func(dec *Decoder) { dec.UseInteger() },
		func(dec *Decoder) { dec.UseInteger(); dec.UseAnyKey() },
	}

	f.Fuzz(func(t *testing.T, a []byte) {
		for _, newv := range types {
			for _, opt := range opts {
				r := bytes.NewReader(a)
				dec := NewDecoder(r)
				opt(dec)
				v := newv()
				if err := dec.Decode(v); err != nil {
					return
				}

				b, err := Marshal(v)
				if err != nil {
					t.Error(err)
				}

				r = bytes.NewReader(a)
				dec = NewDecoder(r)
				opt(dec)
				w := newv()
				if err := dec.Decode(w); err != nil {
					t.Error(err)
				}
				if !deepEqualLite(v, w) {
					v0, err := json.Marshal(v)
					if err != nil {
						v0 = []byte("ERR! " + err.Error())
					}
					w0, err := json.Marshal(w)
					if err != nil {
						w0 = []byte("ERR! " + err.Error())
					}
					t.Errorf("%x: Unmarshal() mismatch: %s != %s", a, v0, w0)
				}

				c, err := Marshal(w)
				if err != nil {
					t.Error(err)
				}
				if diff := gocmp.Diff(b, c); diff != "" {
					t.Errorf("%x: Marshal() mismatch (-want +got):\n%s", a, diff)
				}
			}
		}
	})
}

func FuzzMarshal_string(f *testing.F) {
	f.Add("")
	f.Add("\uFDDD")
	f.Add("a\xffb")
	f.Add("a\xffb\uFFFD")
	f.Add("a☺\xffb☺\xC0\xAFc☺\xff")
	f.Add("\xC0\xAF")
	f.Add("\xE0\x80\xAF")
	f.Add("\xed\xa0\x80")
	f.Add("\xed\xbf\xbf")
	f.Add("\xF0\x80\x80\xaf")
	f.Add("\xF8\x80\x80\x80\xAF")
	f.Add("\xFC\x80\x80\x80\x80\xAF")

	f.Fuzz(func(t *testing.T, s string) {
		data, err := Marshal(s)
		if err != nil {
			t.Error(err)
		}

		var v string
		if err := Unmarshal(data, &v); err != nil {
			t.Error(err)
		}

		// Marshal always encodes strings as valid UTF-8 strings.
		if !utf8.ValidString(v) {
			t.Errorf("%q: invalid UTF-8 string: %q", s, v)
		}
	})
}

// deepEqualLite compares two values without considering NaNs.
// We can't use reflect.DeepEqual because NaN != NaN.
func deepEqualLite(x, y any) bool {
	if x == nil || y == nil {
		return x == y
	}
	rx := reflect.ValueOf(x)
	ry := reflect.ValueOf(y)
	if rx.Type() != ry.Type() {
		return false
	}
	return deepEqualRV(rx, ry)
}

func deepEqualRV(rx, ry reflect.Value) bool {
	if !rx.IsValid() || !ry.IsValid() {
		return rx.IsValid() == ry.IsValid()
	}
	if rx.Type() != ry.Type() {
		return false
	}

	switch rx.Type() {
	case timeType:
		x := rx.Interface().(time.Time)
		y := ry.Interface().(time.Time)
		return x.Equal(y)
	case urlType:
		x := rx.Addr().Interface().(*url.URL)
		y := ry.Addr().Interface().(*url.URL)
		return x.String() == y.String()
	case bigFloatType:
		x := rx.Addr().Interface().(*big.Float)
		y := ry.Addr().Interface().(*big.Float)
		return x.Cmp(y) == 0
	case bigIntType:
		x := rx.Addr().Interface().(*big.Int)
		y := ry.Addr().Interface().(*big.Int)
		return x.Cmp(y) == 0
	}

	switch rx.Kind() {
	case reflect.Float32, reflect.Float64:
		// we can't use == operator because NaN != NaN
		return cmp.Compare(rx.Float(), ry.Float()) == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rx.Int() == ry.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rx.Uint() == ry.Uint()
	case reflect.String:
		return rx.String() == ry.String()
	case reflect.Bool:
		return rx.Bool() == ry.Bool()
	case reflect.Slice:
		if rx.IsNil() != ry.IsNil() {
			return false
		}
		if rx.Len() != ry.Len() {
			return false
		}
		for i := 0; i < rx.Len(); i++ {
			if !deepEqualRV(rx.Index(i), ry.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Array:
		if rx.Len() != ry.Len() {
			return false
		}
		for i := 0; i < rx.Len(); i++ {
			if !deepEqualRV(rx.Index(i), ry.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if rx.IsNil() != ry.IsNil() {
			return false
		}
		if rx.Len() != ry.Len() {
			return false
		}
		for _, k := range rx.MapKeys() {
			v1 := rx.MapIndex(k)
			v2 := ry.MapIndex(k)
			if !v1.IsValid() || !v2.IsValid() || !deepEqualRV(v1, v2) {
				return false
			}
		}
		return true
	case reflect.Pointer:
		return deepEqualRV(rx.Elem(), ry.Elem())
	case reflect.Struct:
		for i := 0; i < rx.NumField(); i++ {
			if !deepEqualRV(rx.Field(i), ry.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Interface:
		if rx.IsNil() != ry.IsNil() {
			return false
		}
		return deepEqualRV(rx.Elem(), ry.Elem())
	}
	panic(fmt.Sprintf("never reach: %v and %v", rx, ry))
}
