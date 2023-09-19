package cbor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"math/big"
	"math/bits"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Unmarshaler is the interface implemented by types that can unmarshal a CBOR description of themselves.
// The input can be assumed to be a valid encoding of a CBOR value.
// UnmarshalCBOR must copy the CBOR data if it wishes to retain the data after returning.
type Unmarshaler interface {
	UnmarshalCBOR([]byte) error
}

// An UnmarshalTypeError describes a CBOR value that was
// not appropriate for a value of a specific Go type.
type UnmarshalTypeError struct {
	Value  string       // description of CBOR value - "bool", "array", "number -5"
	Type   reflect.Type // type of Go value it could not be assigned to
	Offset int64        // error occurred after reading Offset bytes
	Struct string       // name of the struct type containing the field
	Field  string       // the full path from root node to the field
}

func (e *UnmarshalTypeError) Error() string {
	if e.Struct != "" || e.Field != "" {
		return "cbor: cannot unmarshal " + e.Value + " into Go struct field " + e.Struct + "." + e.Field + " of type " + e.Type.String()
	}
	return "cbor: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
}

// An InvalidUnmarshalError describes an invalid argument passed to [Unmarshal].
// (The argument to [Unmarshal] must be a non-nil pointer.)
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "cbor: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Pointer {
		return "cbor: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "cbor: Unmarshal(nil " + e.Type.String() + ")"
}

// Unmarshal parses the CBOR-encoded data and stores the result in the value pointed to by v.
func Unmarshal(data []byte, v any) error {
	d := newDecodeState(data)

	// Check for well-formedness.
	// Avoids filling out half a data structure
	// before discovering a JSON syntax error.
	if err := d.checkWellFormed(); err != nil {
		return err
	}

	d.init(data)
	if err := d.decode(v); err != nil {
		return err
	}
	if d.savedError != nil {
		return d.savedError
	}
	return nil
}

func newDecodeState(data []byte) *decodeState {
	d := new(decodeState)
	d.init(data)
	return d
}

// An errorContext provides context for type errors during decoding.
type errorContext struct {
	Struct     reflect.Type
	FieldStack []string
}

type decodeState struct {
	data         []byte
	off          int // next read offset
	savedError   error
	decodingKeys bool // whether we're decoding a map key (as opposed to a map value)
	errorContext *errorContext

	useAnyKey  bool
	useInteger bool
}

func (d *decodeState) init(data []byte) {
	d.data = data
	d.off = 0
	d.savedError = nil
	if d.errorContext != nil {
		d.errorContext.Struct = nil
		// Reuse the allocated space for the FieldStack slice.
		d.errorContext.FieldStack = d.errorContext.FieldStack[:0]
	}
	d.decodingKeys = false
}

func (s *decodeState) readByte() (byte, error) {
	if !s.isAvailable(1) {
		return 0, s.newSyntaxError("cbor: unexpected end")
	}
	b := s.data[s.off]
	s.off++
	return b, nil
}

func (s *decodeState) peekByte() (byte, error) {
	if !s.isAvailable(1) {
		return 0, s.newSyntaxError("cbor: unexpected end")
	}
	return s.data[s.off], nil
}

func (s *decodeState) readUint16() (uint16, error) {
	if !s.isAvailable(2) {
		return 0, s.newSyntaxError("cbor: unexpected end")
	}
	b := binary.BigEndian.Uint16(s.data[s.off:])
	s.off += 2
	return b, nil
}

func (s *decodeState) readUint32() (uint32, error) {
	if !s.isAvailable(4) {
		return 0, s.newSyntaxError("cbor: unexpected end")
	}
	b := binary.BigEndian.Uint32(s.data[s.off:])
	s.off += 4
	return b, nil
}

func (s *decodeState) readUint64() (uint64, error) {
	if !s.isAvailable(8) {
		return 0, s.newSyntaxError("cbor: unexpected end")
	}
	b := binary.BigEndian.Uint64(s.data[s.off:])
	s.off += 8
	return b, nil
}

// isAvailable reports whether n bytes are available.
func (d *decodeState) isAvailable(n uint64) bool {
	if n > math.MaxInt {
		// int(n) will overflow
		return false
	}
	newOffset := d.off + int(n)
	if newOffset < d.off {
		// overflow
		return false
	}
	return newOffset <= len(d.data)
}

func (d *decodeState) saveError(err error) {
	if d.savedError == nil {
		d.savedError = d.addErrorContext(err)
	}
}

func (d *decodeState) addErrorContext(err error) error {
	if ctx := d.errorContext; ctx != nil && (ctx.Struct != nil || len(ctx.FieldStack) > 0) {
		switch err := err.(type) {
		case *UnmarshalTypeError:
			err.Struct = ctx.Struct.Name()
			err.Field = strings.Join(ctx.FieldStack, ".")
		}
	}
	return err
}

