package dtos

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// DELIVERY SETTINGS DTO
// =============================================================================

// DeliverySettings says how events leave the process: the delivery mode, the size of the bounded queue, the events in
// one batch, the time between flushes and the time before a network operation gives up. The times are whole
// milliseconds, the same number in Go and in Python. The values are already validated.
type DeliverySettings struct {
	Mode                      value_objects.DeliveryMode
	QueueSize                 int
	BatchSize                 int
	FlushIntervalMilliseconds int
	TimeoutMilliseconds       int
}
