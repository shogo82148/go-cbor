package cbor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"reflect"
	"slices"
	"sync"
)

type CBORMarshaler interface {
	// MarshalCBOR returns the CBOR encoding of the receiver.
	MarshalCBOR() ([]byte, error)
}

type majorType byte

const (
	majorTypePositiveInt majorType = 0
	majorTypeNegativeInt majorType = 1
	majorTypeBytes       majorType = 2
	majorTypeString      majorType = 3
	majorTypeArray       majorType = 4
	majorTypeMap         majorType = 5
	majorTypeTag         majorType = 6
	majorTypeOther       majorType = 7
)

func Marshal(v any) ([]byte, error) {
	e := newEncodeState()
	err := e.encode(v)
	if err != nil {
		return nil, err
	}
	return e.buf.Bytes(), nil
}

func newEncodeState() *encodeState {
	return &encodeState{}
}

type encodeState struct {
	buf bytes.Buffer
}

func (s *encodeState) encode(v any) error {
	// fast path for basic types
	switch v := v.(type) {
	case int8:
		return s.encodeInt(int64(v))
	case int16:
		return s.encodeInt(int64(v))
	case int32:
		return s.encodeInt(int64(v))
	case int64:
		return s.encodeInt(v)
	case int:
		return s.encodeInt(int64(v))
	case uint8:
		return s.encodeUint(uint64(v))
	case uint16:
		return s.encodeUint(uint64(v))
	case uint32:
		return s.encodeUint(uint64(v))
	case uintptr:
		return s.encodeUint(uint64(v))
	case uint:
		return s.encodeUint(uint64(v))
	case uint64:
		return s.encodeUint(v)
	case float32:
		return s.encodeFloat64(float64(v))
	case float64:
		return s.encodeFloat64(v)
	case bool:
		return s.encodeBool(v)
	case nil:
		return s.encodeNull()
	case []byte:
		return s.encodeBytes(v)
	case string:
		return s.encodeString(v)
	case CBORMarshaler:
		data, err := v.MarshalCBOR()
		if err != nil {
			return err
		}
		s.buf.Write(data)
		return nil
	}

	return s.encodeReflectValue(reflect.ValueOf(v))
}

func (s *encodeState) encodeReflectValue(v reflect.Value) error {
	if !v.IsValid() {
		return s.encodeNull()
	}
	return typeEncoder(v.Type())(s, v)
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

type encoderFunc func(e *encodeState, v reflect.Value) error

var encoderCache sync.Map // map[reflect.Type]encoderFunc

func typeEncoder(t reflect.Type) encoderFunc {
	if fi, ok := encoderCache.Load(t); ok {
		return fi.(encoderFunc)
	}

	// To deal with recursive types, populate the map with an
	// indirect func before we build it. This type waits on the
	// real func (f) to be ready and then calls it. This indirect
	// func is only used for recursive types.
	var (
		wg sync.WaitGroup
		f  encoderFunc
	)
	wg.Add(1)
	fi, loaded := encoderCache.LoadOrStore(t, encoderFunc(func(e *encodeState, v reflect.Value) error {
		wg.Wait()
		return f(e, v)
	}))
	if loaded {
		return fi.(encoderFunc)
	}

	// Compute the real encoder and replace the indirect func with it.
	f = newTypeEncoder(t)
	wg.Done()
	return f
}

func newTypeEncoder(t reflect.Type) encoderFunc {
	switch t {
	case bigIntType:
		return bigIntEncoder
	case tagType:
		return tagEncoder
	case simpleType:
		return simpleEncoder
	case undefinedType:
		return undefinedEncoder
	}

	switch t.Kind() {
	case reflect.Bool:
		return boolEncoder
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return uintEncoder
	case reflect.Float32, reflect.Float64:
		return floatEncoder
	case reflect.String:
		return stringEncoder
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return bytesEncoder
		}
		return sliceEncoder
	case reflect.Array:
		return sliceEncoder
	case reflect.Map:
		return mapEncoder
	case reflect.Interface:
		return interfaceEncoder
	case reflect.Ptr:
		return newPtrEncoder(t)
	case reflect.Struct:
		return newStructEncoder(t)
	}
	panic("TODO implement")
}

func boolEncoder(e *encodeState, v reflect.Value) error {
	return e.encodeBool(v.Bool())
}

func intEncoder(e *encodeState, v reflect.Value) error {
	return e.encodeInt(v.Int())
}

func uintEncoder(e *encodeState, v reflect.Value) error {
	return e.encodeUint(v.Uint())
}

func floatEncoder(e *encodeState, v reflect.Value) error {
	return e.encodeFloat64(v.Float())
}

func stringEncoder(e *encodeState, v reflect.Value) error {
	return e.encodeString(v.String())
}

