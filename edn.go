package cbor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"strconv"
	"unicode/utf8"

	"github.com/shogo82148/float16"
)

type encodingIndicator int

// DecodeEDN parses the Extended Diagnostic Notation encoded data and returns the result.
func DecodeEDN(data []byte) (RawMessage, error) {
	s := ednDecState{data: data}
	s.decode()
	if s.err != nil {
		return nil, s.err
	}
	return s.buf.Bytes(), nil
}

type ednDecState struct {
	buf  bytes.Buffer
	data []byte
	off  int // next read offset in data
	err  error
}

func (d *ednDecState) readByte() (byte, error) {
	if d.off >= len(d.data) {
		return 0, ErrUnexpectedEnd
	}
	b := d.data[d.off]
	d.off++
	return b, nil
}

func (d *ednDecState) peekByte() (byte, error) {
	if d.off >= len(d.data) {
		return 0, ErrUnexpectedEnd
	}
	b := d.data[d.off]
	return b, nil
}

func (s *ednDecState) readRune() (rune, error) {
	if s.off >= len(s.data) {
		return 0, ErrUnexpectedEnd
	}
	r, size := utf8.DecodeRune(s.data[s.off:])
	s.off += size
	return r, nil
}

func (s *ednDecState) writeByte(v byte) {
	s.buf.WriteByte(v)
}

func (s *ednDecState) writeUint16(v uint16) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	s.buf.Write(buf[:])
}

func (s *ednDecState) writeUint32(v uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	s.buf.Write(buf[:])
}

func (s *ednDecState) writeUint64(v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	s.buf.Write(buf[:])
}

func (d *ednDecState) writeUint(major majorType, ind encodingIndicator, v uint64) {
	bits := byte(major) << 5
	if ind < 0 && v < 24 {
		d.writeByte(bits | byte(v))
		return
	}

	switch {
	case v < 0x100:
		ind = max(ind, 0)
	case v < 0x10000:
		ind = max(ind, 1)
	case v < 0x100000000:
		ind = max(ind, 2)
	default:
		ind = max(ind, 3)
	}

	switch ind {
	case 0:
		d.writeByte(bits | 24)
		d.writeByte(byte(v))
	case 1:
		d.writeByte(bits | 25)
		d.writeUint16(uint16(v))
	case 2:
		d.writeByte(bits | 26)
		d.writeUint32(uint32(v))
	case 3:
		d.writeByte(bits | 27)
		d.writeUint64(v)
	}
}

// skipWhitespace skips the whitespace.
func (d *ednDecState) skipWhitespace() {
	for {
		ch, err := d.peekByte()
		if err != nil {
			d.err = err
			return
		}
		switch ch {
		case ' ', '\t', '\r', '\n':
			d.off++
		case '/':
			// comment
			d.off++
			for {
				ch, err := d.readByte()
				if err != nil {
					d.err = err
					return
				}
				if ch == '/' {
					break
				}
			}
		default:
			return
		}
	}
}

// expectWhitespace skips the whitespace and returns the next byte.
func (d *ednDecState) expectWhitespace() {
	ch, err := d.peekByte()
	if err != nil {
		d.err = err
		return
	}
	switch ch {
	case ' ', '\t', '\r', '\n':
		d.off++
	default:
		d.err = newSemanticError("cbor: expected whitespace")
		return
	}
	d.skipWhitespace()
}

func (s *ednDecState) decode() {
	s.skipWhitespace()
	if s.err != nil {
		return
	}

	ch, err := s.peekByte()
	if err != nil {
		s.err = err
		return
	}
	switch ch {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '+', '-':
		// integer or float
		s.decodeNumber()

	case 'N':
		if bytes.HasPrefix(s.data[s.off:], []byte("NaN")) {
			s.off += len("NaN")
			s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
			s.writeByte(0x7e)
			s.writeByte(0x00)
			return
		}

	case 'I':
		if bytes.HasPrefix(s.data[s.off:], []byte("Infinity")) {
			s.off += len("Infinity")
			s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
			s.writeByte(0x7c)
			s.writeByte(0x00)
			return
		}

	case 'f':
		if bytes.HasPrefix(s.data[s.off:], []byte("false")) {
			s.off += len("false")
			s.writeByte(0xf4) // false
			return
		}

	case 't':
		if bytes.HasPrefix(s.data[s.off:], []byte("true")) {
			s.off += len("true")
			s.writeByte(0xf5) // true
		}

	case 'n':
		if bytes.HasPrefix(s.data[s.off:], []byte("null")) {
			s.off += len("null")
			s.writeByte(0xf6) // null
		}

	case 'u':
		if bytes.HasPrefix(s.data[s.off:], []byte("undefined")) {
			s.off += len("undefined")
			s.writeByte(0xf7) // undefined
		}

	// byte string (hexadecimal format)
	case 'h':
		s.decodeBytes()

	// text string
	case '"':
		s.decodeString()

	// array
	case '[':
		s.decodeArray()
	}
}