// indirect walks down v allocating pointers as needed,
// until it gets to a non-pointer.
// If it encounters an Unmarshaler, indirect stops and returns that.
// If decodingNull is true, indirect stops at the first settable pointer so it
// can be set to nil.
//
// based on https://github.com/golang/go/blob/561a5079057e3a660ab638e1ba957a96c4ff3fd1/src/encoding/json/decode.go#L421-L497
func indirect(v reflect.Value, decodingNull bool) (Unmarshaler, reflect.Value) {
	v0 := v
	haveAddr := false

	// If v is a named type and is addressable,
	// start with its address, so that if the type has pointer methods,
	// we find them.
	if v.Kind() != reflect.Pointer && v.Type().Name() != "" && v.CanAddr() {
		haveAddr = true
		v = v.Addr()
	}
	for {
		// Load value from interface, but only if the result will be
		// usefully addressable.
		if v.Kind() == reflect.Interface && !v.IsNil() {
			e := v.Elem()
			if e.Kind() == reflect.Pointer && !e.IsNil() && (!decodingNull || e.Elem().Kind() == reflect.Pointer) {
				haveAddr = false
				v = e
				continue
			}
		}

		if v.Kind() != reflect.Pointer {
			break
		}

		if decodingNull && v.CanSet() {
			break
		}

		// Prevent infinite loop if v is an interface pointing to its own address:
		//     var v interface{}
		//     v = &v
		if v.Elem().Kind() == reflect.Interface && v.Elem().Elem() == v {
			v = v.Elem()
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if v.Type().NumMethod() > 0 && v.CanInterface() {
			if u, ok := v.Interface().(Unmarshaler); ok {
				return u, reflect.Value{}
			}
		}

		if haveAddr {
			v = v0 // restore original value after round-trip Value.Addr().Elem()
			haveAddr = false
		} else {
			v = v.Elem()
		}
	}
	return nil, v
}

func (d *decodeState) decode(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}
	return d.decodeReflectValue(rv)
}

