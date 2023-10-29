package cbor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"
	"unicode/utf8"
)

// ParseEDN parses the Extended Diagnostic Notation encoded data and returns the result.
func ParseEDN(data []byte) (RawMessage, error) {
	return nil, nil
}

// EncodeEDN returns the Extended Diagnostic Notation encoding of msg.
func (m RawMessage) EncodeEDN() ([]byte, error) {
	s := ednState{data: m}
	s.encode()
	if s.err != nil {
		return nil, s.err
	}
	return s.buf.Bytes(), nil
}

type ednState struct {
	buf  bytes.Buffer
	data RawMessage
	off  int // next read offset in data
	err  error
}

func (s *ednState) readByte() (byte, error) {
	if !s.isAvailable(1) {
		return 0, ErrUnexpectedEnd
	}
	b := s.data[s.off]
	s.off++
	return b, nil
}

func (s *ednState) readUint16() (uint16, error) {
	if !s.isAvailable(2) {
		return 0, ErrUnexpectedEnd
	}
	b := binary.BigEndian.Uint16(s.data[s.off:])
	s.off += 2
	return b, nil
}

func (s *ednState) readUint32() (uint32, error) {
	if !s.isAvailable(4) {
		return 0, ErrUnexpectedEnd
	}
	b := binary.BigEndian.Uint32(s.data[s.off:])
	s.off += 4
	return b, nil
}

func (s *ednState) readUint64() (uint64, error) {
	if !s.isAvailable(8) {
		return 0, ErrUnexpectedEnd
	}
	b := binary.BigEndian.Uint64(s.data[s.off:])
	s.off += 8
	return b, nil
}

