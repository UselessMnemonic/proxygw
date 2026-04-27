package codec

import (
	"io"

	"github.com/fxamacker/cbor/v2"
)

type cborCodec struct{}

// CBOR is the package Codec implementation backed by fxamacker/cbor.
var CBOR Codec = cborCodec{}

func (cborCodec) NewEncoder(w io.Writer) Encoder {
	return cbor.NewEncoder(w)
}

func (cborCodec) NewDecoder(r io.Reader) Decoder {
	return cbor.NewDecoder(r)
}

func (cborCodec) Marshal(v any) ([]byte, error) {
	return cbor.Marshal(v)
}

func (cborCodec) Unmarshal(data []byte, v any) error {
	return cbor.Unmarshal(data, v)
}

func (cborCodec) Raw() any {
	return &cbor.RawMessage{}
}

func (cborCodec) UnwrapRaw(v any) []byte {
	return *v.(*cbor.RawMessage)
}
