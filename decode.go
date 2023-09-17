package cbor

import (
	"encoding/binary"
	"errors"
	"math"
	"math/bits"
	"reflect"
	"slices"
	"strconv"
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
	if err := d.checkValidChild(); err != nil {
		return err
	}

	d.init(data)
	if err := d.decode(v); err != nil {
		return err
	}
	if d.err != nil {
		return d.err
	}
	return nil
}

func newDecodeState(data []byte) *decodeState {
	d := new(decodeState)
	d.init(data)
	return d
}

type decodeState struct {
	data []byte
	off  int // next read offset
	err  error
}

func (d *decodeState) init(data []byte) {
	d.data = data
	d.off = 0
	d.err = nil
}

func (s *decodeState) readByte() (byte, error) {
	if s.off+1 > len(s.data) {
		return 0, s.newSyntaxError("cbor: unexpected end")
	}
	b := s.data[s.off]
	s.off++
	return b, nil
}

func (s *decodeState) peekByte() (byte, error) {
	if s.off+1 > len(s.data) {
		return 0, s.newSyntaxError("cbor: unexpected end")
	}
	return s.data[s.off], nil
}

func (s *decodeState) readUint16() (uint16, error) {
	if s.off+2 > len(s.data) {
		return 0, s.newSyntaxError("cbor: unexpected end")
	}
	b := binary.BigEndian.Uint16(s.data[s.off:])
	s.off += 2
	return b, nil
}

func (s *decodeState) readUint32() (uint32, error) {
	if s.off+4 > len(s.data) {
		return 0, s.newSyntaxError("cbor: unexpected end")
	}
	b := binary.BigEndian.Uint32(s.data[s.off:])
	s.off += 4
	return b, nil
}

func (s *decodeState) readUint64() (uint64, error) {
	if s.off+8 > len(s.data) {
		return 0, s.newSyntaxError("cbor: unexpected end")
	}
	b := binary.BigEndian.Uint64(s.data[s.off:])
	s.off += 8
	return b, nil
}

