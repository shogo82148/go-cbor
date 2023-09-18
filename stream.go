package cbor

import (
	"io"
	"slices"
)

// A Decoder reads and decodes CBOR values from an input stream.
type Decoder struct {
	r     io.Reader
	err   error
	scanp int // start of unread data in buf
	buf   []byte
	d     decodeState
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// Decode reads the next CBOR-encoded value from its input and stores it in the
// value pointed to by v.
func (dec *Decoder) Decode(v any) error {
	if dec.err != nil {
		return dec.err
	}

	n, err := dec.readValue()
	if err != nil {
		return err
	}
	dec.d.init(dec.buf[:n])
	dec.scanp += n

	return dec.d.decode(v)
}

func (dec *Decoder) readValue() (n int, err error) {
	for {
		dec.d.init(dec.buf[dec.scanp:])
		err := dec.d.checkWellFormedChild()
		if err == nil {
			return dec.d.off, nil
		}

		// More data is needed and there was no read error.
		if err := dec.refill(); err != nil {
			return 0, err
		}
	}
}

func (dec *Decoder) refill() error {
	// Make room to read more into the buffer.
	// First slide down data already consumed.
	if dec.scanp > 0 {
		n := copy(dec.buf, dec.buf[dec.scanp:])
		dec.buf = dec.buf[:n]
		dec.scanp = 0
	}

	// Grow buffer if not large enough.
	const minRead = 512
	dec.buf = slices.Grow(dec.buf, minRead)

	// Read. Delay error for next iteration (after scan).
	n, err := dec.r.Read(dec.buf[len(dec.buf):cap(dec.buf)])
	dec.buf = dec.buf[:len(dec.buf)+n]
	return err
}

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
