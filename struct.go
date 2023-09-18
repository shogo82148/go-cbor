package cbor

import (
	"reflect"
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
}
type field struct {
	encodedKey []byte
	omitempty  bool
	index      []int
}

func newStructType(t reflect.Type) *structType {
	var toArray bool
	fields := []field{}
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

		var encodedKey []byte
		if keyasint {
			key, err := strconv.ParseInt(name, 10, 64)
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
			encodedKey, err = Marshal(name)
			if err != nil {
				// TODO: return error
				panic(err)
			}
		}

		fields = append(fields, field{
			encodedKey: encodedKey,
			omitempty:  omitempty,
			index:      f.Index,
		})
	}
	return &structType{
		toArray: toArray,
		fields:  fields,
	}
}