func (d *decodeState) decodeReflectValue(v reflect.Value) error {
	start := d.off // mark position in data so we can rewind in case of error

	typ, err := d.readByte()
	if err != nil {
		return err
	}

	isNull := typ == 0xf6 || typ == 0xf7 // null or undefined
	u, v := indirect(v, isNull)

	switch typ {
	// unsigned integer 0x00..0x17 (0..23)
	case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17:
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodePositiveInt(start, uint64(typ), v)

	// unsigned integer (one-byte uint8_t follows)
	case 0x18:
		w, err := d.readByte()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodePositiveInt(start, uint64(w), v)

	// unsigned integer (two-byte uint16_t follows)
	case 0x19:
		w, err := d.readUint16()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodePositiveInt(start, uint64(w), v)

	// unsigned integer (four-byte uint32_t follows)
	case 0x1a:
		w, err := d.readUint32()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodePositiveInt(start, uint64(w), v)

	// unsigned integer (eight-byte uint64_t follows)
	case 0x1b:
		w, err := d.readUint64()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodePositiveInt(start, w, v)

	// negative integer -1-0x00..-1-0x17 (-1..-24)
	case 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37:
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodeNegativeInt(start, uint64(typ-0x20), v)

	// negative integer -1-n (one-byte uint8_t for n follows)
	case 0x38:
		w, err := d.readByte()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodeNegativeInt(start, uint64(w), v)

	// negative integer -1-n (two-byte uint16_t for n follows)
	case 0x39:
		w, err := d.readUint16()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodeNegativeInt(start, uint64(w), v)

	// negative integer -1-n (four-byte uint32_t for n follows)
	case 0x3a:
		w, err := d.readUint32()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodeNegativeInt(start, uint64(w), v)

	// negative integer -1-n (eight-byte uint64_t for n follows)
	case 0x3b:
		w, err := d.readUint64()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodeNegativeInt(start, w, v)

		// byte string (0x00..0x17 bytes follow)
	case 0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f, 0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57:
		n := uint64(typ - 0x40)
		err := d.decodeBytes(start, n, u, v)
		if err != nil {
			return err
		}
		return nil

	// byte string (one-byte uint8_t for n, and then n bytes follow)
	case 0x58:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		return d.decodeBytes(start, uint64(n), u, v)

	// byte string (two-byte uint16_t for n, and then n bytes follow)
	case 0x59:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		return d.decodeBytes(start, uint64(n), u, v)

	// byte string (four-byte uint32_t for n, and then n bytes follow)
	case 0x5a:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		return d.decodeBytes(start, uint64(n), u, v)

	// byte string (eight-byte uint64_t for n, and then n bytes follow)
	case 0x5b:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		return d.decodeBytes(start, n, u, v)

	// byte string (indefinite length)
	case 0x5f:
		return d.decodeBytesIndefinite(start, u, v)

	// UTF-8 string (0x00..0x17 bytes follow)
	case 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77:
		n := uint64(typ - 0x60)
		return d.decodeString(start, n, u, v)

	// UTF-8 string (one-byte uint8_t for n, and then n bytes follow)
	case 0x78:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		return d.decodeString(start, uint64(n), u, v)

	// UTF-8 string (two-byte uint16_t for n, and then n bytes follow)
	case 0x79:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		return d.decodeString(start, uint64(n), u, v)

	// UTF-8 string (four-byte uint32_t for n, and then n bytes follow)
	case 0x7a:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		return d.decodeString(start, uint64(n), u, v)

	// UTF-8 string (eight-byte uint64_t for n, and then n bytes follow)
	case 0x7b:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		return d.decodeString(start, n, u, v)

	// UTF-8 string (indefinite length)
	case 0x7f:
		return d.decodeStringIndefinite(start, u, v)

		// array (0x00..0x17 data items follow)
	case 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f, 0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97:
		n := int(typ - 0x80)
		return d.decodeArray(start, uint64(n), u, v)

	// array (one-byte uint8_t for n, and then n data items follow)
	case 0x98:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		return d.decodeArray(start, uint64(n), u, v)

	// array (two-byte uint16_t for n, and then n data items follow)
	case 0x99:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		return d.decodeArray(start, uint64(n), u, v)

	// array (four-byte uint32_t for n, and then n data items follow)
	case 0x9a:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		return d.decodeArray(start, uint64(n), u, v)

	// array (eight-byte uint64_t for n, and then n data items follow)
	case 0x9b:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		return d.decodeArray(start, n, u, v)

	// array (indefinite length)
	case 0x9f:
		return d.decodeArrayIndefinite(start, u, v)

	// map (0x00..0x17 pairs of data items follow)
	case 0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf, 0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7:
		n := int(typ - 0xa0)
		return d.decodeMap(start, uint64(n), u, v)

	// map (one-byte uint8_t for n, and then n pairs of data items follow)
	case 0xb8:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		return d.decodeMap(start, uint64(n), u, v)

	// map (two-byte uint16_t for n, and then n pairs of data items follow)
	case 0xb9:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		return d.decodeMap(start, uint64(n), u, v)

	// map (four-byte uint32_t for n, and then n pairs of data items follow)
	case 0xba:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		return d.decodeMap(start, uint64(n), u, v)

	// map (eight-byte uint64_t for n, and then n pairs of data items follow)
	case 0xbb:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		return d.decodeMap(start, n, u, v)

	// map (indefinite length)
	case 0xbf:
		return d.decodeMapIndefinite(start, u, v)

	// tags
	case 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf, 0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7:
		n := TagNumber(typ - 0xc0)
		return d.decodeTag(start, n, u, v)

	// tag 0: Standard date/time string
	case 0xc0:
		if u != nil || v.Type() == tagType {
			n := TagNumber(typ - 0xc0)
			return d.decodeTag(start, n, u, v)
		}
		var s string
		if err := d.decode(&s); err != nil {
			return err
		}
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return err
		}
		return d.setAny(start, "time", t, v)

	// tag 1: Epoch-based date/time
	case 0xc1:
		if u != nil || v.Type() == tagType {
			n := TagNumber(typ - 0xc0)
			return d.decodeTag(start, n, u, v)
		}
		var epoch any
		if err := d.decode(&epoch); err != nil {
			return err
		}

		var t time.Time
		switch epoch := epoch.(type) {
		case int64:
			t = time.Unix(epoch, 0)
		case float64:
			i, f := math.Modf(epoch)
			t = time.Unix(int64(i), int64(f*1e9))
		}
		return d.setAny(start, "time", t, v)

	// tag 2: Unsigned bignum
	case 0xc2:
		if u != nil || v.Type() == tagType {
			n := TagNumber(typ - 0xc0)
			return d.decodeTag(start, n, u, v)
		}
		var b []byte
		if err := d.decode(&b); err != nil {
			return err
		}
		switch v.Type() {
		case bigIntType:
			i := v.Addr().Interface().(*big.Int)
			i.SetBytes(b)
		}
		return nil

	// tag 3: Negative bignum
	case 0xc3:
		if u != nil || v.Type() == tagType {
			n := TagNumber(typ - 0xc0)
			return d.decodeTag(start, n, u, v)
		}
		var b []byte
		if err := d.decode(&b); err != nil {
			return err
		}
		switch v.Type() {
		case bigIntType:
			i := v.Addr().Interface().(*big.Int)
			i.SetBytes(b)
			i.Sub(minusOne, i)
		}
		return nil

	// tag 4: Decimal fraction
	case 0xc4:
		if u != nil || v.Type() == tagType {
			n := TagNumber(typ - 0xc0)
			return d.decodeTag(start, n, u, v)
		}

		return errors.New("TODO: implement")

	// tag 5: Bigfloat
	case 0xc5:
		if u != nil || v.Type() == tagType {
			n := TagNumber(typ - 0xc0)
			return d.decodeTag(start, n, u, v)
		}
		d.decodeBigFloat(start, v)

	case 0xd8:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		return d.decodeTag(start, TagNumber(n), u, v)

	case 0xd9:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		return d.decodeTag(start, TagNumber(n), u, v)

	case 0xda:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		return d.decodeTag(start, TagNumber(n), u, v)

	case 0xdb:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		return d.decodeTag(start, TagNumber(n), u, v)

	// simple values
	case 0xe0, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef, 0xf0, 0xf3:
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.setSimple(start, Simple(typ-0xe0), v)

	// false
	case 0xf4:
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.setBool(start, false, v)

	// true
	case 0xf5:
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.setBool(start, true, v)

	// null
	case 0xf6:
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.setNull(start, v)

	// undefined
	case 0xf7:
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.setUndefined(start, v)

	// simple value (one-byte uint8_t follows)
	case 0xf8:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.setSimple(start, Simple(n), v)

	// half-precision float (two-byte IEEE 754)
	case 0xf9:
		w, err := d.readUint16()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodeFloat16(start, uint16(w), v)

	// single-precision float (four-byte IEEE 754)
	case 0xfa:
		w, err := d.readUint32()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodeFloat32(start, uint32(w), v)

	// double-precision float (eight-byte IEEE 754)
	case 0xfb:
		w, err := d.readUint64()
		if err != nil {
			return err
		}
		if u != nil {
			return u.UnmarshalCBOR(d.data[start:d.off])
		}
		return d.decodeFloat64(start, uint64(w), v)
	}
	return nil
}

func (d *decodeState) setAny(start int, value string, w any, v reflect.Value) error {
	rw := reflect.ValueOf(w)
	if rw.Type() == v.Type() {
		v.Set(rw)
		return nil
	}
	if v.Kind() == reflect.Interface && rw.Type().Implements(v.Type()) {
		v.Set(rw)
		return nil
	}
	d.saveError(&UnmarshalTypeError{Value: value, Type: v.Type(), Offset: int64(start)})
	return nil
}

