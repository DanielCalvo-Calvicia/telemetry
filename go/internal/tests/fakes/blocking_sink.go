package fakes

import (
	"context"
	"slices"
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// BlockingLogSink is a fake of outbound.LogSinkPort whose Send never answers by itself: it waits until Release is
// called or the context of the call is done (a worker that is stopped, or a timeout). It records every batch it receives
// before it blocks, and tells when the first call has started.
type BlockingLogSink struct {
	mutex      sync.Mutex
	batches    [][]value_objects.Envelope
	started    chan struct{}
	release    chan struct{}
	start_once sync.Once
	stop_once  sync.Once
}

// NewBlockingLogSink returns a BlockingLogSink that blocks every call.
func NewBlockingLogSink() *BlockingLogSink {
	return &BlockingLogSink{started: make(chan struct{}), release: make(chan struct{})}
}

// Send records the events and blocks until Release or the end of the context, then accepts the events.
func (sink *BlockingLogSink) Send(ctx context.Context, events []value_objects.Envelope) dtos.SendResult {
	sink.mutex.Lock()
	sink.batches = append(sink.batches, slices.Clone(events))
	sink.mutex.Unlock()
	sink.start_once.Do(func() { close(sink.started) })
	select {
	case <-sink.release:
	case <-ctx.Done():
	}
	return dtos.SendResult{Accepted: len(events)}
}

// Started is closed when the first call to Send has started.
func (sink *BlockingLogSink) Started() <-chan struct{} { return sink.started }

// Release lets every blocked and later call return. It is idempotent.
func (sink *BlockingLogSink) Release() {
	sink.stop_once.Do(func() { close(sink.release) })
}

// Calls returns the number of times Send was called.
func (sink *BlockingLogSink) Calls() int {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	return len(sink.batches)
}
