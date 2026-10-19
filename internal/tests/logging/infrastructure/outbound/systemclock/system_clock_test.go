package systemclock_test

import (
	"context"
	"testing"
	"time"

	systemclock "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/systemclock"
)

func Test_now_returns_the_current_time(t *testing.T) {
	system_clock := systemclock.NewSystemClock()

	before_call := time.Now()
	clock_time := system_clock.Now(context.Background())
	after_call := time.Now()

	if clock_time.Before(before_call) || clock_time.After(after_call) {
		t.Errorf("Now() = %v, want between %v and %v", clock_time, before_call, after_call)
	}
}

func Test_now_advances(t *testing.T) {
	system_clock := systemclock.NewSystemClock()

	first_time := system_clock.Now(context.Background())
	time.Sleep(2 * time.Millisecond)
	second_time := system_clock.Now(context.Background())

	if !second_time.After(first_time) {
		t.Errorf("the second reading %v must be later than the first %v", second_time, first_time)
	}
}
