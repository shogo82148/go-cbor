package cbor

import "slices"

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
