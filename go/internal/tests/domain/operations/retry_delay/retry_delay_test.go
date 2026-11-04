package retry_delay_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/retry_delay"
)

// http_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const http_fixtures_folder = "../../../../../../integration-tests/fixtures/http"

func load_cases(t *testing.T, file_name string, target any) {
	t.Helper()
	content, read_error := os.ReadFile(filepath.Join(http_fixtures_folder, file_name))
	if read_error != nil {
		t.Fatalf("the shared cases are missing: %v", read_error)
	}
	if decode_error := json.Unmarshal(content, target); decode_error != nil {
		t.Fatal(decode_error)
	}
}

func Test_the_shared_backoff_cases_give_the_same_delay(t *testing.T) {
	var file struct {
		Cases []struct {
			Description string  `json:"description"`
			Attempt     int     `json:"attempt"`
			BaseMS      int     `json:"base_ms"`
			MaxMS       int     `json:"max_ms"`
			Random      float64 `json:"random"`
			ExpectedMS  int64   `json:"expected_ms"`
		} `json:"cases"`
	}
	load_cases(t, "backoff.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		if got := retry_delay.BackoffWithJitter(shared_case.Attempt, shared_case.BaseMS, shared_case.MaxMS, shared_case.Random); got != shared_case.ExpectedMS {
			t.Errorf("%s: got %d, expected %d", shared_case.Description, got, shared_case.ExpectedMS)
		}
		// With no hint, the next delay is the backoff.
		if got := retry_delay.NextDelay(shared_case.Attempt, shared_case.BaseMS, shared_case.MaxMS, 0, shared_case.Random); got != shared_case.ExpectedMS {
			t.Errorf("%s: NextDelay got %d, expected %d", shared_case.Description, got, shared_case.ExpectedMS)
		}
	}
}

// The delay that a Retry-After asks is the larger one of the two, as the shared Retry-After cases give it (a hint is
// already read and cut when it comes here; the cut of a hint that is too long is tested apart).
func Test_the_shared_retry_after_cases_are_honored_as_the_next_delay(t *testing.T) {
	var file struct {
		Cases []struct {
			Description     string `json:"description"`
			ExpectedSeconds *int64 `json:"expected_seconds"`
		} `json:"cases"`
	}
	load_cases(t, "retry_after.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		if shared_case.ExpectedSeconds == nil {
			// No hint: the backoff alone (base 1 ms, ceiling 1 ms, random 0 give 0).
			if got := retry_delay.NextDelay(0, 1, 1, 0, 0); got != 0 {
				t.Errorf("%s: got %d", shared_case.Description, got)
			}
			continue
		}
		hint_ms := *shared_case.ExpectedSeconds * 1000
		if got := retry_delay.NextDelay(0, 1, 1, hint_ms, 0); got != hint_ms {
			t.Errorf("%s: got %d, expected %d", shared_case.Description, got, hint_ms)
		}
	}
}

func Test_backoff_edges(t *testing.T) {
	cases := []struct {
		name                     string
		attempt, base_ms, max_ms int
		random                   float64
		expected                 int64
	}{
		{"random below zero is cut", 0, 200, 30000, -3, 100},
		{"random above one is cut", 0, 200, 30000, 7, 200},
		{"not a number is zero", 0, 200, 30000, math.NaN(), 100},
		{"the ceiling is the maximum", 20, 200, 30000, 1, 30000},
		{"no overflow with a huge attempt", 100000, 200, 30000, 0, 15000},
		{"a base above the maximum is cut", 0, 5000, 1000, 1, 1000},
		{"a negative attempt is the first", -4, 200, 30000, 0, 100},
	}
	for _, backoff_case := range cases {
		if got := retry_delay.BackoffWithJitter(backoff_case.attempt, backoff_case.base_ms, backoff_case.max_ms, backoff_case.random); got != backoff_case.expected {
			t.Errorf("%s: got %d, expected %d", backoff_case.name, got, backoff_case.expected)
		}
	}
}