func bytesEncoder(e *encodeState, v reflect.Value) error {
	return e.encodeBytes(v.Bytes())
}

func bigIntEncoder(e *encodeState, v reflect.Value) error {
	i := v.Interface().(*big.Int)
	if i.Sign() >= 0 {
		e.writeByte(0xc2) // tag 2: positive bigint
		return e.encodeBytes(i.Bytes())
	} else {
		e.writeByte(0xc3) // tag 3: negative bigint
		x := big.NewInt(-1)
		x.Sub(x, i)
		return e.encodeBytes(x.Bytes())
	}
}

func tagEncoder(e *encodeState, v reflect.Value) error {
	tag := v.Interface().(Tag)
	switch {
	case tag.Number < 24:
		e.writeByte(byte(0xc0 + tag.Number))
	case tag.Number < 0x100:
		e.writeByte(0xd8)
		e.writeByte(byte(tag.Number))
	case tag.Number < 0x10000:
		e.writeByte(0xd9)
		e.writeUint16(uint16(tag.Number))
	case tag.Number < 0x100000000:
		e.writeByte(0xda)
		e.writeUint32(uint32(tag.Number))
	default:
		e.writeByte(0xdb)
		e.writeUint64(uint64(tag.Number))
	}
	return e.encode(tag.Content)
}

func simpleEncoder(e *encodeState, v reflect.Value) error {
	s := v.Uint()
	switch {
	case s < 24:
		e.writeByte(0xe0 | byte(s))
	case s < 32:
		return errors.New("cbor: reserved simple value")
	default:
		e.writeByte(0xf8) // simple value
		e.writeByte(byte(s))
	}
	return nil

}

func undefinedEncoder(e *encodeState, v reflect.Value) error {
	e.writeByte(0xf7)
	return nil
}

func sliceEncoder(e *encodeState, v reflect.Value) error {
	l := v.Len()
	switch {
	case l < 0x17:
		e.writeByte(byte(0x80 + l))
	case l < 0x100:
		e.writeByte(0x98)
		e.writeByte(byte(l))
	case int64(l) < 0x10000:
		e.writeByte(0x99)
		e.writeUint16(uint16(l))
	case int64(l) < 0x100000000:
		e.writeByte(0x9a)
		e.writeUint32(uint32(l))
	default:
		e.writeByte(0x9b)
		e.writeUint64(uint64(l))
	}
	for i := 0; i < l; i++ {
		err := e.encode(v.Index(i).Interface())
		if err != nil {
			return err
		}
	}
	return nil
}

type mapKey struct {
	key     reflect.Value
	encoded []byte
}

func cmpMapKey(a, b mapKey) int {
	return bytes.Compare(a.encoded, b.encoded)
}

func mapEncoder(s *encodeState, v reflect.Value) error {
	l := v.Len()
	keys := make([]mapKey, 0, l)
	for _, key := range v.MapKeys() {
		encoded, err := Marshal(key.Interface())
		if err != nil {
			return err
		}
		keys = append(keys, mapKey{key, encoded})
	}
	slices.SortFunc(keys, cmpMapKey)

	// encode the length
	s.writeUint(majorTypeMap, uint64(l))
	for _, key := range keys {
		s.buf.Write(key.encoded)
		value := v.MapIndex(key.key)
		if err := s.encodeReflectValue(value); err != nil {
			return err
		}
	}
	return nil
}

func interfaceEncoder(s *encodeState, v reflect.Value) error {
	if v.IsNil() {
		return s.encodeNull()
	}
	return s.encodeReflectValue(v.Elem())
}

type ptrEncoder struct {
	elemEnc encoderFunc
}

func (pe ptrEncoder) encode(e *encodeState, v reflect.Value) error {
	if v.IsNil() {
		return e.encodeNull()
	}
	// TODO: check for circular references
	return pe.elemEnc(e, v.Elem())
}

func newPtrEncoder(t reflect.Type) encoderFunc {
	enc := ptrEncoder{typeEncoder(t.Elem())}
	return enc.encode
}

type structEncoder struct {
	st *structType
}

func (se structEncoder) encodeAsMap(e *encodeState, v reflect.Value) error {
	// count number of fields to encode
	var l int
	for _, f := range se.st.fields {
		fv := v.FieldByIndex(f.index)
		if f.omitempty && isEmptyValue(fv) {
			continue
		}
		l++
	}

	e.writeUint(majorTypeMap, uint64(l))
	for _, f := range se.st.fields {
		fv := v.FieldByIndex(f.index)
		if f.omitempty && isEmptyValue(fv) {
			continue
		}
		e.buf.Write(f.encodedKey)
		if err := e.encodeReflectValue(fv); err != nil {
			return err
		}
	}
	return nil
}

