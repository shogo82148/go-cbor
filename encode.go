package cbor

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"
)

type CBORMarshaler interface {
	// MarshalCBOR returns the CBOR encoding of the receiver.
	MarshalCBOR() ([]byte, error)
}

// An UnsupportedTypeError is returned by Marshal when attempting
// to encode an unsupported value type.
type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "cbor: unsupported type: " + e.Type.String()
}

// An UnsupportedValueError is returned by Marshal when attempting
// to encode an unsupported value.
type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

func (e *UnsupportedValueError) Error() string {
	return "cbor: unsupported value: " + e.Str
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
	return &encodeState{
		ptrSeen: make(map[any]struct{}),
	}
}

type encodeState struct {
	buf bytes.Buffer

	// Keep track of what pointers we've seen in the current recursive call
	// path, to avoid cycles that could lead to a stack overflow. Only do
	// the relatively expensive map operations if ptrLevel is larger than
	// startDetectingCyclesAfter, so that we skip the work if we're within a
	// reasonable amount of nested pointers deep.
	ptrLevel uint
	ptrSeen  map[any]struct{}
}

const startDetectingCyclesAfter = 1000

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
	case bigFloatType:
		return bigFloatEncoder
	case tagType:
		return tagEncoder
	case simpleType:
		return simpleEncoder
	case undefinedType:
		return undefinedEncoder
	case integerType:
		return integerEncoder
	case timeType:
		return timeEncoder
	case urlType:
		return urlEncoder
	case base64StringType:
		return newBase64Encoder(tagNumberBase64, base64.StdEncoding.Strict())
	case base64URLStringType:
		return newBase64Encoder(tagNumberBase64URL, base64.RawURLEncoding.Strict())
	case encodedData:
		// TODO: implement
	case expectedBase16Type:
		// TODO: implement
	case expectedBase64Type:
		// TODO: implement
	case expectedBase64URLType:
		// TODO: implement
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
	default:
		return unsupportedTypeEncoder
	}
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

func integerEncoder(e *encodeState, v reflect.Value) error {
	i := v.Interface().(Integer)
	if i.Sign {
		e.writeUint(majorTypeNegativeInt, i.Value)
	} else {
		e.writeUint(majorTypePositiveInt, i.Value)
	}
	return nil
}

var minInteger *big.Int

func init() {
	minInteger = new(big.Int)
	if _, ok := minInteger.SetString("-18446744073709551616", 10); !ok {
		panic("failed to parse minInteger")
	}
}

func bigIntEncoder(e *encodeState, v reflect.Value) error {
	i := v.Addr().Interface().(*big.Int)
	return e.encodeBigInt(i)
}

