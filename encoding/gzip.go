package encoding

import (
	"bytes"
	"compress/gzip"
	"io"
)

// GzipEncoderDecoder implements the EncoderAndDecoder interface using gzip.
type GzipEncoderDecoder struct{}

// Encode compresses the input data using gzip.
func (g GzipEncoderDecoder) Encode(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write(data)
	if err != nil {
		return nil, err
	}
	// It's important to close the writer to flush the data
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode decompresses the input data using gzip.
func (g GzipEncoderDecoder) Decode(data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(data)
	gr, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	decoded, err := io.ReadAll(gr)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}