func (se structEncoder) encodeAsArray(e *encodeState, v reflect.Value) error {
	e.writeUint(majorTypeArray, uint64(len(se.st.fields)))
	for _, f := range se.st.fields {
		fv := v.FieldByIndex(f.index)
		if err := e.encodeReflectValue(fv); err != nil {
			return err
		}
	}
	return nil
}

func newStructEncoder(t reflect.Type) encoderFunc {
	st := cachedStructType(t)
	se := structEncoder{st}
	if st.toArray {
		return se.encodeAsArray
	} else {
		return se.encodeAsMap
	}
}

func (s *encodeState) writeByte(v byte) {
	s.buf.WriteByte(v)
}

func (s *encodeState) writeUint16(v uint16) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	s.buf.Write(buf[:])
}

func (s *encodeState) writeUint32(v uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	s.buf.Write(buf[:])
}

func (s *encodeState) writeUint64(v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	s.buf.Write(buf[:])
}

func (s *encodeState) writeUint(major majorType, v uint64) {
	bits := byte(major) << 5
	switch {
	case v < 24:
		s.writeByte(bits | byte(v))
	case v < 0x100:
		s.writeByte(bits | 24)
		s.writeByte(byte(v))
	case v < 0x10000:
		s.writeByte(bits | 25)
		s.writeUint16(uint16(v))
	case v < 0x100000000:
		s.writeByte(bits | 26)
		s.writeUint32(uint32(v))
	default:
		s.writeByte(bits | 27)
		s.writeUint64(uint64(v))
	}
}

func (s *encodeState) encodeInt(v int64) error {
	ui := uint64(v >> 63)
	typ := majorType(ui) & majorTypeNegativeInt
	ui ^= uint64(v)
	s.writeUint(typ, ui)
	return nil
}

func (s *encodeState) encodeUint(v uint64) error {
	s.writeUint(majorTypePositiveInt, uint64(v))
	return nil
}

func (s *encodeState) encodeFloat64(v float64) error {
	f64 := math.Float64bits(v)
	sign := f64 >> 63
	exp := int((f64>>52)&0x7ff) - 1023
	frac := f64 & 0xfffffffffffff

	if exp == -1023 && frac == 0 {
		// 0.0 in float16
		s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
		s.writeByte(byte(sign << 7))
		s.writeByte(0x00)
		return nil
	}
	if exp == 1024 {
		if frac == 0 {
			// inf in float16
			s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
			s.writeByte(byte(sign<<7 | 0x7c))
			s.writeByte(0x00)
			return nil
		} else if frac&0x8000000000000 != 0 {
			// NaN in float16
			s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
			f16 := uint16(0x7e00)
			s.writeUint16(f16)
			return nil
		}
	}

	// try converting to subnormal float16
	if -24 <= exp && exp < -14 {
		shift := -exp + 53 - 24 - 1
		if frac&((1<<shift)-1) == 0 {
			frac |= 1 << 52
			f16 := uint16(sign<<15 | frac>>shift)
			s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
			s.writeUint16(f16)
			return nil
		}
	}

	// try converting to normal float16
	if -14 <= exp && exp <= 15 {
		if frac&((1<<42)-1) == 0 {
			f16 := uint16(sign<<15 | uint64(exp+15)<<10 | frac>>42)
			s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
			s.writeUint16(f16)
			return nil
		}
	}

	// try converting to subnormal float32
	if -149 <= exp && exp < -126 {
		shift := -exp + 53 - 149 - 1
		if frac&((1<<shift)-1) == 0 {
			frac |= 1 << 52
			f32 := uint32(sign<<31 | frac>>shift)
			s.writeByte(0xfa) // single-precision float (four-byte IEEE 754)
			s.writeUint32(f32)
			return nil
		}
	}

	// try converting to normal float32
	if -126 <= exp && exp <= 127 {
		if frac&((1<<29)-1) == 0 {
			f32 := uint32(sign<<31 | uint64(exp+127)<<23 | frac>>29)
			s.writeByte(0xfa) // single-precision float (four-byte IEEE 754)
			s.writeUint32(f32)
			return nil
		}
	}

	// default to float64
	s.writeByte(0xfb) // double-precision float (eight-byte IEEE 754)
	s.writeUint64(f64)
	return nil
}

func (s *encodeState) encodeBool(v bool) error {
	if v {
		s.writeByte(0xf5)
	} else {
		s.writeByte(0xf4)
	}
	return nil
}

func (s *encodeState) encodeNull() error {
	s.writeByte(0xf6) // null
	return nil
}

func (s *encodeState) encodeBytes(v []byte) error {
	l := len(v)
	s.writeUint(majorTypeBytes, uint64(l))
	s.buf.Write(v)
	return nil
}

func (s *encodeState) encodeString(v string) error {
	l := len(v)
	s.writeUint(majorTypeString, uint64(l))
	s.buf.WriteString(v)
	return nil
}
