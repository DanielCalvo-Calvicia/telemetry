package outbound

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// EVENT QUEUE PORT
// =============================================================================

// EventQueuePort defines the bounded queue of the events that wait for one destination (REQ-DLV-006). It keeps the order
// of arrival and is safe for concurrent use. It never blocks: a full queue refuses the push.
type EventQueuePort interface {
	// Push appends the envelope with the arrival time in milliseconds of the monotonic clock. It returns false, and
	// stores nothing, when the queue is full.
	Push(envelope value_objects.Envelope, arrival_ms int64) bool

	// Take removes and returns the oldest n events, in order. It returns fewer when the queue holds fewer, and none when
	// n is not positive.
	Take(n int) []value_objects.Envelope

	// Len returns the number of events held.
	Len() int

	// OldestArrivalMS returns the arrival time of the oldest event held, and false when the queue is empty.
	OldestArrivalMS() (int64, bool)
}
