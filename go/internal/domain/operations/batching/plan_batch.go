// Package batching holds the pure rule of the batcher: when a batch leaves (REQ-DLV-006).
package batching

// =============================================================================
// PLAN BATCH
// =============================================================================

// Action is what a worker does next.
type Action int

// The actions of a Plan.
const (
	// PlanIdle means there is nothing queued: the worker waits for a push or for a stop.
	PlanIdle Action = iota
	// PlanSend means a batch leaves now: the worker takes Take events and sends them.
	PlanSend
	// PlanWait means events are queued but no batch is due: the worker waits WaitMS milliseconds, or for a push or a stop.
	PlanWait
)

// Plan is the answer of PlanBatch. Take is the number of events to take and is set only for PlanSend. WaitMS is the
// milliseconds until the oldest event is flush_interval_ms old and is set only for PlanWait; for the other actions it is
// -1 (PlanIdle) or 0 (PlanSend).
type Plan struct {
	Action Action
	Take   int
	WaitMS int64
}

// PlanBatch decides what the worker of one destination does next. It is pure. The rule: nothing queued is idle; a full
// batch (queued >= batch_size) leaves at once with batch_size events; a batch whose oldest event is flush_interval_ms
// old (now_ms - oldest_arrival_ms >= flush_interval_ms) leaves with min(queued, batch_size) events; otherwise the worker
// waits until oldest_arrival_ms + flush_interval_ms. The times are milliseconds of the same monotonic clock.
func PlanBatch(now_ms int64, queued int, oldest_arrival_ms int64, batch_size int, flush_interval_ms int64) Plan {
	if queued <= 0 {
		return Plan{Action: PlanIdle, WaitMS: -1}
	}
	if queued >= batch_size {
		return Plan{Action: PlanSend, Take: batch_size}
	}
	if now_ms-oldest_arrival_ms >= flush_interval_ms {
		return Plan{Action: PlanSend, Take: queued}
	}
	return Plan{Action: PlanWait, WaitMS: oldest_arrival_ms + flush_interval_ms - now_ms}
}
