package outbound

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// EVENT QUEUE PORT
// =============================================================================

// OfferResult is the answer of EventQueuePort.Offer. Admitted says the offered event is now in the queue. Displaced is
// the queued event that was removed to make room for it (nil when none) and Reason is the reason of that displacement, or
// the reason of the drop when the event was not admitted. MustWait says the queue is full and the policy is
// block_with_timeout: nothing changed and the caller may wait for room and offer again (Reason is then the reason to use
// if no room appears in time).
type OfferResult struct {
	Admitted  bool
	Displaced *value_objects.Envelope
	Reason    string
	MustWait  bool
}

// EventQueuePort defines the bounded queue of the events that wait for one destination (REQ-DLV-006). It keeps the order
// of arrival and is safe for concurrent use. It never blocks: a full queue answers at once what to do with the new event.
type EventQueuePort interface {
	// Offer puts the envelope, of the class event_class and with the arrival time in milliseconds of the monotonic clock,
	// in the queue. When the queue has room the event is admitted. When it is full the queue decides, in one atomic step,
	// with the rule of the full queue (domain/operations/full_queue) and the policy: the new event is dropped, or a
	// queued event is removed (the order of the rest is kept) and the new one is appended, or, with block_with_timeout,
	// nothing changes and the result says MustWait.
	Offer(envelope value_objects.Envelope, event_class value_objects.EventClass, arrival_ms int64, policy value_objects.QueuePolicy) OfferResult

	// Take removes and returns the oldest n events, in order. It returns fewer when the queue holds fewer, and none when
	// n is not positive.
	Take(n int) []value_objects.Envelope

	// Len returns the number of events held.
	Len() int

	// OldestArrivalMS returns the arrival time of the oldest event held, and false when the queue is empty.
	OldestArrivalMS() (int64, bool)
}