func (s *ednDecState) decodeEncodingIndicator() encodingIndicator {
	ch, err := s.peekByte()
	if err != nil {
		s.err = err
		return -1
	}
	if ch != '_' {
		return -1
	}
	s.off++

	ch, err = s.peekByte()
	if err != nil {
		s.err = err
		return -1
	}
	switch ch {
	case '0', '1', '2', '3', '4', '5', '6', '7':
		s.off++
		s.expectWhitespace()
		return encodingIndicator(ch - '0')
	default:
		s.expectWhitespace()
		return 7
	}
}

func (s *ednDecState) decodeNumber() {
	// special numbers
	if bytes.HasPrefix(s.data[s.off:], []byte("-Infinity")) {
		s.off += len("-Infinity")
		s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
		s.writeByte(0xfc)
		s.writeByte(0x00)
		return
	}
	if bytes.HasPrefix(s.data[s.off:], []byte("+Infinity")) {
		s.off += len("+Infinity")
		s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
		s.writeByte(0x7c)
		s.writeByte(0x00)
		return
	}

	start := s.off
LOOP:
	for ; s.off < len(s.data); s.off++ {
		switch s.data[s.off] {
		// decimal numbers
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':

		// sign
		case '+', '-':

		// decimal point
		case '.':

		// hexadecimal numbers
		case 'a', 'b', 'c', 'd', 'e', 'f', 'A', 'B', 'C', 'D', 'E', 'F':

		// exponent
		case 'p', 'P':

		// prefix
		case 'x', 'X', 'o', 'O':

		default:
			break LOOP
		}
	}
	end := s.off

	// try to parse as an integer
	str := string(s.data[start:end])
	if s.tryToDecodeInteger(str) {
		return
	}

	// try to parse as a float
	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		s.err = err
		return
	}

	// encode to CBOR
	f64 := math.Float64bits(f)
	sign := f64 >> 63
	exp := int((f64>>52)&0x7ff) - 1023
	frac := f64 & 0xfffffffffffff

	if exp == -1023 && frac == 0 {
		// 0.0 in float16
		s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
		s.writeByte(byte(sign << 7))
		s.writeByte(0x00)
		return
	}

	// try converting to subnormal float16
	if -24 <= exp && exp < -14 {
		shift := -exp + 53 - 24 - 1
		if frac&((1<<shift)-1) == 0 {
			frac |= 1 << 52
			f16 := uint16(sign<<15 | frac>>shift)
			s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
			s.writeUint16(f16)
			return
		}
	}

	// try converting to normal float16
	if -14 <= exp && exp <= 15 {
		if frac&((1<<42)-1) == 0 {
			f16 := uint16(sign<<15 | uint64(exp+15)<<10 | frac>>42)
			s.writeByte(0xf9) // half-precision float (two-byte IEEE 754)
			s.writeUint16(f16)
			return
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
			return
		}
	}

	// try converting to normal float32
	if -126 <= exp && exp <= 127 {
		if frac&((1<<29)-1) == 0 {
			f32 := uint32(sign<<31 | uint64(exp+127)<<23 | frac>>29)
			s.writeByte(0xfa) // single-precision float (four-byte IEEE 754)
			s.writeUint32(f32)
			return
		}
	}

	// default to float64
	s.writeByte(0xfb) // double-precision float (eight-byte IEEE 754)
	s.writeUint64(f64)
}

func (s *ednDecState) tryToDecodeInteger(str string) bool {
	i, ok := new(big.Int).SetString(str, 0)
	if !ok {
		return false
	}

	if i.Sign() >= 0 {
		if i.IsUint64() {
			s.writeUint(majorTypePositiveInt, -1, i.Uint64())
		} else {
			// TODO: support big.Int
			s.err = newSemanticError("cbor: unsupported big.Int")
			return true
		}
	} else {
		i.Not(i)
		if i.IsUint64() {
			s.writeUint(majorTypeNegativeInt, -1, i.Uint64())
		} else {
			// TODO: support big.Int
			s.err = newSemanticError("cbor: unsupported big.Int")
			return true
		}
	}
	return true
}

