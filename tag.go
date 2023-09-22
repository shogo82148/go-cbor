package cbor

import (
	"errors"
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
	data, err := Marshal(tag.Content)
	if err != nil {
		return err
	}
	t := RawTag{Number: tag.Number, Content: data}
	return t.Decode(v)
}

type RawTag struct {
	Number  TagNumber
	Content RawMessage
}

func (tag RawTag) Decode(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}
	return tag.decodeReflectValue(rv.Elem())
}

func (tag RawTag) decodeReflectValue(v reflect.Value) error {
	isNull := tag.Content[0] == 0xf6 || tag.Content[0] == 0xf7 // null or undefined
	u, v := indirect(v, isNull)
	if u != nil {
		return u.UnmarshalCBOR([]byte(tag.Content))
	}

	switch tag.Number {

	// tag number 0: date/time string
	case tagNumberDatetimeString:
		var s string
		if err := Unmarshal([]byte(tag.Content), &s); err != nil {
			return wrapSemanticError("cbor: invalid datetime string", err)
		}
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return wrapSemanticError("cbor: invalid datetime string", err)
		}
		if t.Unix() < minEpoch || t.Unix() >= maxEpoch {
			return newSemanticError("cbor: invalid range of datetime")
		}
		if v.Type() == timeType {
			v.Set(reflect.ValueOf(t))
		}
		return errors.New("TODO: implement")

	// tag number 1: epoch-based date/time
	case tagNumberEpochDatetime:
		var a any
		if err := Unmarshal([]byte(tag.Content), &a); err != nil {
			return wrapSemanticError("cbor: invalid epoch-based datetime", err)
		}

		var t time.Time
		switch epoch := a.(type) {
		case int64:
			if epoch < minEpoch || epoch >= maxEpoch {
				return newSemanticError("cbor: invalid range of datetime")
			}
			t = time.Unix(epoch, 0)
		case Integer:
			i, err := epoch.Int64()
			if err != nil || i < minEpoch || i >= maxEpoch {
				return wrapSemanticError("cbor: invalid range of datetime", err)
			}
			t = time.Unix(i, 0)
		case float64:
			if epoch < minEpoch || epoch >= maxEpoch {
				return newSemanticError("cbor: invalid range of datetime")
			}
			i, f := math.Modf(epoch)
			t = time.Unix(int64(i), int64(math.RoundToEven(f*1e9)))
		default:
			newSemanticError("cbor: invalid epoch-based datetime")
		}
		if v.Type() == timeType {
			v.Set(reflect.ValueOf(t))
		}
		return errors.New("TODO: implement")

	// tag number 2: positive bignum
	case tagNumberPositiveBignum:
		var b []byte
		if err := Unmarshal([]byte(tag.Content), &b); err != nil {
			return wrapSemanticError("cbor: invalid positive bignum", err)
		}
		if v.Type() == bigIntType {
			i := v.Addr().Interface().(*big.Int)
			i.SetBytes(b)
			return nil
		}

		i := new(big.Int).SetBytes(b)
		switch v.Kind() {
		// TODO:
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if !i.IsInt64() {
				return newSemanticError("cbor: integer overflow")
			}
		default:
			return &UnmarshalTypeError{Value: "integer", Type: v.Type()}
		}

	// tag number 3: negative bignum
	case tagNumberNegativeBignum:
		var b []byte
		if err := Unmarshal([]byte(tag.Content), &b); err != nil {
			return wrapSemanticError("cbor: invalid positive bignum", err)
		}
		if v.Type() == bigIntType {
			i := v.Addr().Interface().(*big.Int)
			i.SetBytes(b)
			i.Sub(minusOne, i)
			return nil
		}
		i := new(big.Int).SetBytes(b)
		i.Sub(minusOne, i)

		switch v.Kind() {
		// TODO:
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if !i.IsInt64() {
				return newSemanticError("cbor: integer overflow")
			}
		default:
			return &UnmarshalTypeError{Value: "integer", Type: v.Type()}
		}

	// tag number 4: decimal fraction
	case tagNumberDecimalFraction:
		// TODO: implement
		return errors.New("TODO: implement")

	// tag number 5: bigfloat
	case tagNumberBigfloat:
		var a struct {
			_    struct{} `cbor:",toarray"`
			Exp  Integer
			Mant any
		}
		if err := Unmarshal([]byte(tag.Content), &a); err != nil {
			return wrapSemanticError("cbor: invalid bigfloat", err)
		}

		exp, err := a.Exp.Int64()
		if err != nil {
			return wrapSemanticError("cbor: invalid bigfloat", err)
		}

		var mant *big.Int
		switch x := a.Mant.(type) {
		case int64:
			mant = big.NewInt(x)
		case Integer:
			mant = x.BigInt()
		case *big.Int:
			mant = x
		default:
			return &UnmarshalTypeError{Value: "float", Type: v.Type()}
		}

		f := new(big.Float)
		f.SetInt(mant)
		f.SetMantExp(f, int(exp))

		if v.Type() == bigFloatType {
			f0 := v.Addr().Interface().(*big.Float)
			f0.Set(f)
			return nil
		}
		return errors.New("TODO: implement")

	// tag number 21: expected conversion to base64url
	case tagNumberExpectedBase64URL:
		t := v.Type()
		switch {
		case t == expectedBase64URLType:
			if err := Unmarshal([]byte(tag.Content), v.FieldByName("Content").Interface()); err != nil {
				return wrapSemanticError("cbor: invalid expected conversion to base64url", err)
			}
		case v.Kind() == reflect.Interface && expectedBase64URLType.Implements(t):
			var a any
			if err := Unmarshal([]byte(tag.Content), &a); err != nil {
				return wrapSemanticError("cbor: invalid expected conversion to base64url", err)
			}
			v.Set(reflect.ValueOf(ExpectedBase64URL{Content: a}))
		default:
			return &UnmarshalTypeError{Value: "base64url", Type: v.Type()}
		}
		return nil

	// tag number 22: expected conversion to base64
	case tagNumberExpectedBase64:
		var a any
		if err := Unmarshal([]byte(tag.Content), &a); err != nil {
			return wrapSemanticError("cbor: invalid expected conversion to base64url", err)
		}
		if v.Type() == expectedBase64Type {
			v.Set(reflect.ValueOf(ExpectedBase64{Content: a}))
		}
		return errors.New("TODO: implement")

	// tag number 23: expected conversion to base16
	case tagNumberExpectedBase16:
		var a any
		if err := Unmarshal([]byte(tag.Content), &a); err != nil {
			return wrapSemanticError("cbor: invalid expected conversion to base64url", err)
		}
		if v.Type() == expectedBase16Type {
			v.Set(reflect.ValueOf(ExpectedBase16{Content: a}))
		}
		return errors.New("TODO: implement")

	// tag number 24: encoded CBOR data item
	case tagNumberEncodedData:
		var b []byte
		if err := Unmarshal([]byte(tag.Content), &b); err != nil {
			return wrapSemanticError("cbor: invalid encoded data", err)
		}
		if v.Type() == encodedDataType {
			v.Set(reflect.ValueOf(EncodedData(b)))
		}
		return errors.New("TODO: implement")

	// tag number 32: URI
	case tagNumberURI:
		var s string
		if err := Unmarshal([]byte(tag.Content), &s); err != nil {
			return wrapSemanticError("cbor: invalid URI", err)
		}
		u, err := url.Parse(s)
		if err != nil {
			return wrapSemanticError("cbor: invalid URI", err)
		}
		if v.Type() == urlType {
			v.Set(reflect.ValueOf(*u))
		}
		return errors.New("TODO: implement")

	// tag number 33: base64url
	case tagNumberBase64URL:
		var s string
		if err := Unmarshal([]byte(tag.Content), &s); err != nil {
			return wrapSemanticError("cbor: invalid base64url", err)
		}
		_, err := b64url.DecodeString(s)
		if err != nil {
			return wrapSemanticError("cbor: invalid base64url", err)
		}
		if v.Type() == base64URLStringType {
			v.SetString(s)
		}
		return errors.New("TODO: implement")

	// tag number 34: base64
	case tagNumberBase64:
		var s string
		if err := Unmarshal([]byte(tag.Content), &s); err != nil {
			return wrapSemanticError("cbor: invalid base64url", err)
		}
		_, err := b64.DecodeString(s)
		if err != nil {
			return wrapSemanticError("cbor: invalid base64url", err)
		}
		if v.Type() == base64URLStringType {
			v.SetString(s)
		}
		return errors.New("TODO: implement")

	// tag number 55799 Self-Described CBOR
	case tagNumberSelfDescribe:
		var a any
		if err := Unmarshal([]byte(tag.Content), &a); err != nil {
			return wrapSemanticError("cbor: invalid self-described cbor", err)
		}
	}

	return nil
}
