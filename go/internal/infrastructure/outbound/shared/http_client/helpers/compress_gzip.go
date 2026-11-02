package helpers

import (
	"bytes"
	"compress/gzip"
)

// =============================================================================
// COMPRESS GZIP
// =============================================================================

// CompressGzip returns the body compressed with gzip at the default level. The header is deterministic: no
// modification time and no name, so the same body gives the same bytes.
func CompressGzip(body []byte) ([]byte, error) {
	var buffer bytes.Buffer
	gzip_writer := gzip.NewWriter(&buffer)
	if _, write_error := gzip_writer.Write(body); write_error != nil {
		return nil, write_error
	}
	if close_error := gzip_writer.Close(); close_error != nil {
		return nil, close_error
	}
	return buffer.Bytes(), nil
}
