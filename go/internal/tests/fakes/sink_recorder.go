package fakes

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// sink_recorder is the behavior that the fake sinks share: it records every batch as a copy, waits for the delay and
// returns the result that was set (by default, every event accepted). The zero value is ready to use.
type sink_recorder struct {
	mutex      sync.Mutex
	batches    [][]value_objects.Envelope
	result     dtos.SendResult
	result_set bool
	delay      time.Duration
}

// Send records a copy of the events, waits for the delay and returns the result.
func (recorder *sink_recorder) Send(_ context.Context, events []value_objects.Envelope) dtos.SendResult {
	recorder.mutex.Lock()
	recorder.batches = append(recorder.batches, slices.Clone(events))
	delay := recorder.delay
	result := recorder.result
	if !recorder.result_set {
		result = dtos.SendResult{Accepted: len(events)}
	}
	recorder.mutex.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	return result
}

// SetResult makes every later Send return the result, whatever the events are.
func (recorder *sink_recorder) SetResult(result dtos.SendResult) {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	recorder.result = result
	recorder.result_set = true
}

// SetDelay makes every later Send wait for the delay before it returns.
func (recorder *sink_recorder) SetDelay(delay time.Duration) {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	recorder.delay = delay
}

// Batches returns a copy of the batches received, in order.
func (recorder *sink_recorder) Batches() [][]value_objects.Envelope {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	copied_batches := make([][]value_objects.Envelope, 0, len(recorder.batches))
	for _, batch := range recorder.batches {
		copied_batches = append(copied_batches, slices.Clone(batch))
	}
	return copied_batches
}

// Calls returns the number of times Send was called.
func (recorder *sink_recorder) Calls() int {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	return len(recorder.batches)
}
