package cbor

import "bytes"

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
	switch v := v.(type) {
	case int:
		return s.encodeInt(int64(v))
	case uint:
		return s.encodeUint(uint64(v))
	case uint64:
		return s.encodeUint(v)
	}
	return nil
}

func (s *encodeState) encodeInt(v int64) error {
	if v >= 0 {
		return s.encodeUint(uint64(v))
	}
	v = -1 - v
	switch {
	case v < 24:
		// 0x20..0x37: negative integer (-1..-24)
		s.buf.WriteByte(byte(0x20 + v))
		return nil
	case v < 0x100:
		// 0x38: 1 byte negative integer
		s.buf.WriteByte(0x38)
		s.buf.WriteByte(byte(v))
		return nil
	case v < 0x10000:
		// 0x39: 2 byte negative integer
		s.buf.WriteByte(0x39)
		s.buf.WriteByte(byte(v >> 8))
		s.buf.WriteByte(byte(v))
		return nil
	case v < 0x100000000:
		// 0x3a: 4 byte negative integer
		s.buf.WriteByte(0x3a)
		s.buf.WriteByte(byte(v >> 24))
		s.buf.WriteByte(byte(v >> 16))
		s.buf.WriteByte(byte(v >> 8))
		s.buf.WriteByte(byte(v))
		return nil
	default:
		// 0x3b: 8 byte negative integer
		s.buf.WriteByte(0x3b)
		s.buf.WriteByte(byte(v >> 56))
		s.buf.WriteByte(byte(v >> 48))
		s.buf.WriteByte(byte(v >> 40))
		s.buf.WriteByte(byte(v >> 32))
		s.buf.WriteByte(byte(v >> 24))
		s.buf.WriteByte(byte(v >> 16))
		s.buf.WriteByte(byte(v >> 8))
		s.buf.WriteByte(byte(v))
		return nil
	}
}

func (s *encodeState) encodeUint(v uint64) error {
	switch {
	case v < 24:
		// 0x00..0x17: positive integer (0..23)
		s.buf.WriteByte(byte(v))
		return nil
	case v < 0x100:
		// 0x18: 1 byte positive integer
		s.buf.WriteByte(0x18)
		s.buf.WriteByte(byte(v))
		return nil
	case v < 0x10000:
		// 0x19: 2 byte positive integer
		s.buf.WriteByte(0x19)
		s.buf.WriteByte(byte(v >> 8))
		s.buf.WriteByte(byte(v))
		return nil
	case v < 0x100000000:
		// 0x1a: 4 byte positive integer
		s.buf.WriteByte(0x1a)
		s.buf.WriteByte(byte(v >> 24))
		s.buf.WriteByte(byte(v >> 16))
		s.buf.WriteByte(byte(v >> 8))
		s.buf.WriteByte(byte(v))
		return nil
	default:
		// 0x1b: 8 byte positive integer
		s.buf.WriteByte(0x1b)
		s.buf.WriteByte(byte(v >> 56))
		s.buf.WriteByte(byte(v >> 48))
		s.buf.WriteByte(byte(v >> 40))
		s.buf.WriteByte(byte(v >> 32))
		s.buf.WriteByte(byte(v >> 24))
		s.buf.WriteByte(byte(v >> 16))
		s.buf.WriteByte(byte(v >> 8))
		s.buf.WriteByte(byte(v))
		return nil
	}
}
