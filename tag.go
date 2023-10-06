package cbor

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"net/netip"
	"net/url"
	"reflect"
	"slices"
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

	// RFC 9164
	tagNumberIPv4Address TagNumber = 52
	tagNumberIPv6Address TagNumber = 54
)

// Tag is a CBOR tag.
type Tag struct {
	Number  TagNumber
	Content any
}

// Decode decodes the tag content.
// The following tags are supported:
//
//   - tag number 0: date/time string is decoded as time.Time.
//   - tag number 1: epoch-based date/time is decoded as time.Time.
//   - tag number 2: positive bignum is decoded as *big.Int.
//   - tag number 3: negative bignum is decoded as *big.Int.
//   - tag number 4: decimal fraction is not implemented.
//   - tag number 5: bigfloat is decoded as *big.Float.
//   - tag number 21: expected conversion to base64url is decoded as ExpectedBase64URL.
//   - tag number 22: expected conversion to base64 is decoded as ExpectedBase64.
//   - tag number 23: expected conversion to base16 is decoded as ExpectedBase16.
//   - tag number 24: encoded CBOR data item is decoded as EncodedData.
//   - tag number 32: URI is decoded as *url.URL.
//   - tag number 33: base64url is decoded as Base64URLString.
//   - tag number 34: base64 is decoded as Base64String.
//   - tag number 55799: Self-Described CBOR return the content as is.
//
// Other tags returns tag itself.
func (tag Tag) Decode(v any, opts Options) error {
	data, err := Marshal(tag.Content)
	if err != nil {
		return err
	}
	t := RawTag{Number: tag.Number, Content: data}
	return t.Decode(v, opts)
}

type RawTag struct {
	Number  TagNumber
	Content RawMessage
}

// Decode decodes the tag content.
func (tag RawTag) Decode(v any, opts Options) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}
	return tag.decodeReflectValue(rv.Elem(), opts)
}

