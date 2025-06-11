package encoding

import (
	"bytes"
	"compress/flate"
	"io"
)

// DeflateEncoderDecoder implements the EncoderAndDecoder interface using deflate.
type DeflateEncoderDecoder struct{}

// Encode compresses the input data using deflate.
func (d DeflateEncoderDecoder) Encode(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	fw, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	_, err = fw.Write(data)
	if err != nil {
		return nil, err
	}
	if err := fw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode decompresses the input data using deflate.
func (d DeflateEncoderDecoder) Decode(data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(data)
	fr := flate.NewReader(buf)
	defer fr.Close()
	decoded, err := io.ReadAll(fr)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}
