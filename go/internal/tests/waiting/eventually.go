// Package waiting holds the helpers of the tests that wait for something asynchronous to happen: the delivery of the
// events is done by workers, so a test waits for the fact instead of sleeping for a fixed time.
package waiting

import (
	"testing"
	"time"
)

// poll_interval is the pause between two checks of a condition.
const poll_interval = 2 * time.Millisecond

// DefaultTimeout is how long Eventually waits when a test has no reason to wait less or more.
const DefaultTimeout = 5 * time.Second

// Eventually checks the condition every few milliseconds until it is true or the timeout passes; it fails the test with
// the description of what it waited for when the timeout passes. The condition runs in the goroutine of the test.
func Eventually(test *testing.T, timeout time.Duration, description string, condition func() bool) {
	test.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if condition() {
			return
		}
		if time.Now().After(deadline) {
			test.Fatalf("timed out after %v waiting for: %s", timeout, description)
		}
		time.Sleep(poll_interval)
	}
}

// StaysTrue checks that the condition is true now and keeps being true for the whole duration; it fails the test with
// the description when it is ever false. Tests use it to prove that nothing extra happens after the expected fact.
func StaysTrue(test *testing.T, duration time.Duration, description string, condition func() bool) {
	test.Helper()
	deadline := time.Now().Add(duration)
	for {
		if !condition() {
			test.Fatalf("it stopped being true within %v: %s", duration, description)
		}
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(poll_interval)
	}
}