func (s *decodeState) saveError(err error) {
	if s.err == nil {
		s.err = err
	}
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
		return errors.New("cbor: err") // TODO: introduce InvalidUnmarshalError
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
		data, err := d.decodeBytes(start, n)
		if err != nil {
			return err
		}
		return d.setBytes(start, data, v)

	// byte string (one-byte uint8_t for n, and then n bytes follow)
	case 0x58:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		data, err := d.decodeBytes(start, uint64(n))
		if err != nil {
			return err
		}
		return d.setBytes(start, data, v)

	// byte string (two-byte uint16_t for n, and then n bytes follow)
	case 0x59:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		data, err := d.decodeBytes(start, uint64(n))
		if err != nil {
			return err
		}
		return d.setBytes(start, data, v)

	// byte string (four-byte uint32_t for n, and then n bytes follow)
	case 0x5a:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		data, err := d.decodeBytes(start, uint64(n))
		if err != nil {
			return err
		}
		return d.setBytes(start, data, v)

	// byte string (eight-byte uint64_t for n, and then n bytes follow)
	case 0x5b:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		data, err := d.decodeBytes(start, n)
		if err != nil {
			return err
		}
		return d.setBytes(start, data, v)

	// byte string (indefinite length)
	case 0x5f:
		data, err := d.decodeBytesIndefinite()
		if err != nil {
			return err
		}
		return d.setBytes(start, data, v)

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

func (d *decodeState) decodePositiveInt(start int, w uint64, v reflect.Value) error {
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
		if v.NumMethod() != 0 || w > math.MaxInt64 {
			d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
			break
		}
		v.Set(reflect.ValueOf(int64(w)))
	default:
		d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

func (d *decodeState) decodeNegativeInt(start int, w uint64, v reflect.Value) error {
	_, v = indirect(v, false)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := int64(^w)
		if i >= 0 || v.OverflowInt(i) {
			d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
		}
		v.SetInt(i)
	case reflect.Interface:
		i := int64(^w)
		if v.NumMethod() != 0 || i >= 0 {
			d.saveError(&UnmarshalTypeError{Value: "integer", Type: v.Type(), Offset: int64(start)})
			break
		}
		v.Set(reflect.ValueOf(i))
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

func (d *decodeState) decodeBytes(start int, n uint64) ([]byte, error) {
	if uint64(d.off)+n > uint64(len(d.data)) {
		return nil, d.newSyntaxError("cbor: unexpected end")
	}
	return slices.Clone(d.data[d.off : d.off+int(n)]), nil
}

func (d *decodeState) decodeBytesIndefinite() ([]byte, error) {
	var s []byte
	for {
		var n uint64
		typ, err := d.readByte()
		if err != nil {
			return nil, err
		}
		switch {
		case typ == 0xff:
			return s, nil
		case typ >= 0x40 && typ <= 0x57:
			n = uint64(typ - 0x40)
		case typ == 0x58:
			m, err := d.readByte()
			if err != nil {
				return nil, err
			}
			n = uint64(m)
		case typ == 0x59:
			m, err := d.readUint16()
			if err != nil {
				return nil, err
			}
			n = uint64(m)
		case typ == 0x5a:
			m, err := d.readUint32()
			if err != nil {
				return nil, err
			}
			n = uint64(m)
		case typ == 0x5b:
			m, err := d.readUint64()
			if err != nil {
				return nil, err
			}
			n = m
		default:
			return nil, d.newSyntaxError("cbor: invalid byte string chunk type")
		}
		if uint64(d.off)+n > uint64(len(d.data)) {
			return nil, d.newSyntaxError("cbor: unexpected end")
		}
		s = append(s, d.data[d.off:d.off+int(n)]...)
		d.off += int(n)
	}
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
		v.Set(reflect.ValueOf(data))
	default:
		d.saveError(&UnmarshalTypeError{Value: "bytes", Type: v.Type(), Offset: int64(start)})
	}
	return nil
}

// Valid reports whether data is a valid CBOR encoding.
func Valid(data []byte) bool {
	d := newDecodeState(data)
	return d.checkValid() == nil
}

func (d *decodeState) checkValid() error {
	if err := d.checkValidChild(); err != nil {
		return err
	}
	if d.off != len(d.data) {
		return d.newSyntaxError("cbor: unexpected data after top-level value")
	}
	return nil
}

func (d *decodeState) checkValidChild() error {
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
		if uint64(d.off)+uint64(n) > uint64(len(d.data)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// byte string (two-byte uint16_t for n, and then n bytes follow)
	case 0x59:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		if uint64(d.off)+uint64(n) > uint64(len(d.data)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// byte string (four-byte uint32_t for n, and then n bytes follow)
	case 0x5a:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		if uint64(d.off)+uint64(n) > uint64(len(d.data)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// byte string (eight-byte uint64_t for n, and then n bytes follow)
	case 0x5b:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		if uint64(d.off)+n < n || uint64(d.off)+n > uint64(len(d.data)) {
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
			if err := d.checkValidChild(); err != nil {
				return err
			}
		}

	// text string (0x00..0x17 bytes follow)
	case 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77:
		n := uint64(typ - 0x60)
		if uint64(d.off)+n > uint64(len(d.data)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// text string (one-byte uint8_t for n, and then n bytes follow)
	case 0x78:
		n, err := d.readByte()
		if err != nil {
			return err
		}
		if uint64(d.off)+uint64(n) > uint64(len(d.data)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// text string (two-byte uint16_t for n, and then n bytes follow)
	case 0x79:
		n, err := d.readUint16()
		if err != nil {
			return err
		}
		if uint64(d.off)+uint64(n) > uint64(len(d.data)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// text string (four-byte uint32_t for n, and then n bytes follow)
	case 0x7a:
		n, err := d.readUint32()
		if err != nil {
			return err
		}
		if uint64(d.off)+uint64(n) > uint64(len(d.data)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// text string (eight-byte uint64_t for n, and then n bytes follow)
	case 0x7b:
		n, err := d.readUint64()
		if err != nil {
			return err
		}
		if uint64(d.off)+n < n || uint64(d.off)+n > uint64(len(d.data)) {
			return d.newSyntaxError("cbor: unexpected end")
		}
		d.off += int(n)

	// text string (indefinite length)
	case 0x7f:
		for {
			typ, err := d.peekByte()
			if err != nil {
				return err
			}
			if typ == 0xff {
				d.off++
				break
			}
			if typ < 0x60 || typ > 0x7b {
				return errors.New("cbor: invalid byte string")
			}
			if err := d.checkValidChild(); err != nil {
				return err
			}
		}

	// array (0x00..0x17 data items follow)
	case 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f, 0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97:
		n := int(typ - 0x80)
		for i := 0; i < n; i++ {
			if err := d.checkValidChild(); err != nil {
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
			if err := d.checkValidChild(); err != nil {
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
			if err := d.checkValidChild(); err != nil {
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
			if err := d.checkValidChild(); err != nil {
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
			if err := d.checkValidChild(); err != nil {
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

			if err := d.checkValidChild(); err != nil {
				return err
			}
		}

	// map (0x00..0x17 pairs of data items follow)
	case 0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf, 0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7:
		n := int(typ - 0xa0)
		for i := 0; i < n; i++ {
			if err := d.checkValidChild(); err != nil {
				return err
			}
			if err := d.checkValidChild(); err != nil {
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
			if err := d.checkValidChild(); err != nil {
				return err
			}
			if err := d.checkValidChild(); err != nil {
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
			if err := d.checkValidChild(); err != nil {
				return err
			}
			if err := d.checkValidChild(); err != nil {
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
			if err := d.checkValidChild(); err != nil {
				return err
			}
			if err := d.checkValidChild(); err != nil {
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
			if err := d.checkValidChild(); err != nil {
				return err
			}
			if err := d.checkValidChild(); err != nil {
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

			if err := d.checkValidChild(); err != nil {
				return err
			}
			if err := d.checkValidChild(); err != nil {
				return err
			}
		}

	// text-based date/time
	case 0xc0:
		var s string
		if err := d.decode(&s); err != nil {
			return err
		}

	// epoch-based date/time
	case 0xc1:
		// TODO: check that the integer is a valid date/time

	// unsigned bignum (data item "byte string" follows)
	case 0xc2:
		// TODO: check that the byte string is a valid bignum

	// negative bignum (data item "byte string" follows)
	case 0xc3:
		// TODO: check that the byte string is a valid bignum

	// decimal fraction (data item "array" follows)
	case 0xc4:
		// TODO: check that the array is a valid decimal fraction

	// bigfloat (data item "array" follows)
	case 0xc5:
		// TODO: check that the array is a valid bigfloat

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