func (d *ednState) isAvailable(n uint64) bool {
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

func (s *ednState) encode() {
	if s.err != nil {
		return
	}

	typ, err := s.readByte()
	if err != nil {
		s.err = err
		return
	}
	switch typ {
	// unsigned integer 0x00..0x17 (0..23)
	case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17:
		b := s.buf.AvailableBuffer()
		b = strconv.AppendUint(b, uint64(typ), 10)
		s.buf.Write(b)

	// unsigned integer (one-byte uint8_t follows)
	case 0x18:
		w, err := s.readByte()
		if err != nil {
			s.err = err
			return
		}
		b := s.buf.AvailableBuffer()
		b = strconv.AppendUint(b, uint64(w), 10)
		s.buf.Write(b)

	// unsigned integer (two-byte uint16_t follows)
	case 0x19:
		w, err := s.readUint16()
		if err != nil {
			s.err = err
			return
		}
		b := s.buf.AvailableBuffer()
		b = strconv.AppendUint(b, uint64(w), 10)
		s.buf.Write(b)

	// unsigned integer (four-byte uint32_t follows)
	case 0x1a:
		w, err := s.readUint32()
		if err != nil {
			s.err = err
			return
		}
		b := s.buf.AvailableBuffer()
		b = strconv.AppendUint(b, uint64(w), 10)
		s.buf.Write(b)

	// unsigned integer (eight-byte uint64_t follows)
	case 0x1b:
		w, err := s.readUint64()
		if err != nil {
			s.err = err
			return
		}
		b := s.buf.AvailableBuffer()
		b = strconv.AppendUint(b, w, 10)
		s.buf.Write(b)

	// negative integer -1-0x00..-1-0x17 (-1..-24)
	case 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37:
		b := s.buf.AvailableBuffer()
		b = strconv.AppendInt(b, ^int64(typ&0x1f), 10)
		s.buf.Write(b)

	// negative integer (one-byte uint8_t follows)
	case 0x38:
		w, err := s.readByte()
		if err != nil {
			s.err = err
			return
		}
		b := s.buf.AvailableBuffer()
		b = strconv.AppendInt(b, ^int64(w), 10)
		s.buf.Write(b)

	// negative integer -1-n (two-byte uint16_t follows)
	case 0x39:
		w, err := s.readUint16()
		if err != nil {
			s.err = err
			return
		}
		b := s.buf.AvailableBuffer()
		b = strconv.AppendInt(b, ^int64(w), 10)
		s.buf.Write(b)

	// negative integer -1-n (four-byte uint32_t follows)
	case 0x3a:
		w, err := s.readUint32()
		if err != nil {
			s.err = err
			return
		}
		b := s.buf.AvailableBuffer()
		b = strconv.AppendInt(b, ^int64(w), 10)
		s.buf.Write(b)

	// negative integer -1-n (eight-byte uint64_t follows)
	case 0x3b:
		w, err := s.readUint64()
		if err != nil {
			s.err = err
			return
		}
		b := s.buf.AvailableBuffer()
		b = append(b, '-')
		if w == 0xffffffffffffffff {
			b = append(b, "18446744073709551616"...)
		} else {
			b = strconv.AppendUint(b, w+1, 10)
		}
		s.buf.Write(b)

	// byte string (0x00..0x17 bytes follow)
	case 0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f, 0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57:
		n := int(typ & 0x1f)
		s.convertBytes(uint64(n))

	// byte string (one-byte uint8_t for n follows)
	case 0x58:
		n, err := s.readByte()
		if err != nil {
			s.err = err
			return
		}
		s.convertBytes(uint64(n))

	// byte string (two-byte uint16_t for n follow)
	case 0x59:
		n, err := s.readUint16()
		if err != nil {
			s.err = err
			return
		}
		s.convertBytes(uint64(n))

	// byte string (four-byte uint32_t for n follow)
	case 0x5a:
		n, err := s.readUint32()
		if err != nil {
			s.err = err
			return
		}
		s.convertBytes(uint64(n))

	// byte string (eight-byte uint64_t for n follow)
	case 0x5b:
		n, err := s.readUint64()
		if err != nil {
			s.err = err
			return
		}
		s.convertBytes(n)

	// UTF-8 string (0x00..0x17 bytes follow)
	case 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77:
		s.convertString(uint64(typ & 0x1f))

	// UTF-8 string (one-byte uint8_t for n follows)
	case 0x78:
		n, err := s.readByte()
		if err != nil {
			s.err = err
			return
		}
		s.convertString(uint64(n))

	// UTF-8 string (two-byte uint16_t for n follow)
	case 0x79:
		n, err := s.readUint16()
		if err != nil {
			s.err = err
			return
		}
		s.convertString(uint64(n))

	// UTF-8 string (four-byte uint32_t for n follow)
	case 0x7a:
		n, err := s.readUint32()
		if err != nil {
			s.err = err
			return
		}
		s.convertString(uint64(n))

	// UTF-8 string (eight-byte uint64_t for n follow)
	case 0x7b:
		n, err := s.readUint64()
		if err != nil {
			s.err = err
			return
		}
		s.convertString(n)

	// array (0x00..0x17 data items follow)
	case 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f, 0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97:
		n := int(typ & 0x1f)
		s.convertArray(uint64(n))

	// array (one-byte uint8_t for n follows)
	case 0x98:
		n, err := s.readByte()
		if err != nil {
			s.err = err
			return
		}
		s.convertArray(uint64(n))

	// array (two-byte uint16_t for n follow)
	case 0x99:
		n, err := s.readUint16()
		if err != nil {
			s.err = err
			return
		}
		s.convertArray(uint64(n))

	// array (four-byte uint32_t for n follow)
	case 0x9a:
		n, err := s.readUint32()
		if err != nil {
			s.err = err
			return
		}
		s.convertArray(uint64(n))

	// array (eight-byte uint64_t for n follow)
	case 0x9b:
		n, err := s.readUint64()
		if err != nil {
			s.err = err
			return
		}
		s.convertArray(n)

	// map (0x00..0x17 pairs of data items follow)
	case 0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf, 0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7:
		n := int(typ & 0x1f)
		s.convertMap(uint64(n))

	// map (one-byte uint8_t for n follows)
	case 0xb8:
		n, err := s.readByte()
		if err != nil {
			s.err = err
			return
		}
		s.convertMap(uint64(n))

	// map (two-byte uint16_t for n follow)
	case 0xb9:
		n, err := s.readUint16()
		if err != nil {
			s.err = err
			return
		}
		s.convertMap(uint64(n))

	// map (four-byte uint32_t for n follow)
	case 0xba:
		n, err := s.readUint32()
		if err != nil {
			s.err = err
			return
		}
		s.convertMap(uint64(n))

	// map (eight-byte uint64_t for n follow)
	case 0xbb:
		n, err := s.readUint64()
		if err != nil {
			s.err = err
			return
		}
		s.convertMap(n)

	// tags
	case 0xc0, 0xc1, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf, 0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7:
		s.convertTag(uint64(typ & 0x1f))

	// 1 byte tag
	case 0xd8:
		n, err := s.readByte()
		if err != nil {
			s.err = err
			return
		}
		s.convertTag(uint64(n))

	// 2 byte tag
	case 0xd9:
		n, err := s.readUint16()
		if err != nil {
			s.err = err
			return
		}
		s.convertTag(uint64(n))

	// 4 byte tag
	case 0xda:
		n, err := s.readUint32()
		if err != nil {
			s.err = err
			return
		}
		s.convertTag(uint64(n))

	// 8 byte tag
	case 0xdb:
		n, err := s.readUint64()
		if err != nil {
			s.err = err
			return
		}
		s.convertTag(n)
	}
}

func (s *ednState) convertBytes(n uint64) {
	if !s.isAvailable(n) {
		s.err = ErrUnexpectedEnd
		return
	}
	off := s.off
	s.off += int(n)

	s.buf.WriteByte('h')
	s.buf.WriteByte('\'')
	b := s.buf.AvailableBuffer()
	if cap(b) >= int(n)*2 {
		b = b[:int(n)*2]
	} else {
		b = make([]byte, int(n)*2)
	}
	hex.Encode(b, s.data[off:s.off])
	s.buf.Write(b)
	s.buf.WriteByte('\'')
}

func (s *ednState) convertString(n uint64) {
	if !s.isAvailable(n) {
		s.err = ErrUnexpectedEnd
		return
	}
	off := s.off
	s.off += int(n)

	if !utf8.Valid(s.data[off:s.off]) {
		s.err = newSemanticError("cbor: invalid UTF-8 string")
		return
	}
	v := string(s.data[off:s.off])
	data, err := json.Marshal(v)
	if err != nil {
		s.err = err
		return
	}
	s.buf.Write(data)
}

func (s *ednState) convertArray(n uint64) {
	s.buf.WriteByte('[')
	for i := uint64(0); i < n; i++ {
		if i > 0 {
			s.buf.WriteByte(',')
			s.buf.WriteByte(' ')
		}
		s.encode()
		if s.err != nil {
			return
		}
	}
	s.buf.WriteByte(']')
}

func (s *ednState) convertMap(n uint64) {
	s.buf.WriteByte('{')
	for i := uint64(0); i < n; i++ {
		if i > 0 {
			s.buf.WriteByte(',')
			s.buf.WriteByte(' ')
		}
		s.encode()
		if s.err != nil {
			return
		}
		s.buf.WriteByte(':')
		s.buf.WriteByte(' ')
		s.encode()
		if s.err != nil {
			return
		}
	}
	s.buf.WriteByte('}')
}

func (s *ednState) convertTag(n uint64) {
	b := s.buf.AvailableBuffer()
	b = strconv.AppendUint(b, n, 10)
	s.buf.Write(b)
	s.buf.WriteByte('(')
	s.encode()
	if s.err != nil {
		return
	}
	s.buf.WriteByte(')')
}