func (s *ednDecState) decodeBytes() {
	if bytes.HasPrefix(s.data[s.off:], []byte("h'")) {
		// hexadecimal format
		s.off += len("h'")
		var buf bytes.Buffer
		for {
			s.skipWhitespace()
			if s.err != nil {
				return
			}
			ch, err := s.readByte()
			if err != nil {
				s.err = err
				return
			}
			if ch == '\'' {
				// end of byte string
				break
			}
			buf.WriteByte(ch)
		}
		data, err := hex.DecodeString(buf.String())
		if err != nil {
			s.err = err
			return
		}
		s.writeUint(majorTypeBytes, -1, uint64(len(data)))
		s.buf.Write(data)
		return
	}

	s.err = newSemanticError("cbor: invalid byte string")
}

func (s *ednDecState) decodeString() {
	start := s.off
	ch, err := s.peekByte()
	if err != nil {
		s.err = err
		return
	}
	if ch != '"' {
		s.err = newSemanticError("cbor: invalid string")
		return
	}
	s.off++

LOOP:
	for s.off < len(s.data) {
		ch, err := s.readRune()
		if err != nil {
			s.err = err
			return
		}
		switch ch {
		case '"':
			// end of string
			break LOOP

		case '\\':
			// escape sequences
			_, err := s.readRune()
			if err != nil {
				s.err = err
				return
			}
		}
	}
	end := s.off

	var str string
	if err := json.Unmarshal(s.data[start:end], &str); err != nil {
		s.err = err
		return
	}
	s.writeUint(majorTypeString, -1, uint64(len(str)))
	s.buf.WriteString(str)
}

func (s *ednDecState) decodeArray() {
	ch, err := s.peekByte()
	if err != nil {
		s.err = err
		return
	}
	if ch != '[' {
		s.err = newSemanticError("cbor: invalid array")
		return
	}
	s.off++

	ind := s.decodeEncodingIndicator()
	if s.err != nil {
		return
	}

	if ind == 7 {
		// indefinite length array
		s.buf.WriteByte(0x9f)
		for {
			s.skipWhitespace()
			if s.err != nil {
				return
			}
			ch, err := s.peekByte()
			if err != nil {
				s.err = err
				return
			}
			if ch == ']' {
				// end of array
				s.off++
				break
			}
			s.decode()
			if s.err != nil {
				return
			}
		}
		s.buf.WriteByte(0xff)
		return
	}

	t := &ednDecState{data: s.data, off: s.off}
	var count uint64
	for {
		t.skipWhitespace()
		if t.err != nil {
			s.err = t.err
			return
		}
		ch, err := t.peekByte()
		if err != nil {
			s.err = err
			return
		}
		if ch == ']' {
			// end of array
			t.off++
			break
		}
		if count > 0 {
			if ch == ',' {
				t.off++
			} else {
				s.err = newSemanticError("cbor: expected comma")
				return
			}
		}

		// next element
		count++
		t.decode()
		if t.err != nil {
			s.err = t.err
			return
		}
	}
	s.off = t.off
	s.writeUint(majorTypeArray, ind, count)
	t.buf.WriteTo(&s.buf)
}

// EncodeEDN returns the Extended Diagnostic Notation encoding of msg.
func (m RawMessage) EncodeEDN() ([]byte, error) {
	s := ednEncState{data: m}
	s.encode()
	if s.err != nil {
		return nil, s.err
	}
	return s.buf.Bytes(), nil
}

type ednEncState struct {
	buf  bytes.Buffer
	data RawMessage
	off  int // next read offset in data
	err  error
}

func (s *ednEncState) readByte() (byte, error) {
	if !s.isAvailable(1) {
		return 0, ErrUnexpectedEnd
	}
	b := s.data[s.off]
	s.off++
	return b, nil
}

func (s *ednEncState) peekByte() (byte, error) {
	if !s.isAvailable(1) {
		return 0, ErrUnexpectedEnd
	}
	return s.data[s.off], nil
}

func (s *ednEncState) readUint16() (uint16, error) {
	if !s.isAvailable(2) {
		return 0, ErrUnexpectedEnd
	}
	b := binary.BigEndian.Uint16(s.data[s.off:])
	s.off += 2
	return b, nil
}

func (s *ednEncState) readUint32() (uint32, error) {
	if !s.isAvailable(4) {
		return 0, ErrUnexpectedEnd
	}
	b := binary.BigEndian.Uint32(s.data[s.off:])
	s.off += 4
	return b, nil
}

func (s *ednEncState) readUint64() (uint64, error) {
	if !s.isAvailable(8) {
		return 0, ErrUnexpectedEnd
	}
	b := binary.BigEndian.Uint64(s.data[s.off:])
	s.off += 8
	return b, nil
}

