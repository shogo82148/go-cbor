package cbor

import (
	"bytes"
	"math"
	"testing"
)

func TestFloat(t *testing.T) {
	for _, tt := range f64ToBytesTests {
		input := math.Float64frombits(tt.f64)
		got := EncodeFloat64(input)
		if !bytes.Equal(got, tt.bytes) {
			t.Errorf("EncodeFloat64(%x) = %v, want %v", input, got, tt.bytes)
		}
	}
}