func (d *decodeState) decodePositiveInt(start int, w uint64, v reflect.Value) error {
	switch v.Type() {
	case integerType:
		v.Set(reflect.ValueOf(Integer{Value: w}))
		return nil
	}

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if w > math.MaxInt64 || v.OverflowInt(int64(w)) {
			d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
			break
		}
		v.SetInt(int64(w))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if v.OverflowUint(w) {
			d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
			break
		}
		v.SetUint(uint64(w))
	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
			break
		}
		if d.useInteger {
			v.Set(reflect.ValueOf(Integer{Value: w}))
		} else {
			if w > math.MaxInt64 {
				d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
				break
			}
			v.Set(reflect.ValueOf(int64(w)))
		}
	default:
		d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) decodeNegativeInt(start int, w uint64, v reflect.Value) error {
	switch v.Type() {
	case integerType:
		v.Set(reflect.ValueOf(Integer{Sign: true, Value: w}))
		return nil
	}

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := int64(^w)
		if i >= 0 || v.OverflowInt(i) {
			d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
		}
		v.SetInt(i)
	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
			break
		}
		if d.useInteger {
			v.Set(reflect.ValueOf(Integer{Sign: true, Value: w}))
		} else {
			i := int64(^w)
			if i >= 0 {
				d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
				break
			}
			v.Set(reflect.ValueOf(i))
		}
	default:
		d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) decodeFloat16(start int, w uint16, v reflect.Value) error {
	sign := uint64(w&0x8000) << (64 - 16)
	exp := uint64(w>>10) & 0x1f
	frac := uint64(w & 0x03ff)

	switch {
	case exp == 0:
		// zero or subnormal
		l := bits.Len64(frac)
		if l == 0 {
			// zero
			exp = 0
		} else {
			// subnormal
			frac = (frac << (53 - uint64(l))) & (1<<52 - 1)
			exp = 1023 - (15 + 10) + uint64(l)
		}
	case exp == 0x1f:
		// infinity or NaN
		exp = 0x7ff
		if frac != 0 {
			frac = 1 << 51
		}
	default:
		// normal number
		exp += 1023 - 15
		frac <<= 52 - 10
	}
	f := math.Float64frombits(sign | exp<<52 | uint64(frac))
	return d.decodeFloat(start, f, v)
}

func (d *decodeState) decodeFloat32(start int, w uint32, v reflect.Value) error {
	f := math.Float32frombits(w)
	return d.decodeFloat(start, float64(f), v)
}

func (d *decodeState) decodeFloat64(start int, w uint64, v reflect.Value) error {
	f := math.Float64frombits(w)
	return d.decodeFloat(start, f, v)
}

func (d *decodeState) decodeFloat(start int, f float64, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		if v.OverflowFloat(f) {
			d.saveError(&UnmarshalTypeError{Value: "float", Type: v.Type(), Offset: int64(start)})
		}
		v.SetFloat(f)
	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
			break
		}
		v.Set(reflect.ValueOf(f))
	default:
		d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) decodeBytes(start int, n uint64, u Unmarshaler, v reflect.Value) error {
	if !d.isAvailable(n) {
		return d.newSyntaxError("cbor: unexpected end")
	}
	off := d.off
	d.off += int(n)
	if u != nil {
		return u.UnmarshalCBOR(d.data[start:d.off])
	}
	s := bytes.Clone(d.data[off : off+int(n)])
	return d.setBytes(start, s, v)
}

func (d *decodeState) decodeBytesIndefinite(start int, u Unmarshaler, v reflect.Value) error {
	s := []byte{}

LOOP:
	for {
		var n uint64
		typ, err := d.readByte()
		if err != nil {
			return err
		}
		switch {
		case typ == 0xff:
			break LOOP
		case typ >= 0x40 && typ <= 0x57:
			n = uint64(typ - 0x40)
		case typ == 0x58:
			m, err := d.readByte()
			if err != nil {
				return err
			}
			n = uint64(m)
		case typ == 0x59:
			m, err := d.readUint16()
			if err != nil {
				return err
			}
			n = uint64(m)
		case typ == 0x5a:
			m, err := d.readUint32()
			if err != nil {
				return err
			}
			n = uint64(m)
		case typ == 0x5b:
			m, err := d.readUint64()
			if err != nil {
				return err
			}
			n = m
		default:
			return d.newSyntaxError("cbor: invalid byte string chunk type")
		}
		if !d.isAvailable(n) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		if u == nil {
			s = append(s, d.data[d.off:d.off+int(n)]...)
		}
		d.off += int(n)
	}
	if u != nil {
		return u.UnmarshalCBOR(d.data[start:d.off])
	}
	return d.setBytes(start, s, v)
}

