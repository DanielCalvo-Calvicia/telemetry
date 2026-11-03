package dependencies

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/memory_queue"
)

// NewMemoryQueue builds the default queue of a destination: bounded, in memory, in order.
func NewMemoryQueue(capacity int) outbound.EventQueuePort {
	return memory_queue.NewMemoryQueue(capacity)
}
