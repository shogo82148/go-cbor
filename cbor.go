package cbor

import (
	"errors"
	"math"
	"math/big"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"time"
)

var anySliceType = reflect.TypeOf([]any(nil))
var anyType = reflect.TypeOf((*any)(nil)).Elem()
var bigFloatType = reflect.TypeOf(big.Float{})
var bigIntType = reflect.TypeOf(big.Int{})
var byteType = reflect.TypeOf(byte(0))
var integerType = reflect.TypeOf(Integer{})
var simpleType = reflect.TypeOf(Simple(0))
var tagType = reflect.TypeOf(Tag{})
var timeType = reflect.TypeOf(time.Time{})
var undefinedType = reflect.TypeOf(Undefined)
var urlType = reflect.TypeOf(url.URL{})

var base64StringType = reflect.TypeOf(Base64String(""))
var base64URLStringType = reflect.TypeOf(Base64URLString(""))
var encodedData = reflect.TypeOf(EncodedData(nil))
var expectedBase16Type = reflect.TypeOf(ExpectedBase16{})
var expectedBase64Type = reflect.TypeOf(ExpectedBase64{})
var expectedBase64URLType = reflect.TypeOf(ExpectedBase64URL{})

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

// BigInt returns the integer as *big.Int.
func (i Integer) BigInt() *big.Int {
	v := new(big.Int).SetUint64(i.Value)
	if i.Sign {
		v.Sub(minusOne, v)
	}
	return v
}

// EncodedData is a CBOR encoded data.
type EncodedData []byte

// ExpectedBase64URL is data expected to be encoded as base64url with no padding.
type ExpectedBase64URL struct {
	Content any
}

// ExpectedBase64 is data expected to be encoded as base64 with padding.
type ExpectedBase64 struct {
	Content any
}

// ExpectedBase16 is data expected to be encoded as base16.
type ExpectedBase16 struct {
	Content any
}

// Base64String is a base64 with padding encoded data.
type Base64String string

// Base64URLString is a base64url with no padding encoded string.
type Base64URLString string

// Simple is a CBOR simple type.
type Simple byte

// TagNumber is a CBOR tag number type.
type TagNumber uint64

const (
	tagNumberExpectedBase64URL TagNumber = 21
	tagNumberExpectedBase64    TagNumber = 22
	tagNumberExpectedBase16    TagNumber = 23
	tagNumberEncodedData       TagNumber = 24

	tagNumberURI          TagNumber = 32
	tagNumberBase64URL    TagNumber = 33
	tagNumberBase64       TagNumber = 34
	tagNumberSelfDescribe TagNumber = 55799
)

// Tag is a CBOR tag.
type Tag struct {
	Number  TagNumber
	Content any
}