func (d *decodeState) setBytes(start int, data []byte, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			d.saveError(&UnmarshalTypeError{Value: "bytes", Type: v.Type(), Offset: int64(start)})
			break
		}
		v.SetBytes(data)
	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "bytes", Type: v.Type(), Offset: int64(start)})
			break
		}
		if d.decodingKeys {
			// slices cannot be used as map keys; fall back to array.
			t := reflect.ArrayOf(len(data), byteType)
			array := reflect.New(t).Elem()
			for i, b := range data {
				array.Index(i).SetUint(uint64(b))
			}
			v.Set(array)
		} else {
			v.Set(reflect.ValueOf(data))
		}
	default:
		d.saveError(&UnmarshalTypeError{Value: "bytes", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) decodeString(start int, n uint64, u Unmarshaler, v reflect.Value) error {
	if !d.isAvailable(n) {
		return d.newSyntaxError("cbor: unexpected end")
	}
	off := d.off
	d.off += int(n)
	if u != nil {
		return u.UnmarshalCBOR(d.data[start:d.off])
	}

	if !utf8.Valid(d.data[off:d.off]) {
		return d.newSyntaxError("cbor: invalid UTF-8 string")
	}
	s := string(d.data[off:d.off])
	return d.setString(start, s, v)
}

func (d *decodeState) decodeStringIndefinite(start int, u Unmarshaler, v reflect.Value) error {
	var w io.Writer
	var builder *strings.Builder
	if u != nil {
		w = io.Discard
	} else {
		builder = new(strings.Builder)
		w = builder
	}
LOOP:
	for {
		var n uint64
		typ, err := d.readByte()
		if err != nil {
			return err
		}
		switch {
		case typ == 0xff:
			break LOOP
		case typ >= 0x60 && typ <= 0x77:
			n = uint64(typ - 0x60)
		case typ == 0x78:
			m, err := d.readByte()
			if err != nil {
				return err
			}
			n = uint64(m)
		case typ == 0x79:
			m, err := d.readUint16()
			if err != nil {
				return err
			}
			n = uint64(m)
		case typ == 0x7a:
			m, err := d.readUint32()
			if err != nil {
				return err
			}
			n = uint64(m)
		case typ == 0x7b:
			m, err := d.readUint64()
			if err != nil {
				return err
			}
			n = m
		default:
			return d.newSyntaxError("cbor: invalid byte string chunk type")
		}
		if !d.isAvailable(n) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		w.Write(d.data[d.off : d.off+int(n)])
		d.off += int(n)
	}
	if u != nil {
		return u.UnmarshalCBOR(d.data[start:d.off])
	}
	s := builder.String()
	if !utf8.ValidString(s) {
		return d.newSyntaxError("cbor: invalid UTF-8 string")
	}
	return d.setString(start, s, v)
}

func (d *decodeState) setString(start int, s string, v reflect.Value) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(start)})
			break
		}
		v.Set(reflect.ValueOf(s))
	default:
		d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) decodeArray(start int, n uint64, u Unmarshaler, v reflect.Value) error {
	if u != nil {
		for i := 0; i < int(n); i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}
		return u.UnmarshalCBOR(d.data[start:d.off])
	}
	switch v.Kind() {
	case reflect.Slice:
		if c := uint64(v.Cap()); c < n || (c == 0 && n == 0) {
			v.Set(reflect.MakeSlice(v.Type(), int(n), int(n)))
		}
		v.SetLen(int(n))
		for i := 0; i < int(n); i++ {
			if err := d.decodeReflectValue(v.Index(i)); err != nil {
				return err
			}
		}

	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(start)})
		}
		var s reflect.Value
		if d.decodingKeys {
			// slices cannot be used as map keys; fall back to array.
			s = reflect.New(reflect.ArrayOf(int(n), anyType)).Elem()
		} else {
			s = reflect.MakeSlice(anySliceType, int(n), int(n))
		}
		for i := 0; i < int(n); i++ {
			if err := d.decodeReflectValue(s.Index(i)); err != nil {
				return err
			}
		}
		v.Set(s)

	case reflect.Struct:
		st := cachedStructType(v.Type())
		if !st.toArray {
			d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(start)})
		}
		i := 0
		for i = 0; i < int(n) && i < len(st.fields); i++ {
			f := v.FieldByIndex(st.fields[i].index)
			if err := d.decodeReflectValue(f); err != nil {
				return err
			}
		}

		// skip remaining fields
		for j := i; j < int(n); j++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

		// fill zero values for omitted fields
		for j := i; j < len(st.fields); j++ {
			f := v.FieldByIndex(st.fields[j].index)
			f.Set(reflect.Zero(f.Type()))
		}

	default:
		d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) decodeArrayIndefinite(start int, u Unmarshaler, v reflect.Value) error {
	if u != nil {
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}
		return u.UnmarshalCBOR(d.data[start:d.off])
	}
	switch v.Kind() {
	case reflect.Slice:
		i := 0
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}

			// Expand slice length, growing the slice if necessary.
			if i >= v.Cap() {
				v.Grow(1)
			}
			if i >= v.Len() {
				v.SetLen(i + 1)
			}

			// Decode into the slice element.
			if err := d.decodeReflectValue(v.Index(i)); err != nil {
				return err
			}
			i++
		}
		v.SetLen(i)
		if i == 0 {
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		}

	case reflect.Interface:
		// Decoding into nil interface? Switch to non-reflect code.
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(start)})
		}

		s := []any{}
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}

			var e any
			if err := d.decode(&e); err != nil {
				return err
			}
			s = append(s, e)
		}
		v.Set(reflect.ValueOf(s))

	case reflect.Struct:
		st := cachedStructType(v.Type())
		if !st.toArray {
			d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(start)})
		}

		i := 0
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}

			if i < len(st.fields) {
				f := v.FieldByIndex(st.fields[i].index)
				if err := d.decodeReflectValue(f); err != nil {
					return err
				}
			} else {
				if err := d.checkWellFormedChild(); err != nil {
					return err
				}
			}
			i++
		}

		// fill zero values for omitted fields
		for j := i; j < len(st.fields); j++ {
			f := v.FieldByIndex(st.fields[j].index)
			f.Set(reflect.Zero(f.Type()))
		}

	default:
		d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(start)})
		return nil
	}

	return nil
}

