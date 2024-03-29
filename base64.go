package cbor

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
)

var b64 = base64.StdEncoding.Strict()
var b64url = base64.RawURLEncoding.Strict()

type encMode int

const (
	encModeBase64 encMode = iota
	encModeBase64URL
	encModeBase16
)

func (enc encMode) Encode(data []byte) string {
	switch enc {
	case encModeBase64:
		return b64.EncodeToString(data)
	case encModeBase64URL:
		return b64url.EncodeToString(data)
	case encModeBase16:
		return hex.EncodeToString(data)
	}
	return ""
}

type b64ctx struct {
	mode encMode
}

func (ctx b64ctx) Convert(data any) any {
	switch data := data.(type) {
	case []byte:
		return ctx.mode.Encode(data)

	case map[string]any:
		ret := make(map[string]any, len(data))
		for k, v := range data {
			ret[k] = ctx.Convert(v)
		}
		return ret

	case []any:
		ret := make([]any, len(data))
		for i, v := range data {
			ret[i] = ctx.Convert(v)
		}
		return ret

	case ExpectedBase64URL:
		orig := ctx.mode
		ctx.mode = encModeBase64URL
		ret := ctx.Convert(data.Content)
		ctx.mode = orig
		return ret

	case ExpectedBase64:
		orig := ctx.mode
		ctx.mode = encModeBase64
		ret := ctx.Convert(data.Content)
		ctx.mode = orig
		return ret

	case ExpectedBase16:
		orig := ctx.mode
		ctx.mode = encModeBase16
		ret := ctx.Convert(data.Content)
		ctx.mode = orig
		return ret
	}
	return data
}

var _ json.Marshaler = ExpectedBase64URL{}
var _ json.Marshaler = ExpectedBase64{}
var _ json.Marshaler = ExpectedBase16{}

// ExpectedBase64URL is data expected to be encoded as base64url-encoding.
// CBOR tags that has tag number 21 is converted to this type.
// When encoded to JSON, []byte in Content is converted to a base64url-encoded string.
// See RFC 8949 Section 3.4.5.2.
type ExpectedBase64URL struct {
	Content any
}

func (e ExpectedBase64URL) MarshalJSON() ([]byte, error) {
	ctx := &b64ctx{mode: encModeBase64URL}
	data := ctx.Convert(e.Content)
	return json.Marshal(data)
}

// ExpectedBase64 is data expected to be encoded as base64-encoding.
// CBOR tags that has tag number 22 is converted to this type.
// When encoded to JSON, []byte in Content is converted to a base64-encoded string.
// See RFC 8949 Section 3.4.5.2.
type ExpectedBase64 struct {
	Content any
}

func (e ExpectedBase64) MarshalJSON() ([]byte, error) {
	ctx := &b64ctx{mode: encModeBase64}
	data := ctx.Convert(e.Content)
	return json.Marshal(data)
}

// ExpectedBase16 is data expected to be encoded as base16-encoding (as known as hex-encoding).
// CBOR tags that has tag number 23 is converted to this type.
// When encoded to JSON, []byte in Content is converted to a base16-encoded string.
// See RFC 8949 Section 3.4.5.2.
type ExpectedBase16 struct {
	Content any
}

func (e ExpectedBase16) MarshalJSON() ([]byte, error) {
	ctx := &b64ctx{mode: encModeBase16}
	data := ctx.Convert(e.Content)
	return json.Marshal(data)
}

// Base64String is a base64 with padding encoded data.
// CBOR tags that has tag number 34 is converted to this type.
// The decoder and the encoder validate that it is a valid base64-encoded string.
// See RFC 8949 Section 3.4.5.3.
type Base64String string

// Base64URLString is a base64url with no padding encoded string.
// CBOR tags that has tag number 33 is converted to this type.
// The decoder and the encoder validate that it is a valid base64url-encoded string.
// See RFC 8949 Section 3.4.5.3.
type Base64URLString string
