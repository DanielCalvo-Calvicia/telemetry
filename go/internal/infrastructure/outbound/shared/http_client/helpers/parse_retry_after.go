package helpers

import (
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// PARSE RETRY AFTER
// =============================================================================

// MaxRetryAfterSeconds is the longest wait that a Retry-After header may ask: one day. A longer one is cut, so a server
// cannot make the library wait for ever.
const MaxRetryAfterSeconds = 86400

// The HTTP date forms accepted: IMF-fixdate (RFC 7231) and asctime.
const (
	imf_fixdate_layout = "Mon, 02 Jan 2006 15:04:05 GMT"
	asctime_layout     = "Mon Jan _2 15:04:05 2006"
)

// ParseRetryAfter reads the value of a Retry-After header. A non-negative whole number of digits is seconds; an HTTP
// date gives the time from now to that date (0 when it is in the past). The result in milliseconds is cut to
// MaxRetryAfterSeconds. Anything else (negative, decimal, text, empty, a broken date) gives ok false.
func ParseRetryAfter(header string, now time.Time) (milliseconds int64, ok bool) {
	text := strings.TrimSpace(header)
	if text == "" {
		return 0, false
	}
	if only_digits(text) {
		seconds, parse_error := strconv.ParseInt(text, 10, 64)
		if parse_error != nil || seconds > MaxRetryAfterSeconds {
			seconds = MaxRetryAfterSeconds
		}
		return seconds * 1000, true
	}
	for _, layout := range [...]string{imf_fixdate_layout, asctime_layout} {
		date, parse_error := time.Parse(layout, text)
		if parse_error != nil {
			continue
		}
		wait := date.Sub(now).Milliseconds()
		if wait < 0 {
			wait = 0
		}
		if wait > MaxRetryAfterSeconds*1000 {
			wait = MaxRetryAfterSeconds * 1000
		}
		return wait, true
	}
	return 0, false
}

// only_digits tells whether the text is made of ASCII digits only (and is not empty).
func only_digits(text string) bool {
	if text == "" {
		return false
	}
	for _, character := range text {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
