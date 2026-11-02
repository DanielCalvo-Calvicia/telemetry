package helpers

import "math"

// =============================================================================
// BACKOFF WITH JITTER
// =============================================================================

// BackoffWithJitter returns the wait in milliseconds before the retry number attempt (0 for the first retry):
// ceiling = min(max_ms, base_ms * 2^attempt) and delay = floor(ceiling/2 + random * ceiling/2), the equal jitter (half
// fixed, half random). random is in [0, 1] and a value outside is cut to it. The doubling stops when it reaches the
// maximum, so it never overflows.
func BackoffWithJitter(attempt int, base_ms int, max_ms int, random float64) int64 {
	ceiling := int64(base_ms)
	for step := 0; step < attempt && ceiling < int64(max_ms); step++ {
		ceiling *= 2
	}
	if ceiling > int64(max_ms) {
		ceiling = int64(max_ms)
	}
	if math.IsNaN(random) || random < 0 {
		random = 0
	}
	if random > 1 {
		random = 1
	}
	half := float64(ceiling) / 2
	return int64(math.Floor(half + random*half))
}

// RetryDelay decides whether to try again after the attempt number attempt (0 for the first one) and how long to wait.
// It retries only a transient class and only while attempt+1 is below max_attempts. The wait is the larger of the
// backoff and the Retry-After of the response (when retry_after_ok). When it does not retry it returns 0 and false.
func RetryDelay(attempt int, max_attempts int, class Class, retry_after_ms int64, retry_after_ok bool, base_ms int, max_ms int, random float64) (delay_ms int64, retry bool) {
	if class != ClassTransient || attempt+1 >= max_attempts {
		return 0, false
	}
	delay_ms = BackoffWithJitter(attempt, base_ms, max_ms, random)
	if retry_after_ok && retry_after_ms > delay_ms {
		delay_ms = retry_after_ms
	}
	return delay_ms, true
}
