package cbor

// xorshift64 is a pseudo random number generator.
// https://en.wikipedia.org/wiki/Xorshift
type xorshift64 uint64

func newXorshift64() *xorshift64 {
	x := xorshift64(42)
	return &x
}

func (x *xorshift64) Uint64() uint64 {
	a := *x
	a ^= a << 13
	a ^= a >> 7
	a ^= a << 17
	*x = a
	return uint64(a)
}
