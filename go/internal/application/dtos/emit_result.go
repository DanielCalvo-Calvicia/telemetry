package dtos

// =============================================================================
// EMIT RESULT DTO
// =============================================================================

// EmitResult is what the core answers to one input: how many events were accepted, dropped or failed, summed over the
// destinations that received it, and the reason of the first drop or failure (an error code of the catalog, or one of
// the reasons of the pipeline). Reason is empty when nothing was dropped or failed. It is a result, never an error: a
// caller of the core never has to handle a failure of the telemetry (ADR-005).
type EmitResult struct {
	Accepted int
	Dropped  int
	Failed   int
	Reason   string
}
