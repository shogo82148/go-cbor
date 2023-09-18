package cbor

import "io"

// An Encoder writes CBOR to an output stream.
type Encoder struct {
	w   io.Writer
	err error
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode writes the CBOR encoding of v to the stream.
func (enc *Encoder) Encode(v any) error {
	if enc.err != nil {
		return enc.err
	}

	data, err := Marshal(v)
	if err != nil {
		enc.err = err
		return err
	}

	_, err = enc.w.Write(data)
	return err
}
