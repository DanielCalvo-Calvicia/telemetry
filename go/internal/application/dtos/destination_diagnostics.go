package dtos

// =============================================================================
// DESTINATION DIAGNOSTICS DTO
// =============================================================================

// DestinationDiagnostics is how the delivery to one destination is doing (REQ-DGN-001). Kind and Index say which sink it
// is. QueueDepth is the events waiting now and QueueCapacity the size of the queue. Accepted is the events that were
// queued, Delivered the events the sink accepted, Dropped the events that were lost and DroppedByReason the same count
// per reason (its sum is Dropped); FailedEvents is the events a sink failed to take. ExportFailures is the calls to the
// sink in which some event failed or was dropped; Attempts is the calls, Retries the calls beyond the first of a batch
// and Batches the batches that ended. LastBatchSize and MaxBatchSize are the events of the last and of the largest batch.
// The export latency, in whole milliseconds, is the time of each call to the sink: the last one, the longest one and the
// total of all. LastSuccessAtMS is the wall clock, in milliseconds since the Unix epoch, of the last call in which the
// sink accepted an event (zero means never). ConfigurationErrors and SerializationFailures are the failed and dropped
// events of the answers whose reason is configuration_error and serialization_failed. SpoolEvents, SpoolBytes and
// SpoolQuotaBytes are zero until the spool exists (T-06.5). LastFlush is the result of the last flush or shutdown and
// LastReason the last reason that was not empty. A snapshot is a copy: changing it changes nothing in the library.
type DestinationDiagnostics struct {
	Kind                  string
	Index                 int
	QueueDepth            int
	QueueCapacity         int
	Accepted              int
	Delivered             int
	Dropped               int
	DroppedByReason       map[string]int
	FailedEvents          int
	ExportFailures        int
	Attempts              int
	Retries               int
	Batches               int
	LastBatchSize         int
	MaxBatchSize          int
	LastExportLatencyMS   int64
	MaxExportLatencyMS    int64
	TotalExportLatencyMS  int64
	LastSuccessAtMS       int64
	ConfigurationErrors   int
	SerializationFailures int
	SpoolEvents           int
	SpoolBytes            int64
	SpoolQuotaBytes       int64
	LastFlush             FlushOutcome
	LastReason            string
}