func (d *decodeState) decodeMap(start int, n uint64, u Unmarshaler, v reflect.Value) error {
	if u != nil {
		for i := 0; i < int(n); i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return nil
			}
			if err := d.checkWellFormedChild(); err != nil {
				return nil
			}
		}
		return u.UnmarshalCBOR(d.data[start:d.off])
	}

	if d.decodingKeys {
		return d.newSyntaxError("cbor: unexpected map key")
	}

	switch v.Kind() {
	case reflect.Map:
		if v.IsNil() {
			v.Set(reflect.MakeMapWithSize(v.Type(), int(n)))
		}
		kt := v.Type().Key()
		et := v.Type().Elem()
		for i := 0; i < int(n); i++ {
			// decode the key.
			d.decodingKeys = true
			key := reflect.New(kt).Elem()
			err := d.decodeReflectValue(key)
			d.decodingKeys = false
			if err != nil {
				return err
			}

			// decode the element.
			elem := reflect.New(et).Elem()
			if err := d.decodeReflectValue(elem); err != nil {
				return err
			}
			v.SetMapIndex(key, elem)
		}

	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "map", Type: v.Type(), Offset: int64(start)})
		}

		if d.useAnyKey {
			m := map[any]any{}
			for i := 0; i < int(n); i++ {
				d.decodingKeys = true
				var key any
				err := d.decode(&key)
				d.decodingKeys = false
				if err != nil {
					return err
				}

				var elem any
				if err := d.decode(&elem); err != nil {
					return err
				}
				m[key] = elem
			}
			v.Set(reflect.ValueOf(m))
		} else {
			m := map[string]any{}
			for i := 0; i < int(n); i++ {
				d.decodingKeys = true
				var key string
				err := d.decode(&key)
				d.decodingKeys = false
				if err != nil {
					return err
				}

				var elem any
				if err := d.decode(&elem); err != nil {
					return err
				}
				m[key] = elem
			}
			v.Set(reflect.ValueOf(m))
		}

	case reflect.Struct:
		// save original error context
		var origErrorContext errorContext
		if d.errorContext != nil {
			origErrorContext = *d.errorContext
		} else {
			d.errorContext = new(errorContext)
		}

		t := v.Type()
		st := cachedStructType(t)
		for i := 0; i < int(n); i++ {
			var key any
			d.decodingKeys = true
			err := d.decode(&key)
			d.decodingKeys = false
			if err != nil {
				d.saveError(err)
				break
			}
			if f, ok := st.maps[key]; ok {
				d.errorContext.Struct = t
				d.errorContext.FieldStack = append(d.errorContext.FieldStack, f.name)
				if err := d.decodeReflectValue(v.FieldByIndex(f.index)); err != nil {
					d.saveError(err)
					break
				}
			} else {
				if err := d.checkWellFormedChild(); err != nil {
					d.saveError(err)
					break
				}
			}
		}

		// restore original error context
		if d.errorContext != nil {
			// Reset errorContext to its original state.
			// Keep the same underlying array for FieldStack, to reuse the
			// space and avoid unnecessary allocs.
			d.errorContext.FieldStack = d.errorContext.FieldStack[:len(origErrorContext.FieldStack)]
			d.errorContext.Struct = origErrorContext.Struct
		}

	default:
		d.saveError(&UnmarshalTypeError{Value: "map", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) decodeMapIndefinite(start int, u Unmarshaler, v reflect.Value) error {
	if u != nil {
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}
		return u.UnmarshalCBOR(d.data[start:d.off])
	}

	if d.decodingKeys {
		return d.newSyntaxError("cbor: unexpected map key")
	}

	switch v.Kind() {
	case reflect.Map:
		if v.IsNil() {
			v.Set(reflect.MakeMapWithSize(v.Type(), 0))
		}
		kt := v.Type().Key()
		et := v.Type().Elem()
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}

			key := reflect.New(kt).Elem()
			d.decodingKeys = true
			err = d.decodeReflectValue(key)
			d.decodingKeys = false
			if err != nil {
				return err
			}
			elem := reflect.New(et).Elem()
			if err := d.decodeReflectValue(elem); err != nil {
				return err
			}
			v.SetMapIndex(key, elem)
		}

	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "map", Type: v.Type(), Offset: int64(start)})
		}

		if d.useAnyKey {
			m := map[any]any{}
			for {
				typ, err := d.peekByte()
				if err != nil {
					return err
				}
				if typ == 0xff {
					d.off++
					break
				}

				var key any
				d.decodingKeys = true
				err = d.decode(&key)
				d.decodingKeys = false
				if err != nil {
					return err
				}

				var elem any
				if err := d.decode(&elem); err != nil {
					return err
				}
				m[key] = elem
			}
			v.Set(reflect.ValueOf(m))
		} else {
			m := map[string]any{}
			for {
				// find the end of the map
				typ, err := d.peekByte()
				if err != nil {
					return err
				}
				if typ == 0xff {
					d.off++
					break
				}

				// decode the key
				var key string
				d.decodingKeys = true
				err = d.decode(&key)
				d.decodingKeys = false
				if err != nil {
					return err
				}

				// decode the element
				var elem any
				if err := d.decode(&elem); err != nil {
					return err
				}
				m[key] = elem
			}
			v.Set(reflect.ValueOf(m))
		}

	case reflect.Struct:
		// save original error context
		var origErrorContext errorContext
		if d.errorContext != nil {
			origErrorContext = *d.errorContext
		} else {
			d.errorContext = new(errorContext)
		}

		t := v.Type()
		st := cachedStructType(t)
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}

			var key any
			d.decodingKeys = true
			err = d.decode(&key)
			d.decodingKeys = false
			if err != nil {
				d.saveError(err)
				break
			}
			if f, ok := st.maps[key]; ok {
				d.errorContext.Struct = t
				d.errorContext.FieldStack = append(d.errorContext.FieldStack, f.name)
				if err := d.decodeReflectValue(v.FieldByIndex(f.index)); err != nil {
					d.saveError(err)
					break
				}
			} else {
				if err := d.checkWellFormedChild(); err != nil {
					d.saveError(err)
					break
				}
			}
		}

		// restore original error context
		if d.errorContext != nil {
			// Reset errorContext to its original state.
			// Keep the same underlying array for FieldStack, to reuse the
			// space and avoid unnecessary allocs.
			d.errorContext.FieldStack = d.errorContext.FieldStack[:len(origErrorContext.FieldStack)]
			d.errorContext.Struct = origErrorContext.Struct
		}

	default:
		d.saveError(&UnmarshalTypeError{Value: "map", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) decodeTag(start int, n TagNumber, u Unmarshaler, v reflect.Value) error {
	if u != nil {
		if err := d.checkWellFormedChild(); err != nil {
			return err
		}
		return u.UnmarshalCBOR(d.data[start:d.off])
	}

	var content any
	if err := d.decode(&content); err != nil {
		return err
	}
	if v.Type() == tagType {
		v.Set(reflect.ValueOf(Tag{Number: n, Content: content}))
		return nil
	}
	switch v.Kind() {
	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "tag", Type: v.Type(), Offset: int64(start)})
			break
		}
		v.Set(reflect.ValueOf(Tag{Number: n, Content: content}))
	default:
		d.saveError(&UnmarshalTypeError{Value: "tag", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) setSimple(start int, s Simple, v reflect.Value) error {
	t := v.Type()
	if t == simpleType {
		v.Set(reflect.ValueOf(s))
		return nil
	}
	switch v.Kind() {
	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "simple", Type: v.Type(), Offset: int64(start)})
			break
		}
		v.Set(reflect.ValueOf(s))
	default:
		d.saveError(&UnmarshalTypeError{Value: "simple", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) setBool(start int, b bool, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(b)
	case reflect.Interface:
		if v.NumMethod() != 0 {
			d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(start)})
			break
		}
		v.Set(reflect.ValueOf(b))
	default:
		d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) setNull(start int, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr, reflect.Map, reflect.Slice:
		v.Set(reflect.Zero(v.Type()))
	default:
		d.saveError(&UnmarshalTypeError{Value: "null", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) setUndefined(start int, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Interface:
		v.Set(reflect.ValueOf(Undefined))
	case reflect.Ptr, reflect.Map, reflect.Slice:
		v.Set(reflect.Zero(v.Type()))
	default:
		d.saveError(&UnmarshalTypeError{Value: "undefined", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) decodeBigFloat(start int, v reflect.Value) error {
	var a []any
	if err := d.decode(&a); err != nil {
		return err
	}
	if len(a) != 2 {
		d.saveError(&UnmarshalTypeError{Value: "bigfloat", Type: v.Type(), Offset: int64(start)})
	}
	var exp int64
	switch x := a[0].(type) {
	case int64:
		exp = x
	case Integer:
		var err error
		exp, err = x.Int64()
		if err != nil {
			return err
		}
	default:
		d.saveError(&UnmarshalTypeError{Value: "bigfloat", Type: v.Type(), Offset: int64(start)})
	}

	var mant *big.Int
	switch x := a[1].(type) {
	case int64:
		mant = big.NewInt(x)
	case Integer:
		mant = x.BigInt()
	case *big.Int:
		mant = x
	default:
		d.saveError(&UnmarshalTypeError{Value: "bigfloat", Type: v.Type(), Offset: int64(start)})
		return nil
	}

	var f *big.Float
	if v.Type() == bigFloatType {
		// reuse the existing big.Float
		f = v.Addr().Interface().(*big.Float)
	} else {
		f = new(big.Float)
	}
	f.SetInt(mant)
	f.SetMantExp(f, int(exp))
	if v.Type() != bigFloatType {
		return d.setAny(start, "bigfloat", f, v)
	}
	return nil
}

// WellFormed reports whether data is a valid CBOR encoding.
func WellFormed(data []byte) bool {
	d := newDecodeState(data)
	return d.checkWellFormed() == nil
}

func (d *decodeState) checkWellFormed() error {
	if err := d.checkWellFormedChild(); err != nil {
		return err
	}
	if d.off != len(d.data) {
		return d.newSyntaxError("cbor: unexpected data after top-level value")
	}
	return nil
}

func (d *decodeState) checkWellFormedChild() error {
	typ, err := d.readByte()
	if err != nil {
		return err
	}

	switch typ {
	// unsigned integer 0x00..0x17 (0..23)
	case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17:

	// unsigned integer (one-byte uint8_t follows)
	case 0x18:
		if _, err := d.readByte(); err != nil {
			return err
		}

	// unsigned integer (two-byte uint16_t follows)
	case 0x19:
		if _, err := d.readUint16(); err != nil {
			return err
		}

	// unsigned integer (four-byte uint32_t follows)
	case 0x1a:
		if _, err := d.readUint32(); err != nil {
			return err
		}

	// unsigned integer (eight-byte uint64_t follows)
	case 0x1b:
		if _, err := d.readUint64(); err != nil {
			return err
		}

	// negative integer -1-0x00..-1-0x17 (-1..-24)
	case 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37:

	// negative integer -1-n (one-byte uint8_t for n follows)
	case 0x38:
		if _, err := d.readByte(); err != nil {
			return err
		}

	// negative integer -1-n (two-byte uint16_t for n follows)
	case 0x39:
		if _, err := d.readUint16(); err != nil {
			return err
		}

	// negative integer -1-n (four-byte uint32_t for n follows)
	case 0x3a:
		if _, err := d.readUint32(); err != nil {
			return err
		}

	// negative integer -1-n (eight-byte uint64_t for n follows)
	case 0x3b:
		if _, err := d.readUint64(); err != nil {
			return err
		}

	// byte string (0x00..0x17 bytes follow)
	case 0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f, 0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57:
		n := uint64(typ - 0x40)
		if uint64(d.off)+n > uint64(len(d.data)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// byte string (one-byte uint8_t for n, and then n bytes follow)
	case 0x58:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		if !d.isAvailable(uint64(n)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// byte string (two-byte uint16_t for n, and then n bytes follow)
	case 0x59:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		if !d.isAvailable(uint64(n)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// byte string (four-byte uint32_t for n, and then n bytes follow)
	case 0x5a:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		if !d.isAvailable(uint64(n)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// byte string (eight-byte uint64_t for n, and then n bytes follow)
	case 0x5b:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		if !d.isAvailable(uint64(n)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// byte string (indefinite length)
	case 0x5f:
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}
			if typ < 0x40 || typ > 0x5b {
				return errors.New("cbor: invalid byte string")
			}
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// text string (0x00..0x17 bytes follow)
	case 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77:
		n := int(typ - 0x60)
		if !d.isAvailable(uint64(n)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// text string (one-byte uint8_t for n, and then n bytes follow)
	case 0x78:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		if !d.isAvailable(uint64(n)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// text string (two-byte uint16_t for n, and then n bytes follow)
	case 0x79:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		if !d.isAvailable(uint64(n)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// text string (four-byte uint32_t for n, and then n bytes follow)
	case 0x7a:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		if !d.isAvailable(uint64(n)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// text string (eight-byte uint64_t for n, and then n bytes follow)
	case 0x7b:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		if !d.isAvailable(uint64(n)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// text string (indefinite length)
	case 0x7f:
		var s string
		err := d.decodeStringIndefinite(d.off-1, nil, reflect.ValueOf(&s).Elem())
		if err != nil {
			return err
		}

	// array (0x00..0x17 data items follow)
	case 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f, 0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97:
		n := int(typ - 0x80)
		for i := 0; i < n; i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// array (one-byte uint8_t for n, and then n data items follow)
	case 0x98:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		for i := 0; i < int(n); i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// array (two-byte uint16_t for n, and then n data items follow)
	case 0x99:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		for i := 0; i < int(n); i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// array (four-byte uint32_t for n, and then n data items follow)
	case 0x9a:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		for i := uint64(0); i < uint64(n); i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// array (eight-byte uint64_t for n, and then n data items follow)
	case 0x9b:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		for i := uint64(0); i < n; i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// array (indefinite length)
	case 0x9f:
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}

			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// map (0x00..0x17 pairs of data items follow)
	case 0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf, 0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7:
		n := int(typ - 0xa0)
		for i := 0; i < n; i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// map (one-byte uint8_t for n, and then n pairs of data items follow)
	case 0xb8:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		for i := uint8(0); i < n; i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// map (two-byte uint16_t for n, and then n pairs of data items follow)
	case 0xb9:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		for i := uint16(0); i < n; i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// map (four-byte uint32_t for n, and then n pairs of data items follow)
	case 0xba:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// map (eight-byte uint64_t for n, and then n pairs of data items follow)
	case 0xbb:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		for i := uint64(0); i < n; i++ {
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// map (indefinite length)
	case 0xbf:
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}

			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
		}

	// tags
	case 0xc0, 0xc1, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf, 0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7:
		if err := d.checkWellFormedChild(); err != nil {
			return err
		}

	// (more tags; 1/2/4/8 bytes of tag number and then a data item follow)
	case 0xd8:
		if _, err := d.readByte(); err != nil {
			return err
		}
		if err := d.checkWellFormedChild(); err != nil {
			return err
		}
	case 0xd9:
		if _, err := d.readUint16(); err != nil {
			return err
		}
		if err := d.checkWellFormedChild(); err != nil {
			return err
		}
	case 0xda:
		if _, err := d.readUint32(); err != nil {
			return err
		}
		if err := d.checkWellFormedChild(); err != nil {
			return err
		}
	case 0xdb:
		if _, err := d.readUint64(); err != nil {
			return err
		}
		if err := d.checkWellFormedChild(); err != nil {
			return err
		}

	// simple values
	case 0xe0, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef, 0xf0, 0xf3:

	// false
	case 0xf4:

	// true
	case 0xf5:

	// null
	case 0xf6:

	// undefined
	case 0xf7:

	// simple value (one-byte uint8_t follows)
	case 0xf8:
		b, err := d.readByte()
		if err != nil {
			return err
		}
		if b < 0x20 {
			return d.newSyntaxError("cbor: invalid simple value")
		}

	// half-precision float (two-byte IEEE 754)
	case 0xf9:
		if _, err := d.readUint16(); err != nil {
			return err
		}

	// single-precision float (four-byte IEEE 754)
	case 0xfa:
		if _, err := d.readUint32(); err != nil {
			return err
		}

	// double-precision float (eight-byte IEEE 754)
	case 0xfb:
		if _, err := d.readUint64(); err != nil {
			return err
		}

	default:
		return d.newSyntaxError("cbor: unknown initial byte: " + strconv.Itoa(int(typ)))
	}

	return nil
}

// A SyntaxError is a description of a CBOR syntax error.
// Unmarshal will return a SyntaxError if the CBOR can't be parsed.
type SyntaxError struct {
	msg    string // description of error
	Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return e.msg }

func (d *decodeState) newSyntaxError(msg string) error {
	return &SyntaxError{msg: msg, Offset: int64(d.off)}
}
