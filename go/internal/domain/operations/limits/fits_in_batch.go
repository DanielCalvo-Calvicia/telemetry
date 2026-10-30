package limits

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// ===== FitsInBatch ==========================================================

// FitsInBatch reports whether an event of event_bytes bytes can join a batch that already holds batch_bytes bytes
// without going over the limit of the batch: batch_bytes + event_bytes <= MaxBatchBytes. It is pure. The framing bytes
// of a batch (brackets, separators) belong to the batcher (T-04.4), which adds them to the numbers it passes.
func FitsInBatch(batch_bytes int, event_bytes int, limits value_objects.Limits) bool {
	return batch_bytes+event_bytes <= limits.MaxBatchBytes()
}
