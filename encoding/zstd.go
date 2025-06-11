package encoding

import (
	"github.com/klauspost/compress/zstd"
)

// ZstdEncoderDecoder implements the EncoderAndDecoder interface using Zstandard.
type ZstdEncoderDecoder struct{}

// Encode compresses the input data using Zstandard.
func (z ZstdEncoderDecoder) Encode(data []byte) ([]byte, error) {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, err
	}
	defer encoder.Close()

	encoded := encoder.EncodeAll(data, make([]byte, 0, len(data)))
	return encoded, nil
}

// Decode decompresses the input data using Zstandard.
func (z ZstdEncoderDecoder) Decode(data []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()

	decoded, err := decoder.DecodeAll(data, nil)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}
