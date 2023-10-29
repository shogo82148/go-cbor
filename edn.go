package cbor

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
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
	}
}
