package cbor

import (
	"cmp"
	"fmt"
	"reflect"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
)

func FuzzUnmarshal(f *testing.F) {
	for _, tt := range unmarshalTests {
		f.Add(tt.data)
	}
	for _, tt := range notWellFormed {
		f.Add(tt)
	}

	f.Fuzz(func(t *testing.T, a []byte) {
		var v any
		if err := Unmarshal(a, &v); err != nil {
			return
		}

		b, err := Marshal(v)
		if err != nil {
			t.Error(err)
		}

		var w any
		if err := Unmarshal(b, &w); err != nil {
			t.Error(err)
		}
		if !deepEqualLite(v, w) {
			t.Errorf("Unmarshal() mismatch: %#v != %#v", v, w)
		}
		// if diff := cmp.Diff(v, w, cmpopts.EquateNaNs()); diff != "" {
		// 	t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
		// }

		c, err := Marshal(w)
		if err != nil {
			t.Error(err)
		}
		if diff := gocmp.Diff(b, c); diff != "" {
			t.Errorf("Marshal() mismatch (-want +got):\n%s", diff)
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
