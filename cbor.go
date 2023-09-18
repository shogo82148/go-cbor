package cbor

import (
	"errors"
	"math"
	"math/big"
	"reflect"
	"slices"
	"strconv"
	"time"
)

var anySliceType = reflect.TypeOf([]any(nil))
var anyType = reflect.TypeOf((*any)(nil)).Elem()
var bigIntType = reflect.TypeOf((*big.Int)(nil))
var byteType = reflect.TypeOf(byte(0))
var integerType = reflect.TypeOf(Integer{})
var simpleType = reflect.TypeOf(Simple(0))
var tagType = reflect.TypeOf(Tag{})
var timeType = reflect.TypeOf((*time.Time)(nil)).Elem()
var undefinedType = reflect.TypeOf(Undefined)

var minusOne = big.NewInt(-1)

type undefined *struct{}

var Undefined undefined = nil

// RawMessage is a raw encoded CBOR value. It implements Marshaler and
// Unmarshaler and can be used to delay CBOR decoding or precompute a CBOR
// encoding.
type RawMessage []byte

// MarshalCBOR returns m as the CBOR encoding of m.
func (m RawMessage) MarshalCBOR() ([]byte, error) {
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

// Simple is a CBOR simple type.
type Simple byte

// TagNumber is a CBOR tag number type.
type TagNumber uint64

// Tag is a CBOR tag.
type Tag struct {
	Number  TagNumber
	Content any
}
