package external

import (
	"io"
	"os"
)

// StandardOutput returns the process standard output.
func StandardOutput() io.Writer {
	return os.Stdout
}
