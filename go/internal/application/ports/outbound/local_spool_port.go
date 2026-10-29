package outbound

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// LOCAL SPOOL PORT
// =============================================================================

// LocalSpoolPort defines the local store of events that could not be delivered yet: append, read in batches,
// acknowledge, size and clean up. It is only the contract; an adapter gives it a storage.
type LocalSpoolPort interface {
	// Append stores the events and reports how many were accepted, dropped or failed.
	Append(ctx context.Context, events []value_objects.Envelope) dtos.SendResult

	// ReadBatch returns up to maximum_events records that are not acknowledged yet.
	ReadBatch(ctx context.Context, maximum_events int) ([]dtos.SpoolRecord, error)

	// Acknowledge removes the records with the ids, because they were delivered.
	Acknowledge(ctx context.Context, record_ids []string) error

	// Usage reports how many events and bytes the spool holds and its quota.
	Usage(ctx context.Context) (dtos.SpoolUsage, error)

	// Cleanup removes what the spool no longer needs and reports what it removed.
	Cleanup(ctx context.Context) (dtos.SpoolCleanup, error)
}
