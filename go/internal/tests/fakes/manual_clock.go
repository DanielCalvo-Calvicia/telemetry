package fakes

import (
	"context"
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// ManualClock is a fake of outbound.ClockPort whose monotonic reading changes only when a test advances it, so a test
// controls the milliseconds of a rate limit exactly. The wall time is the zero instant.
type ManualClock struct {
	mutex       sync.Mutex
	nanoseconds int64
}

// NewManualClock returns a ManualClock that reads zero.
func NewManualClock() *ManualClock {
	return &ManualClock{}
}

// AdvanceMilliseconds moves the monotonic reading forward.
func (manual_clock *ManualClock) AdvanceMilliseconds(milliseconds int64) {
	manual_clock.mutex.Lock()
	defer manual_clock.mutex.Unlock()
	manual_clock.nanoseconds += milliseconds * 1_000_000
}

// Now returns the zero instant.
func (manual_clock *ManualClock) Now(_ context.Context) value_objects.UTCInstant {
	return value_objects.UTCInstant{}
}

// MonotonicNanoseconds returns the reading that the test set.
func (manual_clock *ManualClock) MonotonicNanoseconds(_ context.Context) int64 {
	manual_clock.mutex.Lock()
	defer manual_clock.mutex.Unlock()
	return manual_clock.nanoseconds
}

// ClockInfo returns an empty ClockInfo.
func (manual_clock *ManualClock) ClockInfo(_ context.Context) dtos.ClockInfo {
	return dtos.ClockInfo{}
}
