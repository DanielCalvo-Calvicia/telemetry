// Package memory_queue is the adapter of the event queue port that keeps the events in memory, bounded and in order.
package memory_queue

import (
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/full_queue"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/memory_queue/helpers"
)

// =============================================================================
// MEMORY QUEUE
// =============================================================================

// MemoryQueue implements outbound.EventQueuePort with a ring buffer guarded by a mutex. It does no I/O and holds no
// global state. No call waits for anything but the mutex, which is held only for a copy of a few entries and, when the queue is full, for
// the scan of the classes.
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

// Offer puts the envelope in the queue when there is room. When the queue is full it asks full_queue.ResolveFullQueue
// with the classes of the queued events, UNDER THE SAME LOCK, so the decision and the change are one atomic step: drop
// the new event, or remove the queued event the rule chose (the order of the rest is kept) and append the new one, or
// answer MustWait and change nothing.
func (memory_queue *MemoryQueue) Offer(
	envelope value_objects.Envelope,
	event_class value_objects.EventClass,
	arrival_ms int64,
	policy value_objects.QueuePolicy,
) outbound.OfferResult {
	memory_queue.mutex.Lock()
	defer memory_queue.mutex.Unlock()
	entry := helpers.Entry{Envelope: envelope, Class: event_class, ArrivalMS: arrival_ms}
	if memory_queue.ring.Push(entry) {
		return outbound.OfferResult{Admitted: true}
	}

	resolution := full_queue.ResolveFullQueue(policy, event_class, memory_queue.ring.Classes())
	switch resolution.Outcome {
	case full_queue.OutcomeDisplace:
		// The index is valid by construction (it comes from the classes of this same queue, under this lock) and the
		// removal frees one slot, so the push that follows cannot fail.
		removed, _ := memory_queue.ring.RemoveAt(resolution.Index)
		memory_queue.ring.Push(entry)
		displaced := removed.Envelope
		return outbound.OfferResult{Admitted: true, Displaced: &displaced, Reason: resolution.Reason}
	case full_queue.OutcomeWait:
		return outbound.OfferResult{MustWait: true, Reason: resolution.Reason}
	default:
		return outbound.OfferResult{Reason: resolution.Reason}
	}
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
