// Package fakes holds in-memory fakes of the outbound ports, so that services can be tested without an adapter.
// They record what they receive, return what a test sets and guard their state with a mutex.
package fakes

import (
	"context"
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// FakeClock is a fake of outbound.ClockPort. The wall time is the one that was set; the monotonic reading advances
// by a settable step on every read, so it never goes back (unless the step is set negative, which a test must not do).
type FakeClock struct {
	mutex            sync.Mutex
	wall_time        value_objects.UTCInstant
	monotonic_nanos  int64
	monotonic_step   int64
	clock_info       dtos.ClockInfo
	now_calls        int
	monotonic_calls  int
	clock_info_calls int
}

// NewFakeClock returns a FakeClock that reports the wall time and whose monotonic reading advances one microsecond
// per read.
func NewFakeClock(wall_time value_objects.UTCInstant) *FakeClock {
	return &FakeClock{wall_time: wall_time, monotonic_step: 1000}
}

// Set sets the wall time that Now returns.
func (fake_clock *FakeClock) Set(wall_time value_objects.UTCInstant) {
	fake_clock.mutex.Lock()
	defer fake_clock.mutex.Unlock()
	fake_clock.wall_time = wall_time
}

// SetStep sets how many nanoseconds the monotonic reading advances on every read.
func (fake_clock *FakeClock) SetStep(step_nanoseconds int64) {
	fake_clock.mutex.Lock()
	defer fake_clock.mutex.Unlock()
	fake_clock.monotonic_step = step_nanoseconds
}

// SetClockInfo sets what ClockInfo returns.
func (fake_clock *FakeClock) SetClockInfo(clock_info dtos.ClockInfo) {
	fake_clock.mutex.Lock()
	defer fake_clock.mutex.Unlock()
	fake_clock.clock_info = clock_info
}

// Calls returns the number of calls received by Now, MonotonicNanoseconds and ClockInfo, in that order.
func (fake_clock *FakeClock) Calls() (now_calls int, monotonic_calls int, clock_info_calls int) {
	fake_clock.mutex.Lock()
	defer fake_clock.mutex.Unlock()
	return fake_clock.now_calls, fake_clock.monotonic_calls, fake_clock.clock_info_calls
}

// Now returns the wall time that was set.
func (fake_clock *FakeClock) Now(_ context.Context) value_objects.UTCInstant {
	fake_clock.mutex.Lock()
	defer fake_clock.mutex.Unlock()
	fake_clock.now_calls++
	return fake_clock.wall_time
}

// MonotonicNanoseconds advances the reading by the step and returns it.
func (fake_clock *FakeClock) MonotonicNanoseconds(_ context.Context) int64 {
	fake_clock.mutex.Lock()
	defer fake_clock.mutex.Unlock()
	fake_clock.monotonic_calls++
	fake_clock.monotonic_nanos += fake_clock.monotonic_step
	return fake_clock.monotonic_nanos
}

// ClockInfo returns the clock information that was set.
func (fake_clock *FakeClock) ClockInfo(_ context.Context) dtos.ClockInfo {
	fake_clock.mutex.Lock()
	defer fake_clock.mutex.Unlock()
	fake_clock.clock_info_calls++
	return fake_clock.clock_info
}
