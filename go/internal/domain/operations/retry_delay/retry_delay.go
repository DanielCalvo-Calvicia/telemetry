// Package retry_delay holds the pure rule of a retry: how long to wait before the next attempt (the exponential backoff
// with equal jitter and the Retry-After of the destination) and whether to try again at all (REQ-DLV-003, section 10.6).
// It reads no clock and no random source: the caller gives the random number, so the same inputs give the same delay in
// every language.
package retry_delay

import "math"

// =============================================================================
// RETRY DELAY
// =============================================================================

// MaxRetryAfterMilliseconds is the longest Retry-After that is honored: one day. A longer hint is cut, so a destination
// cannot make a worker wait for ever.
const MaxRetryAfterMilliseconds int64 = 86_400_000

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

// NextDelay returns the wait in milliseconds before the next attempt: the larger of the backoff with jitter of the
// attempt number attempt (0 for the first retry) and the Retry-After of the destination, retry_after_ms, cut to
// MaxRetryAfterMilliseconds. A retry_after_ms that is zero or negative is no hint.
func NextDelay(attempt int, base_ms int, max_ms int, retry_after_ms int64, random float64) int64 {
	delay_ms := BackoffWithJitter(attempt, base_ms, max_ms, random)
	retry_after_ms = min(retry_after_ms, MaxRetryAfterMilliseconds)
	if retry_after_ms > delay_ms {
		delay_ms = retry_after_ms
	}
	return delay_ms
}

// RetryDelay decides whether to try again after the attempt number attempt (0 for the first one) and how long to wait.
// It retries only a transient failure and only while attempt+1 is below max_attempts. The wait is NextDelay with the
// Retry-After of the response when retry_after_ok. When it does not retry it returns 0 and false.
func RetryDelay(attempt int, max_attempts int, transient bool, retry_after_ms int64, retry_after_ok bool, base_ms int, max_ms int, random float64) (delay_ms int64, retry bool) {
	if !transient || attempt+1 >= max_attempts {
		return 0, false
	}
	if !retry_after_ok {
		retry_after_ms = 0
	}
	return NextDelay(attempt, base_ms, max_ms, retry_after_ms, random), true
}