func Test_the_backoff_grows_with_the_attempt_and_the_jitter_stays_between_half_and_all_of_the_ceiling(t *testing.T) {
	previous_ceiling := int64(0)
	for attempt := 0; attempt < 12; attempt++ {
		low := retry_delay.BackoffWithJitter(attempt, 100, 5000, 0)
		high := retry_delay.BackoffWithJitter(attempt, 100, 5000, 1)
		if low*2 != high {
			t.Errorf("attempt %d: low %d and high %d are not half and all of one ceiling", attempt, low, high)
		}
		if high < previous_ceiling {
			t.Errorf("attempt %d: the ceiling went down from %d to %d", attempt, previous_ceiling, high)
		}
		previous_ceiling = high
	}
	if previous_ceiling != 5000 {
		t.Errorf("the ceiling never reached the maximum: %d", previous_ceiling)
	}
}

func Test_next_delay_is_the_larger_of_the_backoff_and_the_hint_and_the_hint_is_cut(t *testing.T) {
	cases := []struct {
		name           string
		retry_after_ms int64
		expected       int64
	}{
		{"no hint", 0, 150},
		{"a negative hint is no hint", -5, 150},
		{"a smaller hint loses", 10, 150},
		{"a larger hint wins", 5000, 5000},
		{"a hint of a day is kept", retry_delay.MaxRetryAfterMilliseconds, retry_delay.MaxRetryAfterMilliseconds},
		{"a hint over a day is cut", retry_delay.MaxRetryAfterMilliseconds * 10, retry_delay.MaxRetryAfterMilliseconds},
		{"the largest hint is cut without overflow", math.MaxInt64, retry_delay.MaxRetryAfterMilliseconds},
	}
	for _, delay_case := range cases {
		if got := retry_delay.NextDelay(0, 200, 30000, delay_case.retry_after_ms, 0.5); got != delay_case.expected {
			t.Errorf("%s: got %d, expected %d", delay_case.name, got, delay_case.expected)
		}
	}
	if retry_delay.MaxRetryAfterMilliseconds != 86400*1000 {
		t.Errorf("the cut is not one day: %d", retry_delay.MaxRetryAfterMilliseconds)
	}
}

func Test_retry_delay_table(t *testing.T) {
	cases := []struct {
		name              string
		attempt           int
		transient         bool
		retry_after_ms    int64
		retry_after_ok    bool
		expected_retry    bool
		expected_delay_ms int64
	}{
		{"transient first attempt", 0, true, 0, false, true, 100},
		{"the last attempt does not retry", 4, true, 0, false, false, 0},
		{"past the last attempt", 9, true, 0, false, false, 0},
		{"not transient never retries", 0, false, 0, false, false, 0},
		{"a larger Retry-After wins", 0, true, 5000, true, true, 5000},
		{"a smaller Retry-After loses", 0, true, 10, true, true, 100},
		{"a Retry-After that is not ok is ignored", 0, true, 5000, false, true, 100},
		{"a Retry-After over a day is cut", 0, true, math.MaxInt64, true, true, retry_delay.MaxRetryAfterMilliseconds},
	}
	for _, delay_case := range cases {
		delay_ms, retry := retry_delay.RetryDelay(delay_case.attempt, 5, delay_case.transient, delay_case.retry_after_ms, delay_case.retry_after_ok, 200, 30000, 0)
		if retry != delay_case.expected_retry || delay_ms != delay_case.expected_delay_ms {
			t.Errorf("%s: got %d %v", delay_case.name, delay_ms, retry)
		}
	}
}

func Test_one_attempt_never_retries(t *testing.T) {
	if _, retry := retry_delay.RetryDelay(0, 1, true, 0, false, 200, 30000, 0.5); retry {
		t.Fatal("with one attempt there is no retry")
	}
	if _, retry := retry_delay.RetryDelay(0, 0, true, 0, false, 200, 30000, 0.5); retry {
		t.Fatal("with no attempts there is no retry")
	}
}
