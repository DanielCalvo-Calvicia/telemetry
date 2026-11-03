// Package memory_queue is the adapter of the event queue port that keeps the events in memory, bounded and in order.
package memory_queue

import (
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/memory_queue/helpers"
)

// =============================================================================
// MEMORY QUEUE
// =============================================================================

// MemoryQueue implements outbound.EventQueuePort with a ring buffer guarded by a mutex. It does no I/O and holds no
// global state. No call waits for anything but the mutex, which is held only for a copy of a few entries.
type MemoryQueue struct {
	mutex sync.Mutex
	ring  *helpers.RingBuffer
}

var _ outbound.EventQueuePort = (*MemoryQueue)(nil)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewMemoryQueue returns an empty MemoryQueue that holds up to capacity events. A capacity below 1 is 1.
func NewMemoryQueue(capacity int) *MemoryQueue {
	return &MemoryQueue{ring: helpers.NewRingBuffer(capacity)}
}

// ----- METHODS --------------------------------------------------------------

// Push appends the envelope, or returns false when the queue is full.
func (memory_queue *MemoryQueue) Push(envelope value_objects.Envelope, arrival_ms int64) bool {
	memory_queue.mutex.Lock()
	defer memory_queue.mutex.Unlock()
	return memory_queue.ring.Push(helpers.Entry{Envelope: envelope, ArrivalMS: arrival_ms})
}

// Take removes and returns the oldest n events, in order.
func (memory_queue *MemoryQueue) Take(n int) []value_objects.Envelope {
	memory_queue.mutex.Lock()
	entries := memory_queue.ring.Take(n)
	memory_queue.mutex.Unlock()
	envelopes := make([]value_objects.Envelope, len(entries))
	for index, entry := range entries {
		envelopes[index] = entry.Envelope
	}
	return envelopes
}

// Len returns the number of events held.
func (memory_queue *MemoryQueue) Len() int {
	memory_queue.mutex.Lock()
	defer memory_queue.mutex.Unlock()
	return memory_queue.ring.Len()
}

// OldestArrivalMS returns the arrival time of the oldest event, and false when the queue is empty.
func (memory_queue *MemoryQueue) OldestArrivalMS() (int64, bool) {
	memory_queue.mutex.Lock()
	defer memory_queue.mutex.Unlock()
	return memory_queue.ring.Oldest()
}