func bigFloatEncoder(e *encodeState, v reflect.Value) error {
	// breaks into exponent and mantissa
	f := v.Addr().Interface().(*big.Float)
	return e.encodeBigFloat(f)
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

func timeEncoder(e *encodeState, v reflect.Value) error {
	e.writeByte(0xc1) // tag 1: epoch-based date/time
	t := v.Interface().(time.Time)
	epoch := float64(t.UnixNano()) / 1e9
	return e.encodeFloat64(epoch)
}

func urlEncoder(e *encodeState, v reflect.Value) error {
	u := v.Addr().Interface().(*url.URL)
	s := u.String()

	// write tag number 32: URI
	e.writeByte(0xd8)
	e.writeByte(byte(tagNumberURI))

	e.writeUint(majorTypeString, uint64(len(s)))
	e.buf.WriteString(s)
	return nil
}

func newBase64Encoder(n TagNumber, enc *base64.Encoding) encoderFunc {
	return func(e *encodeState, v reflect.Value) error {
		// validate that the value is a base64 encoded string.
		data := v.String()
		if _, err := enc.DecodeString(data); err != nil {
			return wrapSemanticError("cbor: invalid base64 encoding", err)
		}

		// write tag number
		e.writeByte(0xd8)
		e.writeByte(byte(n))

		// write data
		e.writeUint(majorTypeString, uint64(len(data)))
		e.buf.WriteString(data)
		return nil
	}
}

func undefinedEncoder(e *encodeState, v reflect.Value) error {
	e.writeByte(0xf7)
	return nil
}

func sliceEncoder(e *encodeState, v reflect.Value) error {
	if v.IsZero() {
		return e.encodeNull()
	}

	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		// We're a large number of nested ptrEncoder.encode calls deep;
		// start checking if we've run into a pointer cycle.
		ptr := v.UnsafePointer()
		if _, ok := e.ptrSeen[ptr]; ok {
			return &UnsupportedValueError{v, fmt.Sprintf("encountered a cycle via %s", v.Type())}
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
	}

	l := v.Len()
	e.writeUint(majorTypeArray, uint64(l))
	for i := 0; i < l; i++ {
		err := e.encode(v.Index(i).Interface())
		if err != nil {
			return err
		}
	}
	e.ptrLevel--
	return nil
}

type mapKey struct {
	key     reflect.Value
	encoded []byte
}

func cmpMapKey(a, b mapKey) int {
	return bytes.Compare(a.encoded, b.encoded)
}

func mapEncoder(e *encodeState, v reflect.Value) error {
	if v.IsZero() {
		return e.encodeNull()
	}

	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		// We're a large number of nested ptrEncoder.encode calls deep;
		// start checking if we've run into a pointer cycle.
		ptr := v.UnsafePointer()
		if _, ok := e.ptrSeen[ptr]; ok {
			return &UnsupportedValueError{v, fmt.Sprintf("encountered a cycle via %s", v.Type())}
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
	}

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
	e.writeUint(majorTypeMap, uint64(l))

	for _, key := range keys {
		e.buf.Write(key.encoded)
		value := v.MapIndex(key.key)
		if err := e.encodeReflectValue(value); err != nil {
			return err
		}
	}
	e.ptrLevel--
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

	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		// We're a large number of nested ptrEncoder.encode calls deep;
		// start checking if we've run into a pointer cycle.
		ptr := v.Interface()
		if _, ok := e.ptrSeen[ptr]; ok {
			return &UnsupportedValueError{v, fmt.Sprintf("encountered a cycle via %s", v.Type())}
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
	}

	err := pe.elemEnc(e, v.Elem())
	e.ptrLevel--
	return err
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

func unsupportedTypeEncoder(e *encodeState, v reflect.Value) error {
	return &UnsupportedTypeError{v.Type()}
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
		} else if frac != 0 {
			// NaN in float16
			// we don't support NaN payloads or signaling NaNs.
			s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
			s.writeUint16(0x7e00)
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

func (e *encodeState) encodeBytes(v []byte) error {
	l := len(v)
	e.writeUint(majorTypeBytes, uint64(l))
	e.buf.Write(v)
	return nil
}

func (e *encodeState) encodeString(v string) error {
	s := strings.ToValidUTF8(v, "\ufffd")
	e.writeUint(majorTypeString, uint64(len(s)))
	e.buf.WriteString(s)
	return nil
}

func (e *encodeState) encodeBigInt(i *big.Int) error {
	// encode as int if possible
	if i.IsUint64() {
		return e.encodeUint(i.Uint64())
	}
	if i.IsInt64() {
		return e.encodeInt(i.Int64())
	}
	if i.Cmp(minInteger) == 0 {
		e.writeUint(majorTypeNegativeInt, 1<<64-1)
		return nil
	}

	// encode as bigint
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

func (e *encodeState) encodeBigFloat(f *big.Float) error {
	// encode as float if possible
	f64, acc := f.Float64()
	if acc == big.Exact {
		return e.encodeFloat64(f64)
	}

	mant := new(big.Float)
	exp := f.MantExp(mant)

	// convert mantissa to integer
	prec := mant.MinPrec()
	n, _ := mant.SetMantExp(mant, int(prec)).Int(new(big.Int))

	e.writeByte(0xc5) // tag 5: Bigfloat
	e.writeByte(0x82) // array of length 2

	// encode exponent
	if err := e.encodeInt(int64(exp) + int64(prec) - 1); err != nil {
		return err
	}

	// encode mantissa
	return e.encodeBigInt(n)
}
