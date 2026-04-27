package codec

import (
	"io"
	"strings"
)

// Encoder encodes values onto a stream.
type Encoder interface {
	Encode(any) error
}

// Decoder decodes values from a stream.
type Decoder interface {
	Decode(any) error
}

// Codec creates stream encoders and decoders and provides whole-value helpers.
type Codec interface {
	// NewEncoder returns an encoder that writes to w.
	NewEncoder(io.Writer) Encoder
	// NewDecoder returns a decoder that reads from r.
	NewDecoder(io.Reader) Decoder
	// Marshal encodes a value into a byte slice.
	Marshal(any) ([]byte, error)
	// Unmarshal decodes data into a value.
	Unmarshal([]byte, any) error
	// Raw returns the codec's raw message type, interchangeable with []byte
	Raw() any
	// UnwrapRaw extracts the raw byte representation from the codec's raw message type
	UnwrapRaw(any) []byte
}

func FindByName(name string) Codec {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "json":
		return JSON
	case "cbor":
		return CBOR
	default:
		return nil
	}
}
