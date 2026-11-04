package dtos

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// DELIVERY SETTINGS DTO
// =============================================================================

// DeliverySettings says how events leave the process: the delivery mode, the size of the bounded queue, the events in
// one batch, the time between flushes, the time before a network operation gives up, what a full queue does, the time a
// call may wait for room with the policy block_with_timeout and the class of each event type that has one assigned (a
// priority for a full queue, never a guarantee). With the retry mode, RetryMaxAttempts is the attempts at most for one
// batch (the first included) and the delay between two attempts is a backoff with jitter from RetryBaseDelayMS to
// RetryMaxDelayMS. The times are whole milliseconds, the same number in Go and in Python.
// The values are already validated.
type DeliverySettings struct {
	Mode                      value_objects.DeliveryMode
	QueueSize                 int
	BatchSize                 int
	FlushIntervalMilliseconds int
	TimeoutMilliseconds       int
	QueuePolicy               value_objects.QueuePolicy
	BlockTimeoutMilliseconds  int
	EventClasses              map[string]value_objects.EventClass
	RetryMaxAttempts          int
	RetryBaseDelayMS          int
	RetryMaxDelayMS           int
}
