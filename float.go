package cbor

//go:generate sh -c "perl scripts/float_gen.pl | gofmt > float_gen_test.go"

func EncodeFloat64(f float64) []byte {
	return []byte{}
}