func (d *ednEncState) isAvailable(n uint64) bool {
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

func (s *ednEncState) encode() {
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

	// byte string (indefinite length)
	case 0x5f:
		tye, err := s.peekByte()
		if err != nil {
			s.err = err
			return
		}
		if tye == 0xff {
			s.off++
			s.buf.WriteString("''_")
			break
		}
		if typ&0xe0 != 0x40 {
			s.err = newSemanticError("cbor: invalid byte string")
			return
		}
		s.buf.WriteString("(_ ")
		s.encode()
		for {
			typ, err := s.peekByte()
			if err != nil {
				s.err = err
				return
			}
			if typ == 0xff {
				s.off++
				break
			}
			if typ&0xe0 != 0x40 {
				s.err = newSemanticError("cbor: invalid byte string")
				return
			}
			s.buf.WriteString(", ")
			s.encode()
		}
		s.buf.WriteByte(')')

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

	// byte string (indefinite length)
	case 0x7f:
		tye, err := s.peekByte()
		if err != nil {
			s.err = err
			return
		}
		if tye == 0xff {
			s.off++
			s.buf.WriteString(`""_`)
			break
		}

		s.buf.WriteString("(_ ")
		s.encode()
		for {
			typ, err := s.peekByte()
			if err != nil {
				s.err = err
				return
			}
			if typ == 0xff {
				s.off++
				break
			}
			if typ&0xe0 != 0x60 {
				s.err = newSemanticError("cbor: invalid byte string")
				return
			}
			s.buf.WriteString(", ")
			s.encode()
		}
		s.buf.WriteByte(')')

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

	// array (indefinite length)
	case 0x9f:
		s.buf.WriteString("[_ ")
		first := true
		for {
			typ, err := s.peekByte()
			if err != nil {
				s.err = err
				return
			}
			if typ == 0xff {
				s.off++
				break
			}
			if !first {
				s.buf.WriteString(", ")
			}
			first = false
			s.encode()
		}
		s.buf.WriteByte(']')

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

	// map (indefinite length)
	case 0xbf:
		s.buf.WriteString("{_ ")
		first := true
		for {
			typ, err := s.peekByte()
			if err != nil {
				s.err = err
				return
			}
			if typ == 0xff {
				s.off++
				break
			}
			if !first {
				s.buf.WriteString(", ")
			}
			first = false
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

	// simple values
	case 0xe0, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef, 0xf0, 0xf3:
		s.buf.WriteString("simple(")
		b := s.buf.AvailableBuffer()
		b = strconv.AppendUint(b, uint64(typ&0x1f), 10)
		s.buf.Write(b)
		s.buf.WriteByte(')')

	// false
	case 0xf4:
		s.buf.WriteString("false")

	// true
	case 0xf5:
		s.buf.WriteString("true")

	// null
	case 0xf6:
		s.buf.WriteString("null")

	// undefined
	case 0xf7:
		s.buf.WriteString("undefined")

	// simple value (one-byte uint8_t follows)
	case 0xf8:
		n, err := s.readByte()
		if err != nil {
			s.err = err
			return
		}
		s.buf.WriteString("simple(")
		b := s.buf.AvailableBuffer()
		b = strconv.AppendUint(b, uint64(n), 10)
		s.buf.Write(b)
		s.buf.WriteByte(')')

	// half-precision float (two-byte IEEE 754)
	case 0xf9:
		w, err := s.readUint16()
		if err != nil {
			s.err = err
			return
		}
		f := float16.FromBits(w)
		s.convertFloat(f.Float64())

	// single-precision float (four-byte IEEE 754)
	case 0xfa:
		w, err := s.readUint32()
		if err != nil {
			s.err = err
			return
		}
		f := math.Float32frombits(w)
		s.convertFloat(float64(f))

	// double-precision float (eight-byte IEEE 754)
	case 0xfb:
		w, err := s.readUint64()
		if err != nil {
			s.err = err
			return
		}
		f := math.Float64frombits(w)
		s.convertFloat(f)
	}
}

func (s *ednEncState) convertBytes(n uint64) {
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

func (s *ednEncState) convertString(n uint64) {
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

func (s *ednEncState) convertArray(n uint64) {
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

func (s *ednEncState) convertMap(n uint64) {
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

func (s *ednEncState) convertTag(n uint64) {
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

func (s *ednEncState) convertFloat(v float64) {
	// special cases
	switch {
	case math.IsNaN(v):
		s.buf.WriteString("NaN")
		return
	case math.IsInf(v, 1):
		s.buf.WriteString("Infinity")
		return
	case math.IsInf(v, -1):
		s.buf.WriteString("-Infinity")
		return
	}

	str := strconv.FormatFloat(v, 'g', -1, 64)
	if _, err := strconv.ParseInt(str, 10, 64); err == nil {
		// float point number and integer should be distinguished.
		// e.g. float64(1) -> "1.0"
		str = strconv.FormatFloat(v, 'f', 1, 64)
	}
	s.buf.WriteString(str)
}
