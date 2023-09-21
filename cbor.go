package cbor

import (
	"encoding/base64"
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

// epoch time for 10000-01-01T00:00:00Z
const year10000 = 253402300800

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
type EncodedData []byte

// Simple is a CBOR simple type.
type Simple byte

// TagNumber is a CBOR tag number type.
type TagNumber uint64

const (
	tagNumberDatetimeString  TagNumber = 0
	tagNumberEpochDatetime   TagNumber = 1
	tagNumberPositiveBignum  TagNumber = 2
	tagNumberNegativeBignum  TagNumber = 3
	tagNumberDecimalFraction TagNumber = 4
	tagNumberBigfloat        TagNumber = 5

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

func (tag Tag) Decode() (any, error) {
	switch tag.Number {
	// tag number 0: date/time string
	case tagNumberDatetimeString:
		s, ok := tag.Content.(string)
		if !ok {
			return nil, newSemanticError("cbor: invalid datetime string")
		}
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil, wrapSemanticError("cbor: invalid datetime string", err)
		}
		if t.Unix() < 0 || t.Unix() >= year10000 {
			return nil, newSemanticError("cbor: invalid range of datetime")
		}
		return t, nil

	// tag number 1: epoch-based date/time
	case tagNumberEpochDatetime:
		var t time.Time
		switch epoch := tag.Content.(type) {
		case int64:
			if epoch < 0 || epoch >= year10000 {
				return nil, newSemanticError("cbor: invalid range of datetime")
			}
			t = time.Unix(epoch, 0)
		case Integer:
			i, err := epoch.Int64()
			if err != nil || i < 0 || i >= year10000 {
				return nil, wrapSemanticError("cbor: invalid range of datetime", err)
			}
			t = time.Unix(i, 0)
		case float64:
			if epoch < 0 || epoch >= year10000 {
				return nil, newSemanticError("cbor: invalid range of datetime")
			}
			i, f := math.Modf(epoch)
			t = time.Unix(int64(i), int64(math.RoundToEven(f*1e9)))
		default:
			return nil, newSemanticError("cbor: invalid epoch-based datetime")
		}
		return t, nil

	// tag number 2: positive bignum
	case tagNumberPositiveBignum:
		b, ok := tag.Content.([]byte)
		if !ok {
			return nil, newSemanticError("cbor: invalid positive bignum")
		}
		return new(big.Int).SetBytes(b), nil

	// tag number 3: negative bignum
	case tagNumberNegativeBignum:
		b, ok := tag.Content.([]byte)
		if !ok {
			return nil, newSemanticError("cbor: invalid positive bignum")
		}
		i := new(big.Int).SetBytes(b)
		return i.Sub(minusOne, i), nil

	// tag number 4: decimal fraction
	case tagNumberDecimalFraction:
		// TODO: implement

	// tag number 5: bigfloat
	case tagNumberBigfloat:
		a, ok := tag.Content.([]any)
		if !ok {
			b, ok := tag.Content.([2]any)
			if !ok {
				return nil, newSemanticError("cbor: invalid bigfloat")
			}
			a = b[:]
		}
		if len(a) != 2 {
			return nil, newSemanticError("cbor: invalid bigfloat")
		}
		var exp int64
		switch x := a[0].(type) {
		case int64:
			exp = x
		case Integer:
			var err error
			exp, err = x.Int64()
			if err != nil {
				return nil, newSemanticError("cbor: invalid bigfloat")
			}
		default:
			return nil, newSemanticError("cbor: invalid bigfloat")
		}

		var mant *big.Int
		switch x := a[1].(type) {
		case int64:
			mant = big.NewInt(x)
		case Integer:
			mant = x.BigInt()
		case *big.Int:
			mant = x
		default:
			return nil, newSemanticError("cbor: invalid bigfloat")
		}

		f := new(big.Float)
		f.SetInt(mant)
		f.SetMantExp(f, int(exp))
		return f, nil

	// tag number 21: expected conversion to base64url
	case tagNumberExpectedBase64URL:
		return ExpectedBase64URL{Content: tag.Content}, nil

	// tag number 22: expected conversion to base64
	case tagNumberExpectedBase64:
		return ExpectedBase64{Content: tag.Content}, nil

	// tag number 23: expected conversion to base16
	case tagNumberExpectedBase16:
		return ExpectedBase16{Content: tag.Content}, nil

	// tag number 24: encoded CBOR data item
	case tagNumberEncodedData:
		b, ok := tag.Content.([]byte)
		if !ok {
			return nil, newSemanticError("cbor: invalid encoded data")
		}
		return EncodedData(b), nil

	// tag number 32: URI
	case tagNumberURI:
		s, ok := tag.Content.(string)
		if !ok {
			return nil, newSemanticError("cbor: invalid URI")
		}
		u, err := url.Parse(s)
		if err != nil {
			return nil, wrapSemanticError("cbor: invalid URI", err)
		}
		return u, nil

	// tag number 33: base64url
	case tagNumberBase64URL:
		s, ok := tag.Content.(string)
		if !ok {
			return nil, newSemanticError("cbor: invalid base64url")
		}
		_, err := base64.RawURLEncoding.Strict().DecodeString(s)
		if err != nil {
			return nil, wrapSemanticError("cbor: invalid base64url", err)
		}
		return Base64URLString(s), nil

	// tag number 34: base64
	case tagNumberBase64:
		s, ok := tag.Content.(string)
		if !ok {
			return nil, newSemanticError("cbor: invalid base64")
		}
		_, err := base64.StdEncoding.Strict().DecodeString(s)
		if err != nil {
			return nil, wrapSemanticError("cbor: invalid base64", err)
		}
		return Base64String(s), nil

	// tag number 55799 Self-Described CBOR
	case tagNumberSelfDescribe:
		// RFC 8949 Section 3.4.6.
		// It does not impart any special semantics on the data item that it encloses.
		return tag.Content, nil
	}
	return tag, nil
}
