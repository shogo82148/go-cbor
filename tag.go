package cbor

import (
	"encoding/base64"
	"math"
	"math/big"
	"net/url"
	"reflect"
	"time"
)

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

func (tag Tag) Decode(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}
	return tag.decodeReflectValue(rv.Elem())
}

func (tag Tag) decodeReflectValue(rv reflect.Value) error {
	switch tag.Number {
	default:
		// TODO: do not use decode.
		decoded, err := tag.decode()
		if err != nil {
			return err
		}
		rv.Set(reflect.ValueOf(decoded))
	}
	return nil
}

func (tag Tag) decode() (any, error) {
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
		if t.Unix() < minEpoch || t.Unix() >= maxEpoch {
			return nil, newSemanticError("cbor: invalid range of datetime")
		}
		return t, nil

	// tag number 1: epoch-based date/time
	case tagNumberEpochDatetime:
		var t time.Time
		switch epoch := tag.Content.(type) {
		case int64:
			if epoch < minEpoch || epoch >= maxEpoch {
				return nil, newSemanticError("cbor: invalid range of datetime")
			}
			t = time.Unix(epoch, 0)
		case Integer:
			i, err := epoch.Int64()
			if err != nil || i < minEpoch || i >= maxEpoch {
				return nil, wrapSemanticError("cbor: invalid range of datetime", err)
			}
			t = time.Unix(i, 0)
		case float64:
			if math.IsNaN(epoch) {
				return Undefined, nil
			}
			if epoch < minEpoch || epoch >= maxEpoch {
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
		i := new(big.Int).SetBytes(b)
		if i.IsInt64() {
			return i.Int64(), nil
		}
		return i, nil

	// tag number 3: negative bignum
	case tagNumberNegativeBignum:
		b, ok := tag.Content.([]byte)
		if !ok {
			return nil, newSemanticError("cbor: invalid positive bignum")
		}
		i := new(big.Int).SetBytes(b)
		i.Sub(minusOne, i)
		if i.IsInt64() {
			return i.Int64(), nil
		}
		return i, nil

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

type RawTag struct {
	Number  TagNumber
	Content RawMessage
}
