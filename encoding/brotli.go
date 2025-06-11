package encoding

import (
	"bytes"
	"io"

	"github.com/andybalholm/brotli"
)

// BrotliEncoderDecoder implements the EncoderAndDecoder interface using Brotli.
type BrotliEncoderDecoder struct{}

// Encode compresses the input data using Brotli.
func (b BrotliEncoderDecoder) Encode(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	bw := brotli.NewWriter(&buf)
	_, err := bw.Write(data)
	if err != nil {
		return nil, err
	}
	if err := bw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode decompresses the input data using Brotli.
func (b BrotliEncoderDecoder) Decode(data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(data)
	br := brotli.NewReader(buf)
	decoded, err := io.ReadAll(br)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}
