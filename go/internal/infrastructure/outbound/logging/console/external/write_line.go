package external

import (
	"errors"
	"io"
)

// =============================================================================
// WRITE LINE
// =============================================================================

// ErrShortWrite is the cause of a writer that took fewer bytes than it was given without saying why.
var ErrShortWrite = errors.New("the writer took fewer bytes than the line has")

// WriteLine writes the whole line with one call to the writer, so the line of a caller is never cut by the line of
// another one. A short write is an error.
func WriteLine(writer io.Writer, line []byte) error {
	written, write_error := writer.Write(line)
	if write_error != nil {
		return write_error
	}
	if written < len(line) {
		return ErrShortWrite
	}
	return nil
}
