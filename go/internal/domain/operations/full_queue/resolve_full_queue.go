// Package full_queue holds the pure rule of a full queue: what happens to a new event when there is no room (REQ-DLV-007,
// REQ-DLV-009).
package full_queue

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// RESOLVE FULL QUEUE
// =============================================================================

// Outcome is what a full queue does with a new event.
type Outcome string

// The outcomes of a Resolution.
const (
	// OutcomeDropNew means the new event is dropped and the queue does not change.
	OutcomeDropNew Outcome = "drop_new"
	// OutcomeDisplace means the queued event at Index is removed and the new event takes the last place.
	OutcomeDisplace Outcome = "displace"
	// OutcomeWait means nothing changes now: the caller may wait for room and offer again.
	OutcomeWait Outcome = "wait"
)

// The reasons of a drop or a displacement, the names of the counters per reason (REQ-DLV-008).
const (
	// ReasonQueueFullNewest is the reason of a new event that was dropped because the queue was full.
	ReasonQueueFullNewest = "queue_full_newest"
	// ReasonQueueFullOldest is the reason of a queued event that was removed to make room for a new one (drop_oldest).
	ReasonQueueFullOldest = "queue_full_oldest"
	// ReasonDisplacedByCritical is the reason of a queued event that was removed to make room for a critical one.
	ReasonDisplacedByCritical = "displaced_by_critical"
	// ReasonBlockTimeout is the reason of a new event that found no room within the timeout of block_with_timeout.
	ReasonBlockTimeout = "block_timeout"
)

// Resolution is the answer of ResolveFullQueue. Index is the position (oldest is zero) of the queued event to remove, and
// is set only for OutcomeDisplace; otherwise it is -1. Reason is the reason of the drop or the displacement; for
// OutcomeWait it is ReasonBlockTimeout, the reason the caller uses when no room appears in time.
type Resolution struct {
	Outcome Outcome
	Index   int
	Reason  string
}

// ResolveFullQueue decides what a FULL queue does with a new event. queued_classes holds the class of every queued event,
// oldest first. The class order is bulk < normal < critical. The rule:
//
//  1. A new critical event displaces the OLDEST queued event of class bulk or, when there is none, the OLDEST one of
//     class normal (reason displaced_by_critical).
//  2. When that displaces nothing, the policy decides. drop_newest drops the new event (queue_full_newest). drop_oldest
//     displaces the oldest queued event whose class is not higher than the class of the new one (queue_full_oldest) and,
//     when there is none, drops the new event (queue_full_newest). block_with_timeout answers wait.
//
// An unknown policy is treated as drop_newest. It is pure and it does not change queued_classes.
func ResolveFullQueue(policy value_objects.QueuePolicy, new_class value_objects.EventClass, queued_classes []value_objects.EventClass) Resolution {
	if new_class == value_objects.EventClassCritical {
		if index := oldest_index_of(queued_classes, func(queued_class value_objects.EventClass) bool {
			return queued_class == value_objects.EventClassBulk
		}); index >= 0 {
			return Resolution{Outcome: OutcomeDisplace, Index: index, Reason: ReasonDisplacedByCritical}
		}
		if index := oldest_index_of(queued_classes, func(queued_class value_objects.EventClass) bool {
			return queued_class == value_objects.EventClassNormal
		}); index >= 0 {
			return Resolution{Outcome: OutcomeDisplace, Index: index, Reason: ReasonDisplacedByCritical}
		}
	}

	switch policy {
	case value_objects.QueuePolicyDropOldest:
		new_rank := class_rank(new_class)
		if index := oldest_index_of(queued_classes, func(queued_class value_objects.EventClass) bool {
			return class_rank(queued_class) <= new_rank
		}); index >= 0 {
			return Resolution{Outcome: OutcomeDisplace, Index: index, Reason: ReasonQueueFullOldest}
		}
	case value_objects.QueuePolicyBlockWithTimeout:
		return Resolution{Outcome: OutcomeWait, Index: -1, Reason: ReasonBlockTimeout}
	}
	return Resolution{Outcome: OutcomeDropNew, Index: -1, Reason: ReasonQueueFullNewest}
}

// ----- HELPERS --------------------------------------------------------------

// class_rank orders the classes: bulk is 0, normal is 1, critical is 2. A class that is not known ranks as normal.
func class_rank(event_class value_objects.EventClass) int {
	switch event_class {
	case value_objects.EventClassBulk:
		return 0
	case value_objects.EventClassCritical:
		return 2
	default:
		return 1
	}
}

// oldest_index_of returns the position of the first class that matches, or -1.
func oldest_index_of(queued_classes []value_objects.EventClass, matches func(queued_class value_objects.EventClass) bool) int {
	for index, queued_class := range queued_classes {
		if matches(queued_class) {
			return index
		}
	}
	return -1
}
