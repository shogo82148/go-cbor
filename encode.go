package cbor

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"sync"
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
	case undefined:
		return s.encodeUndefined()
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
	}
	return nil
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

func sliceEncoder(e *encodeState, v reflect.Value) error {
	l := v.Len()
	switch {
	case l < 0x17:
		e.writeByte(byte(0x80 + l))
	case l < 0x100:
		e.writeByte(0x98)
		e.writeByte(byte(l))
	case l < 0x10000:
		e.writeByte(0x99)
		e.writeUint16(uint16(l))
	case l < 0x100000000:
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

func (s *encodeState) encodeInt(v int64) error {
	if v >= 0 {
		return s.encodeUint(uint64(v))
	}
	v = -1 - v
	switch {
	case v < 24:
		// 0x20..0x37: negative integer (-1..-24)
		s.writeByte(byte(0x20 + v))
		return nil
	case v < 0x100:
		// 0x38: 1 byte negative integer
		s.writeByte(0x38)
		s.writeByte(byte(v))
		return nil
	case v < 0x10000:
		// 0x39: 2 byte negative integer
		s.writeByte(0x39)
		s.writeUint16(uint16(v))
		return nil
	case v < 0x100000000:
		// 0x3a: 4 byte negative integer
		s.writeByte(0x3a)
		s.writeUint32(uint32(v))
		return nil
	default:
		// 0x3b: 8 byte negative integer
		s.writeByte(0x3b)
		s.writeUint64(uint64(v))
		return nil
	}
}

func (s *encodeState) encodeUint(v uint64) error {
	switch {
	case v < 24:
		// 0x00..0x17: positive integer (0..23)
		s.writeByte(byte(v))
		return nil
	case v < 0x100:
		// 0x18: 1 byte positive integer
		s.writeByte(0x18)
		s.writeByte(byte(v))
		return nil
	case v < 0x10000:
		// 0x19: 2 byte positive integer
		s.writeByte(0x19)
		s.writeUint16(uint16(v))
		return nil
	case v < 0x100000000:
		// 0x1a: 4 byte positive integer
		s.writeByte(0x1a)
		s.writeUint32(uint32(v))
		return nil
	default:
		// 0x1b: 8 byte positive integer
		s.writeByte(0x1b)
		s.writeUint64(uint64(v))
		return nil
	}
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

func (s *encodeState) encodeUndefined() error {
	s.writeByte(0xf7) // undefined
	return nil
}

func (s *encodeState) encodeBytes(v []byte) error {
	l := len(v)
	switch {
	case l < 0x17:
		s.writeByte(byte(0x40 + l))
	case l < 0x100:
		s.writeByte(0x58)
		s.writeByte(byte(l))
	case l < 0x10000:
		s.writeByte(0x59)
		s.writeUint16(uint16(l))
	case l < 0x100000000:
		s.writeByte(0x5a)
		s.writeUint32(uint32(l))
	default:
		s.writeByte(0x5b)
		s.writeUint64(uint64(l))
	}
	s.buf.Write(v)
	return nil
}

func (s *encodeState) encodeString(v string) error {
	l := len(v)
	switch {
	case l < 0x17:
		s.writeByte(byte(0x60 + l))
	case l < 0x100:
		s.writeByte(0x78)
		s.writeByte(byte(l))
	case l < 0x10000:
		s.writeByte(0x79)
		s.writeUint16(uint16(l))
	case l < 0x100000000:
		s.writeByte(0x7a)
		s.writeUint32(uint32(l))
	default:
		s.writeByte(0x7b)
		s.writeUint64(uint64(l))
	}
	s.buf.WriteString(v)
	return nil
}
