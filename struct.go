package cbor

import (
	"bytes"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
)

var structTypeCache sync.Map // map[reflect.Type]*structType

func cachedStructType(t reflect.Type) *structType {
	if st, ok := structTypeCache.Load(t); ok {
		return st.(*structType)
	}
	st := newStructType(t)
	structTypeCache.Store(t, st)
	return st
}

type structType struct {
	toArray bool
	fields  []field
	maps    map[any]*field
}

type field struct {
	name       string
	key        any
	encodedKey []byte
	omitempty  bool
	index      []int
}

func cmpFieldKey(a, b field) int {
	return bytes.Compare(a.encodedKey, b.encodedKey)
}

func newStructType(t reflect.Type) *structType {
	var toArray bool
	fields := make([]field, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("cbor")
		if tag == "-" {
			continue
		}

		// parse tag
		var omitempty bool
		var keyasint bool
		name, tag, _ := strings.Cut(tag, ",")
		for tag != "" {
			var opt string
			opt, tag, _ = strings.Cut(tag, ",")
			switch opt {
			case "omitempty":
				omitempty = true
			case "keyasint":
				keyasint = true
			case "toarray":
				if f.Name == "_" {
					toArray = true
				}
			}
		}

		if !f.IsExported() {
			continue
		}

		var key any
		var encodedKey []byte
		if keyasint {
			var err error
			key, err = strconv.ParseInt(name, 10, 64)
			if err != nil {
				// TODO: return error
				panic(err)
			}
			encodedKey, err = Marshal(key)
			if err != nil {
				// TODO: return error
				panic(err)
			}
		} else {
			var err error
			if name == "" {
				name = f.Name
			}
			key = name
			encodedKey, err = Marshal(name)
			if err != nil {
				// TODO: return error
				panic(err)
			}
		}

		fields = append(fields, field{
			name:       f.Name,
			key:        key,
			encodedKey: encodedKey,
			omitempty:  omitempty,
			index:      f.Index,
		})
	}

	// sort fields by encodedKey
	if !toArray {
		slices.SortStableFunc(fields, cmpFieldKey)
	}

	// build maps
	maps := make(map[any]*field)
	for i := range fields {
		maps[fields[i].key] = &fields[i]
	}

	return &structType{
		toArray: toArray,
		fields:  fields,
		maps:    maps,
	}
}
