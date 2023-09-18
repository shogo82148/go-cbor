package cbor

type FooA struct {
	A int
	B string
}

type FooB struct {
	Alg int    `cbor:"1,keyasint,omitempty"`
	Kit []byte `cbor:"4,keyasint,omitempty"`
}

type FooC struct {
	_ struct{} `cbor:",toarray"`
	A int
	B string
}
