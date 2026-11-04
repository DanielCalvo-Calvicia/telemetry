package outbound

// =============================================================================
// RANDOM PORT
// =============================================================================

// RandomPort defines the random numbers the core needs: the jitter of the wait between two attempts of a retry. It is a
// port so a test can give a fixed number and the delays are the same in every run.
type RandomPort interface {
	// Float64 returns a number in [0, 1]. It is safe for concurrent use and never fails.
	Float64() float64
}
