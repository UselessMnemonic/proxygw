package codec

import (
	"encoding/json"
	"io"
)

type jsonCodec struct{}

// JSON is the package Codec implementation backed by encoding/json.
var JSON Codec = jsonCodec{}

func (jsonCodec) NewEncoder(w io.Writer) Encoder {
	return json.NewEncoder(w)
}

func (jsonCodec) NewDecoder(r io.Reader) Decoder {
	return json.NewDecoder(r)
}

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonCodec) Raw() any {
	return &json.RawMessage{}
}

func (jsonCodec) UnwrapRaw(v any) []byte {
	return *v.(*json.RawMessage)
}
