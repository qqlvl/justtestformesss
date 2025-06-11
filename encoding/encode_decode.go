package encoding

import (
	"fmt"
	"strings"
)

func Encode(data []byte, encoding string) ([]byte, error) {
	var encoder EncoderAndDecoder
	switch encoding {
	case "gzip":
		encoder = GzipEncoderDecoder{}
	case "brotli", "br":
		encoder = BrotliEncoderDecoder{}
	case "deflate":
		encoder = DeflateEncoderDecoder{}
	case "zstd":
		encoder = ZstdEncoderDecoder{}
	case "plain", "":
		return data, nil
	default:
		return nil, fmt.Errorf("unknown encoding: %s", encoding)
	}
	return encoder.Encode(data)
}

func Decode(data []byte, encoding string) ([]byte, error) {
	var decoder EncoderAndDecoder
	switch encoding {
	case "gzip":
		decoder = GzipEncoderDecoder{}
	case "brotli", "br":
		decoder = BrotliEncoderDecoder{}
	case "deflate":
		decoder = DeflateEncoderDecoder{}
	case "zstd":
		decoder = ZstdEncoderDecoder{}
	case "plain", "":
		return data, nil
	default:
		return nil, fmt.Errorf("unknown encoding: %s", encoding)
	}
	return decoder.Decode(data)
}

func EncodeWithSomething(data []byte, acceptEncoding string) ([]byte, string, error) {
	acceptEncodingChunks := strings.Split(acceptEncoding, ",")
	for _, encoding := range acceptEncodingChunks {
		encoding = strings.TrimSpace(encoding)
		if encoding == "" {
			continue
		}
		encoded, err := Encode(data, encoding)
		if err != nil {
			return nil, "", err
		}
		if len(encoded) < len(data) {
			return encoded, encoding, nil
		}
	}

	return data, "", nil
}
