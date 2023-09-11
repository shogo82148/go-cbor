package cbor

type undefined *struct{}

var Undefined undefined = nil

type CBORMarshaler interface {
	MarshalCBOR() ([]byte, error)
}
