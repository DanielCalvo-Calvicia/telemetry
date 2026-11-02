// Package external holds the calls of the UUID generator to the machine: the clock and the random source.
package external

import (
	"crypto/rand"
	"time"
)

// ReadMilliseconds returns the unix time of the machine in milliseconds.
func ReadMilliseconds() int64 {
	return time.Now().UnixMilli()
}

// ReadRandom fills the buffer with bytes of the cryptographic random source of the machine. There is no other source
// to fall back to.
func ReadRandom(buffer []byte) error {
	_, read_error := rand.Read(buffer)
	return read_error
}
