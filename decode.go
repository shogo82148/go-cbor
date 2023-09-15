package cbor

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"math/bits"
	"reflect"
)

func Unmarshal(data []byte, v any) error {
	d := newDecodeState(data)
	return d.decode(v)
}

func newDecodeState(data []byte) *decodeState {
	return &decodeState{data: data}
}

type decodeState struct {
	data []byte
	off  int // next read offset
}

func (s *decodeState) readByte() (byte, error) {
	if s.off+1 > len(s.data) {
		return 0, io.ErrUnexpectedEOF
	}
	b := s.data[s.off]
	s.off++
	return b, nil
}

func (s *decodeState) readUint16() (uint16, error) {
	if s.off+2 > len(s.data) {
		return 0, io.ErrUnexpectedEOF
	}
	b := binary.BigEndian.Uint16(s.data[s.off:])
	s.off += 2
	return b, nil
}

func (s *decodeState) readUint32() (uint32, error) {
	if s.off+4 > len(s.data) {
		return 0, io.ErrUnexpectedEOF
	}
	b := binary.BigEndian.Uint32(s.data[s.off:])
	s.off += 4
	return b, nil
}

func (s *decodeState) readUint64() (uint64, error) {
	if s.off+8 > len(s.data) {
		return 0, io.ErrUnexpectedEOF
	}
	b := binary.BigEndian.Uint64(s.data[s.off:])
	s.off += 8
	return b, nil
}

func indirect(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v
}

func (s *decodeState) decode(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("cbor: err") // TODO: introduce InvalidUnmarshalError
	}
	return s.decodeReflectValue(rv)
}

func (s *decodeState) decodeReflectValue(v reflect.Value) error {
	typ, err := s.readByte()
	if err != nil {
		return err
	}

	switch typ {
	// unsigned integer 0x00..0x17 (0..23)
	case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17:
		return s.decodePositiveInt(uint64(typ), v)

	// unsigned integer (one-byte uint8_t follows)
	case 0x18:
		w, err := s.readByte()
		if err != nil {
			return err
		}
		return s.decodePositiveInt(uint64(w), v)

	// unsigned integer (two-byte uint16_t follows)
	case 0x19:
		w, err := s.readUint16()
		if err != nil {
			return err
		}
		return s.decodePositiveInt(uint64(w), v)

	// unsigned integer (four-byte uint32_t follows)
	case 0x1a:
		w, err := s.readUint32()
		if err != nil {
			return err
		}
		return s.decodePositiveInt(uint64(w), v)

	// unsigned integer (eight-byte uint64_t follows)
	case 0x1b:
		w, err := s.readUint64()
		if err != nil {
			return err
		}
		return s.decodePositiveInt(w, v)

	// negative integer -1-0x00..-1-0x17 (-1..-24)
	case 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37:
		return s.decodeNegativeInt(uint64(typ-0x20), v)

	// negative integer -1-n (one-byte uint8_t for n follows)
	case 0x38:
		w, err := s.readByte()
		if err != nil {
			return err
		}
		return s.decodeNegativeInt(uint64(w), v)

	// negative integer -1-n (two-byte uint16_t for n follows)
	case 0x39:
		w, err := s.readUint16()
		if err != nil {
			return err
		}
		return s.decodeNegativeInt(uint64(w), v)

	// negative integer -1-n (four-byte uint32_t for n follows)
	case 0x3a:
		w, err := s.readUint32()
		if err != nil {
			return err
		}
		return s.decodeNegativeInt(uint64(w), v)

	// negative integer -1-n (eight-byte uint64_t for n follows)
	case 0x3b:
		w, err := s.readUint64()
		if err != nil {
			return err
		}
		return s.decodeNegativeInt(w, v)

	// half-precision float (two-byte IEEE 754)
	case 0xf9:
		w, err := s.readUint16()
		if err != nil {
			return err
		}
		return s.decodeFloat16(uint16(w), v)

	// single-precision float (four-byte IEEE 754)
	case 0xfa:
		w, err := s.readUint32()
		if err != nil {
			return err
		}
		return s.decodeFloat32(uint32(w), v)

	// double-precision float (eight-byte IEEE 754)
	case 0xfb:
		w, err := s.readUint64()
		if err != nil {
			return err
		}
		return s.decodeFloat64(uint64(w), v)
	}
	return nil
}

func (d *decodeState) decodePositiveInt(w uint64, v reflect.Value) error {
	v = indirect(v)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.OverflowInt(int64(w)) {
			return errors.New("cbor: err") // TODO: introduce InvalidUnmarshalError
		}
		v.SetInt(int64(w))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.OverflowUint(w) {
			return errors.New("cbor: err") // TODO: introduce InvalidUnmarshalError
		}
		v.SetUint(w)
	}
	return nil
}

func (d *decodeState) decodeNegativeInt(w uint64, v reflect.Value) error {
	v = indirect(v)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := int64(^w)
		if i >= 0 || v.OverflowInt(i) {
			return errors.New("cbor: err") // TODO: introduce InvalidUnmarshalError
		}
		v.SetInt(i)
	}
	return nil
}

func (d *decodeState) decodeFloat16(w uint16, v reflect.Value) error {
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
	return d.decodeFloat(f, v)
}

func (d *decodeState) decodeFloat32(w uint32, v reflect.Value) error {
	f := math.Float32frombits(w)
	return d.decodeFloat(float64(f), v)
}

func (d *decodeState) decodeFloat64(w uint64, v reflect.Value) error {
	f := math.Float64frombits(w)
	return d.decodeFloat(f, v)
}

func (d *decodeState) decodeFloat(f float64, v reflect.Value) error {
	v = indirect(v)
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		if v.OverflowFloat(f) {
			return errors.New("cbor: err") // TODO: introduce InvalidUnmarshalError
		}
		v.SetFloat(f)
	}
	return nil
}
