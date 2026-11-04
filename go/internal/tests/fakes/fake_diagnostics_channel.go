package fakes

import (
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
)

// FakeDiagnosticsChannel is a fake of outbound.DiagnosticsChannelPort that records every diagnostic it is given, with no
// rate limit and no dispatcher, and counts the calls of Start and Stop.
type FakeDiagnosticsChannel struct {
	mutex  sync.Mutex
	events []dtos.DiagnosticEvent
	starts int
	stops  int
}

// Publish records the diagnostic.
func (fake_channel *FakeDiagnosticsChannel) Publish(event dtos.DiagnosticEvent) {
	fake_channel.mutex.Lock()
	defer fake_channel.mutex.Unlock()
	fake_channel.events = append(fake_channel.events, event)
}

// Start counts the call.
func (fake_channel *FakeDiagnosticsChannel) Start() {
	fake_channel.mutex.Lock()
	defer fake_channel.mutex.Unlock()
	fake_channel.starts++
}

// Stop counts the call.
func (fake_channel *FakeDiagnosticsChannel) Stop() {
	fake_channel.mutex.Lock()
	defer fake_channel.mutex.Unlock()
	fake_channel.stops++
}

// Counters returns the published diagnostics as Published.
func (fake_channel *FakeDiagnosticsChannel) Counters() dtos.ChannelDiagnostics {
	fake_channel.mutex.Lock()
	defer fake_channel.mutex.Unlock()
	return dtos.ChannelDiagnostics{Published: len(fake_channel.events)}
}

// Events returns a copy of the diagnostics recorded, in order.
func (fake_channel *FakeDiagnosticsChannel) Events() []dtos.DiagnosticEvent {
	fake_channel.mutex.Lock()
	defer fake_channel.mutex.Unlock()
	return append([]dtos.DiagnosticEvent(nil), fake_channel.events...)
}

// Calls returns how many times Start and Stop were called.
func (fake_channel *FakeDiagnosticsChannel) Calls() (starts int, stops int) {
	fake_channel.mutex.Lock()
	defer fake_channel.mutex.Unlock()
	return fake_channel.starts, fake_channel.stops
}