func (tag RawTag) decodeReflectValue(rv reflect.Value, opts Options) error {
	firstByte := tag.Content[0]
	mt := majorType(firstByte >> 5)
	d := newDecodeState(tag.Content)

	isNull := firstByte == 0xf6 || firstByte == 0xf7 // null or undefined
	u, rv := indirect(rv, isNull)
	if u != nil {
		return u.UnmarshalCBOR([]byte(tag.Content))
	}

	switch tag.Number {

	// tag number 0: date/time string
	case tagNumberDatetimeString:
		var s string
		if err := d.decode(&s); err != nil {
			return wrapSemanticError("cbor: invalid datetime string", err)
		}
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return wrapSemanticError("cbor: invalid datetime string", err)
		}
		if t.Unix() <= minEpoch || t.Unix() >= maxEpoch {
			return newSemanticError("cbor: invalid range of datetime")
		}

		rt := rv.Type()
		if rt == timeType {
			rv.Set(reflect.ValueOf(t))
			return nil
		}
		if rt.Kind() == reflect.Interface && timeType.Implements(rt) {
			rv.Set(reflect.ValueOf(t))
			return nil
		}
		return &UnmarshalTypeError{Value: "datetime", Type: rv.Type()}

	// tag number 1: epoch-based date/time
	case tagNumberEpochDatetime:
		var t time.Time
		switch mt {
		case majorTypePositiveInt, majorTypeNegativeInt:
			var epoch Integer
			if err := d.decode(&epoch); err != nil {
				return wrapSemanticError("cbor: invalid epoch-based datetime", err)
			}
			i, err := epoch.Int64()
			if err != nil || i <= minEpoch || i >= maxEpoch {
				return wrapSemanticError("cbor: invalid range of datetime", err)
			}
			t = time.Unix(i, 0)
		case majorTypeOther:
			var epoch float64
			if err := d.decode(&epoch); err != nil {
				return wrapSemanticError("cbor: invalid epoch-based datetime", err)
			}
			if !math.IsNaN(epoch) {
				if epoch <= minEpoch || epoch >= maxEpoch {
					return newSemanticError("cbor: invalid range of datetime")
				}
				i, f := math.Modf(epoch)
				t = time.Unix(int64(i), int64(math.RoundToEven(f*1e9)))
			}
		default:
			return newSemanticError("cbor: invalid epoch-based datetime")
		}

		rt := rv.Type()
		if rt == timeType {
			rv.Set(reflect.ValueOf(t))
			return nil
		}
		if rt.Kind() == reflect.Interface {
			if rt.NumMethod() == 0 {
				if t.IsZero() {
					rv.Set(reflect.Zero(rt))
				} else {
					rv.Set(reflect.ValueOf(t))
				}
			} else {
				return &UnmarshalTypeError{Value: "datetime", Type: rv.Type()}
			}
			return nil
		}
		return &UnmarshalTypeError{Value: "datetime", Type: rv.Type()}

	// tag number 2: positive bignum
	case tagNumberPositiveBignum:
		var b []byte
		if err := d.decode(&b); err != nil {
			return wrapSemanticError("cbor: invalid positive bignum", err)
		}
		if rv.Type() == bigIntType {
			i := rv.Addr().Interface().(*big.Int)
			i.SetBytes(b)
			return nil
		}

		i := new(big.Int).SetBytes(b)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if !i.IsInt64() || rv.OverflowInt(i.Int64()) {
				return newSemanticError("cbor: integer overflow")
			}
			rv.SetInt(i.Int64())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if !i.IsUint64() || rv.OverflowUint(i.Uint64()) {
				return newSemanticError("cbor: integer overflow")
			}
			rv.SetUint(i.Uint64())
		case reflect.Interface:
			if rv.NumMethod() == 0 {
				if i.IsInt64() {
					rv.Set(reflect.ValueOf(i.Int64()))
				} else {
					rv.Set(reflect.ValueOf(i))
				}
			} else if reflect.PointerTo(bigIntType).Implements(rv.Type()) {
				rv.Set(reflect.ValueOf(i))
			} else {
				return &UnmarshalTypeError{Value: "integer", Type: rv.Type()}
			}
		default:
			return &UnmarshalTypeError{Value: "integer", Type: rv.Type()}
		}

	// tag number 3: negative bignum
	case tagNumberNegativeBignum:
		var b []byte
		if err := d.decode(&b); err != nil {
			return wrapSemanticError("cbor: invalid positive bignum", err)
		}
		if rv.Type() == bigIntType {
			i := rv.Addr().Interface().(*big.Int)
			i.SetBytes(b)
			i.Sub(minusOne, i)
			return nil
		}

		i := new(big.Int).SetBytes(b)
		i.Sub(minusOne, i)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if !i.IsInt64() || rv.OverflowInt(i.Int64()) {
				return newSemanticError("cbor: integer overflow")
			}
			rv.SetInt(i.Int64())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return newSemanticError("cbor: integer overflow")
		case reflect.Interface:
			if rv.NumMethod() == 0 {
				if i.IsInt64() {
					rv.Set(reflect.ValueOf(i.Int64()))
				} else {
					rv.Set(reflect.ValueOf(i))
				}
			} else if reflect.PointerTo(bigIntType).Implements(rv.Type()) {
				rv.Set(reflect.ValueOf(i))
			} else {
				return &UnmarshalTypeError{Value: "integer", Type: rv.Type()}
			}
		default:
			return &UnmarshalTypeError{Value: "integer", Type: rv.Type()}
		}

	// tag number 4: decimal fraction
	case tagNumberDecimalFraction:
		// TODO: implement
		return errors.New("TODO: implement")

	// tag number 5: bigfloat
	case tagNumberBigfloat:
		var a []any
		if err := d.decode(&a); err != nil {
			return wrapSemanticError("cbor: invalid bigfloat", err)
		}
		if len(a) != 2 {
			return newSemanticError("cbor: invalid bigfloat")
		}

		exp, ok := a[0].(int64)
		if !ok {
			return newSemanticError("cbor: invalid bigfloat")
		}

		f := new(big.Float)
		switch x := a[1].(type) {
		case int64:
			f.SetInt64(x)
		case Integer:
			f.SetInt(x.BigInt())
		case *big.Int:
			f.SetInt(x)
		default:
			return &UnmarshalTypeError{Value: "float", Type: rv.Type()}
		}

		f.SetMantExp(f, int(exp))

		if rv.Type() == bigFloatType {
			rv.Set(reflect.ValueOf(*f))
			return nil
		}
		switch rv.Kind() {
		case reflect.Float32, reflect.Float64:
			fv, _ := f.Float64()
			if rv.OverflowFloat(fv) {
				return newSemanticError("cbor: float overflow")
			}
			rv.SetFloat(fv)
		case reflect.Interface:
			if rv.NumMethod() == 0 {
				fv, acc := f.Float64()
				if acc == big.Exact {
					rv.Set(reflect.ValueOf(fv))
				} else {
					rv.Set(reflect.ValueOf(f))
				}
			} else if reflect.PointerTo(bigFloatType).Implements(rv.Type()) {
				rv.Set(reflect.ValueOf(f))
			} else {
				return &UnmarshalTypeError{Value: "integer", Type: rv.Type()}
			}
		default:
			return &UnmarshalTypeError{Value: "integer", Type: rv.Type()}
		}

	// tag number 21: expected conversion to base64url
	case tagNumberExpectedBase64URL:
		opts.set(d)
		t := rv.Type()
		switch {
		case t == expectedBase64URLType:
			if err := d.decodeReflectValue(rv.FieldByName("Content")); err != nil {
				return wrapSemanticError("cbor: invalid expected conversion to base64url", err)
			}
		case rv.Kind() == reflect.Interface && expectedBase64URLType.Implements(t):
			var a any
			if err := d.decode(&a); err != nil {
				return wrapSemanticError("cbor: invalid expected conversion to base64url", err)
			}
			rv.Set(reflect.ValueOf(ExpectedBase64URL{Content: a}))
		default:
			return &UnmarshalTypeError{Value: "base64url", Type: rv.Type()}
		}
		return nil

	// tag number 22: expected conversion to base64
	case tagNumberExpectedBase64:
		opts.set(d)
		t := rv.Type()
		switch {
		case t == expectedBase64Type:
			if err := d.decodeReflectValue(rv.FieldByName("Content")); err != nil {
				return wrapSemanticError("cbor: invalid expected conversion to base64", err)
			}
		case rv.Kind() == reflect.Interface && expectedBase64Type.Implements(t):
			var a any
			if err := d.decode(&a); err != nil {
				return wrapSemanticError("cbor: invalid expected conversion to base64", err)
			}
			rv.Set(reflect.ValueOf(ExpectedBase64{Content: a}))
		default:
			return &UnmarshalTypeError{Value: "base64", Type: rv.Type()}
		}
		return nil

	// tag number 23: expected conversion to base16
	case tagNumberExpectedBase16:
		opts.set(d)
		t := rv.Type()
		switch {
		case t == expectedBase16Type:
			if err := d.decodeReflectValue(rv.FieldByName("Content")); err != nil {
				return wrapSemanticError("cbor: invalid expected conversion to base16", err)
			}
		case rv.Kind() == reflect.Interface && expectedBase16Type.Implements(t):
			var a any
			if err := d.decode(&a); err != nil {
				return wrapSemanticError("cbor: invalid expected conversion to base16", err)
			}
			rv.Set(reflect.ValueOf(ExpectedBase16{Content: a}))
		default:
			return &UnmarshalTypeError{Value: "base16", Type: rv.Type()}
		}
		return nil

	// tag number 24: encoded CBOR data item
	case tagNumberEncodedData:
		t := rv.Type()
		if mt != majorTypeBytes {
			return newSemanticError("cbor: invalid encoded data")
		}
		switch {
		case t == encodedDataType:
			if err := d.decodeReflectValue(rv); err != nil {
				return wrapSemanticError("cbor: invalid encoded data", err)
			}
		case rv.Kind() == reflect.Interface && encodedDataType.Implements(t):
			var b []byte
			if err := Unmarshal([]byte(tag.Content), &b); err != nil {
				return wrapSemanticError("cbor: invalid encoded data", err)
			}
			rv.Set(reflect.ValueOf(EncodedData(b)))
		default:
			return &UnmarshalTypeError{Value: "encoded data", Type: rv.Type()}
		}
		return nil

	// tag number 32: URI
	case tagNumberURI:
		var s string
		if err := d.decode(&s); err != nil {
			return wrapSemanticError("cbor: invalid URI", err)
		}
		u, err := url.Parse(s)
		if err != nil {
			return wrapSemanticError("cbor: invalid URI", err)
		}

		t := rv.Type()
		switch {
		case t == urlType:
			rv.Set(reflect.ValueOf(*u))
		case rv.Kind() == reflect.Interface && reflect.PointerTo(urlType).Implements(t):
			rv.Set(reflect.ValueOf(u))
		default:
			return &UnmarshalTypeError{Value: "uri", Type: rv.Type()}
		}

	// tag number 33: base64url
	case tagNumberBase64URL:
		var s string
		if err := d.decode(&s); err != nil {
			return wrapSemanticError("cbor: invalid base64url", err)
		}
		_, err := b64url.DecodeString(s)
		if err != nil {
			return wrapSemanticError("cbor: invalid base64url", err)
		}

		t := rv.Type()
		switch {
		case t == base64URLStringType:
			rv.SetString(s)
		case rv.Kind() == reflect.Interface && base64URLStringType.Implements(t):
			rv.Set(reflect.ValueOf(Base64URLString(s)))
		default:
			return &UnmarshalTypeError{Value: "base64url", Type: rv.Type()}
		}

	// tag number 34: base64
	case tagNumberBase64:
		var s string
		if err := d.decode(&s); err != nil {
			return wrapSemanticError("cbor: invalid base64url", err)
		}
		_, err := b64.DecodeString(s)
		if err != nil {
			return wrapSemanticError("cbor: invalid base64url", err)
		}

		t := rv.Type()
		switch {
		case t == base64StringType:
			rv.SetString(s)
		case rv.Kind() == reflect.Interface && base64URLStringType.Implements(t):
			rv.Set(reflect.ValueOf(Base64String(s)))
		default:
			return &UnmarshalTypeError{Value: "base64url", Type: rv.Type()}
		}

	// tag number 52: IPv4 address
	case tagNumberIPv4Address:
		if mt == majorTypeBytes {
			var b []byte
			if err := d.decode(&b); err != nil {
				return wrapSemanticError("cbor: invalid IPv4 address", err)
			}
			if len(b) != 4 {
				return newSemanticError("cbor: invalid IPv4 address")
			}
			var b4 [4]byte
			copy(b4[:], b)
			addr := netip.AddrFrom4(b4)

			t := rv.Type()
			switch {
			case t == netipAddrType:
				rv.Set(reflect.ValueOf(addr))
			case rv.Kind() == reflect.Interface && netipAddrType.Implements(t):
				rv.Set(reflect.ValueOf(addr))
			default:
				return &UnmarshalTypeError{Value: "IPv4 address", Type: rv.Type()}
			}
		} else if mt == majorTypeArray {
			var a []any
			if err := d.decode(&a); err != nil {
				return wrapSemanticError("cbor: invalid IPv4 address", err)
			}
			if len(a) != 2 && len(a) != 3 {
				return newSemanticError("cbor: invalid IPv4 address")
			}
			if bits, ok := a[0].(int64); ok {
				if bits < 0 || bits > 32 {
					return newSemanticError("cbor: invalid IPv4 prefix")
				}
				b, ok := a[1].([]byte)
				if !ok || len(b) > 4 || (len(b) > 0 && b[len(b)-1] == 0x00) {
					return newSemanticError("cbor: invalid IPv4 prefix")
				}
				var b4 [4]byte
				copy(b4[:], b)
				u32 := binary.BigEndian.Uint32(b4[:])
				if (u32 << bits) != 0 {
					return newSemanticError("cbor: invalid IPv4 prefix")
				}

				addr := netip.AddrFrom4(b4)
				prefix := netip.PrefixFrom(addr, int(bits))
				rv.Set(reflect.ValueOf(prefix))
			}
		}

	// tag number 54: IPv6 address
	case tagNumberIPv6Address:

	// tag number 55799 Self-Described CBOR
	case tagNumberSelfDescribe:
		opts.set(d)
		if err := d.decodeReflectValue(rv); err != nil {
			return err
		}

	default:
		switch rv.Type() {
		case tagType:
			rv.FieldByName("Number").SetUint(uint64(tag.Number))
			return d.decodeReflectValue(rv.FieldByName("Content"))
		case rawTagType:
			contentStart := d.off
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
			rv.FieldByName("Number").SetUint(uint64(tag.Number))
			rv.FieldByName("Content").SetBytes(slices.Clone(d.data[contentStart:d.off]))
			return nil
		}
		if rv.Kind() == reflect.Interface && rawTagType.Implements(rv.Type()) {
			contentStart := d.off
			if err := d.checkWellFormedChild(); err != nil {
				return err
			}
			v := RawTag{
				Number:  tag.Number,
				Content: RawMessage(slices.Clone(d.data[contentStart:d.off])),
			}
			rv.Set(reflect.ValueOf(v))
			return nil
		}
		return &UnmarshalTypeError{Value: "tag", Type: rv.Type()}
	}

	return nil
}
