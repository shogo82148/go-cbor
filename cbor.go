package cbor

import (
	"errors"
	"math"
	"math/big"
	"math/bits"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"time"
)

// maximum epoch time we accept (10000-01-01T00:00:00Z) excluded
const maxEpoch = 253402300800

// minimum epoch time we accept (0000-01-01T00:00:00Z) excluded
const minEpoch = -62135596800

var anySliceType = reflect.TypeOf([]any(nil))
var anyType = reflect.TypeOf((*any)(nil)).Elem()
var bigFloatType = reflect.TypeOf(big.Float{})
var bigIntType = reflect.TypeOf(big.Int{})
var byteType = reflect.TypeOf(byte(0))
var integerType = reflect.TypeOf(Integer{})
var rawTagType = reflect.TypeOf(RawTag{})
var simpleType = reflect.TypeOf(Simple(0))
var tagType = reflect.TypeOf(Tag{})
var timeType = reflect.TypeOf(time.Time{})
var undefinedType = reflect.TypeOf(Undefined)
var urlType = reflect.TypeOf(url.URL{})

var base64StringType = reflect.TypeOf(Base64String(""))
var base64URLStringType = reflect.TypeOf(Base64URLString(""))
var encodedDataType = reflect.TypeOf(EncodedData(nil))
var expectedBase16Type = reflect.TypeOf(ExpectedBase16{})
var expectedBase64Type = reflect.TypeOf(ExpectedBase64{})
var expectedBase64URLType = reflect.TypeOf(ExpectedBase64URL{})

var minusOne = big.NewInt(-1)

type undefined *struct{}

var Undefined undefined = nil

// RawMessage is a raw encoded CBOR value. It implements Marshaler and
// Unmarshaler and can be used to delay CBOR decoding or precompute a CBOR
// encoding.
// nil RawMessage encodes as the CBOR undefined value.
type RawMessage []byte

// MarshalCBOR returns m as the CBOR encoding of m.
func (m RawMessage) MarshalCBOR() ([]byte, error) {
	if m == nil {
		return []byte{0xf6}, nil // null
	}
	return []byte(m), nil
}

// UnmarshalCBOR sets *m to a copy of data.
func (m *RawMessage) UnmarshalCBOR(data []byte) error {
	*m = slices.Clone(data)
	return nil
}

// Integer is a CBOR integer type.
type Integer struct {
	// Sign is true if the integer is negative.
	Sign bool

	// Value presents the value of the integer.
	// If the integer is positive, the value is Value itself.
	// If the integer is negative, the value is -Value-1.
	Value uint64
}

// Int64 returns the integer as int64.
func (i Integer) Int64() (int64, error) {
	if i.Sign {
		v := int64(^i.Value)
		if v >= 0 {
			return 0, errors.New("cbor: integer overflow")
		}
		return v, nil
	} else {
		v := int64(i.Value)
		if v < 0 {
			return 0, errors.New("cbor: integer overflow")
		}
		return v, nil
	}
}

// Uint64 returns the integer as uint64.
func (i Integer) Uint64() (uint64, error) {
	if i.Sign {
		return 0, errors.New("cbor: integer overflow")
	} else {
		return i.Value, nil
	}
}

func (i Integer) String() string {
	if i.Sign {
		if i.Value == math.MaxUint64 {
			return "-18446744073709551616"
		}
		return "-" + strconv.FormatUint(i.Value+1, 10)
	} else {
		return strconv.FormatUint(i.Value, 10)
	}
}

// BigInt returns the integer as *big.Int.
func (i Integer) BigInt() *big.Int {
	v := new(big.Int).SetUint64(i.Value)
	if i.Sign {
		v.Sub(minusOne, v)
	}
	return v
}

func (i Integer) MarshalJSON() ([]byte, error) {
	var buf []byte
	if i.Sign {
		if i.Value == math.MaxUint64 {
			return []byte(`-18446744073709551616`), nil
		}
		buf = append(buf, '-')
		return strconv.AppendUint(buf, i.Value+1, 10), nil
	} else {
		return strconv.AppendUint(buf, i.Value, 10), nil
	}
}

func (i *Integer) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return errors.New("cbor: empty JSON string")
	}
	if b[0] == '-' {
		i.Sign = true
		b = b[1:]
	} else if b[0] == '+' {
		i.Sign = false
		b = b[1:]
	} else if '0' <= b[0] && b[0] <= '9' {
		i.Sign = false
	} else {
		return errors.New("cbor: invalid JSON string")
	}

	// parse the integer as uint128
	var hi, lo uint64
	for _, c := range b {
		var carry uint64
		if c < '0' || '9' < c {
			return errors.New("cbor: invalid JSON string")
		}
		d := int64(c - '0')

		// hi, lo = (hi, lo) * 10 + d
		h, l := bits.Mul64(lo, 10)
		hi += h
		lo, carry = bits.Add64(l, uint64(d), 0)
		hi, _ = bits.Add64(hi, 0, carry)

		if hi > 0 && lo > 0 {
			return errors.New("cbor: integer overflow")
		}
	}

	if i.Sign {
		if lo == 0 && hi == 0 {
			i.Sign = false
			i.Value = 0
			return nil
		}
		var borrow uint64
		lo, borrow = bits.Sub64(lo, 1, 0)
		hi, _ = bits.Sub64(hi, 0, borrow)
		if hi > 0 {
			return errors.New("cbor: integer overflow")
		}
		i.Value = lo
	} else {
		if hi > 0 {
			return errors.New("cbor: integer overflow")
		}
		i.Value = lo
	}
	return nil
}

// EncodedData is a CBOR encoded data.
// CBOR tags that has tag number 24 is converted to this type.
// See RFC 8949 Section 3.4.5.1.
type EncodedData []byte

// Simple is a CBOR simple type.
type Simple byte
