package fakes

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// ScriptedLogSink is a fake of outbound.LogSinkPort that answers a script: the answer of call number n is the n-th result
// that was given, and the last one repeats when the script is used up. It records the events and the moment of every call
// (the real time), so a test can read how many calls were made, how big they were and how much time passed between the
// first and the last. A call whose number is in the panics set panics instead of answering.
type ScriptedLogSink struct {
	mutex   sync.Mutex
	answers []dtos.SendResult
	panics  map[int]bool
	batches [][]value_objects.Envelope
	moments []time.Time
}

// NewScriptedLogSink returns a ScriptedLogSink that answers the results in order. With no result it accepts every event.
func NewScriptedLogSink(answers ...dtos.SendResult) *ScriptedLogSink {
	return &ScriptedLogSink{answers: answers, panics: map[int]bool{}}
}

// PanicOnCalls makes the calls with these numbers (from zero) panic.
func (sink *ScriptedLogSink) PanicOnCalls(call_numbers ...int) {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	for _, call_number := range call_numbers {
		sink.panics[call_number] = true
	}
}

// Send records the call and answers the script.
func (sink *ScriptedLogSink) Send(_ context.Context, events []value_objects.Envelope) dtos.SendResult {
	sink.mutex.Lock()
	call_number := len(sink.batches)
	sink.batches = append(sink.batches, slices.Clone(events))
	sink.moments = append(sink.moments, time.Now())
	should_panic := sink.panics[call_number]
	var result dtos.SendResult
	switch {
	case len(sink.answers) == 0:
		result = dtos.SendResult{Accepted: len(events)}
	default:
		result = sink.answers[min(call_number, len(sink.answers)-1)]
	}
	sink.mutex.Unlock()
	if should_panic {
		panic("scripted panic")
	}
	return result
}

// Calls returns the number of calls received.
func (sink *ScriptedLogSink) Calls() int {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	return len(sink.batches)
}

// Batches returns a copy of the events of every call, in order.
func (sink *ScriptedLogSink) Batches() [][]value_objects.Envelope {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	copied := make([][]value_objects.Envelope, 0, len(sink.batches))
	for _, batch := range sink.batches {
		copied = append(copied, slices.Clone(batch))
	}
	return copied
}

// Elapsed returns the time between the first call and the last one (zero with fewer than two calls).
func (sink *ScriptedLogSink) Elapsed() time.Duration {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	if len(sink.moments) < 2 {
		return 0
	}
	return sink.moments[len(sink.moments)-1].Sub(sink.moments[0])
}
