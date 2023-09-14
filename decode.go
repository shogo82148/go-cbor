package cbor

import (
	"encoding/binary"
	"errors"
	"io"
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
	case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17:
		return s.decodePositiveInt(uint64(typ), v)
	case 0x18:
		w, err := s.readByte()
		if err != nil {
			return err
		}
		return s.decodePositiveInt(uint64(w), v)
	case 0x19:
		w, err := s.readUint16()
		if err != nil {
			return err
		}
		return s.decodePositiveInt(uint64(w), v)
	case 0x1a:
		w, err := s.readUint32()
		if err != nil {
			return err
		}
		return s.decodePositiveInt(uint64(w), v)
	case 0x1b:
		w, err := s.readUint64()
		if err != nil {
			return err
		}
		return s.decodePositiveInt(w, v)
	case 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37:
		return s.decodeNegativeInt(uint64(typ-0x20), v)
	case 0x38:
		w, err := s.readByte()
		if err != nil {
			return err
		}
		return s.decodeNegativeInt(uint64(w), v)
	case 0x39:
		w, err := s.readUint16()
		if err != nil {
			return err
		}
		return s.decodeNegativeInt(uint64(w), v)
	case 0x3a:
		w, err := s.readUint32()
		if err != nil {
			return err
		}
		return s.decodeNegativeInt(uint64(w), v)
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
		if i >= 0 || v.OverflowInt(int64(^w)) {
			return errors.New("cbor: err") // TODO: introduce InvalidUnmarshalError
		}
		v.SetInt(int64(^w))
	}
	return nil
}
