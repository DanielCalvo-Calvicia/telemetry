// Package external holds the only I/O of the console destination: the standard output and the write of one line.
package external

import (
	"io"
	"os"
)

// =============================================================================
// STANDARD OUTPUT
// =============================================================================

// StandardOutput returns the standard output of the process.
func StandardOutput() io.Writer {
	return os.Stdout
}
