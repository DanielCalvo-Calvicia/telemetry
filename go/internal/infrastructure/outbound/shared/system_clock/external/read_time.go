// Package external holds the calls of the system clock to the machine; they are the only ones that read the time.
package external

import "time"

// ReadTime returns the current wall time of the machine.
func ReadTime() time.Time {
	return time.Now()
}

// NewMonotonicReader returns a function that reads the nanoseconds elapsed since the call of NewMonotonicReader, from
// the monotonic reading of the machine, so it never goes back when the wall clock is adjusted.
func NewMonotonicReader() func() int64 {
	started_at := time.Now()
	return func() int64 {
		return time.Since(started_at).Nanoseconds()
	}
}
