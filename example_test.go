package cbor_test

import (
	"encoding/json"
	"fmt"

	"github.com/shogo82148/go-cbor"
)

func ExampleExpectedBase64URL() {
	data, _ := json.Marshal(cbor.ExpectedBase64URL{
		Content: []byte("🍣"),
	})
	fmt.Println(string(data))

	// Output:
	// "8J-Now"
}

func ExampleExpectedBase64() {
	data, _ := json.Marshal(cbor.ExpectedBase64{
		Content: []byte("🍣"),
	})
	fmt.Println(string(data))

	// Output:
	// "8J+Now=="
}

func ExampleExpectedBase16() {
	data, _ := json.Marshal(cbor.ExpectedBase16{
		Content: []byte("🍣"),
	})
	fmt.Println(string(data))

	// Output:
	// "f09f8da3"
}
