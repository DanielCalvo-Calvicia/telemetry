package dtos

// =============================================================================
// FLUSH OUTCOME DTO
// =============================================================================

// FlushOutcome is the result of the last flush or shutdown for one destination (REQ-DGN-001). Done says a flush or a
// shutdown has finished (it is false until the first one ends); Sent is the events the destination accepted, Spooled is
// the events written to the spool (zero until T-06.5), Dropped is the rest and AtMS is the wall clock, in milliseconds
// since the Unix epoch, when it ended.
type FlushOutcome struct {
	Done    bool
	Sent    int
	Spooled int
	Dropped int
	AtMS    int64
}
