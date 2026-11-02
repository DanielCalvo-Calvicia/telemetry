package system_clock_test

import (
	"context"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/system_clock"
)

func Test_now_is_utc_with_microsecond_precision(t *testing.T) {
	system_clock_under_test := system_clock.NewSystemClock()
	before := time.Now().UTC().Add(-time.Second)
	instant := system_clock_under_test.Now(context.Background())
	after := time.Now().UTC().Add(time.Second)

	if instant.Time().Before(before) || instant.Time().After(after) {
		t.Fatalf("the instant %s is not the time of the machine", instant)
	}
	if _, offset := instant.Time().Zone(); offset != 0 {
		t.Fatalf("the instant is not in UTC: offset %d", offset)
	}
	if instant.Time().Nanosecond()%1000 != 0 {
		t.Fatalf("the instant has more than microsecond precision: %d", instant.Time().Nanosecond())
	}
}

func Test_monotonic_never_goes_back(t *testing.T) {
	system_clock_under_test := system_clock.NewSystemClock()
	previous := system_clock_under_test.MonotonicNanoseconds(context.Background())
	if previous < 0 {
		t.Fatalf("the first reading is negative: %d", previous)
	}
	for index := 0; index < 1000; index++ {
		current := system_clock_under_test.MonotonicNanoseconds(context.Background())
		if current < previous {
			t.Fatalf("the reading went back from %d to %d", previous, current)
		}
		previous = current
	}
}

func Test_clock_info_is_zero_because_the_system_clock_knows_nothing_about_its_source(t *testing.T) {
	if got := system_clock.NewSystemClock().ClockInfo(context.Background()); got != (dtos.ClockInfo{}) {
		t.Fatalf("expected the zero ClockInfo, got %+v", got)
	}
}

func Test_the_injected_functions_are_used(t *testing.T) {
	wall_time := time.Date(2026, 10, 28, 12, 0, 0, 123456789, time.FixedZone("CET", 3600))
	system_clock_under_test := system_clock.NewSystemClockWithFunctions(
		func() time.Time { return wall_time },
		func() int64 { return 42 },
	)

	if got := system_clock_under_test.Now(context.Background()).String(); got != "2026-10-28T11:00:00.123456Z" {
		t.Fatalf("got %s", got)
	}
	if got := system_clock_under_test.MonotonicNanoseconds(context.Background()); got != 42 {
		t.Fatalf("got %d", got)
	}
}

func Test_a_time_the_domain_refuses_gives_the_zero_instant_and_never_panics(t *testing.T) {
	system_clock_under_test := system_clock.NewSystemClockWithFunctions(
		func() time.Time { return time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) },
		func() int64 { return 0 },
	)
	if got := system_clock_under_test.Now(context.Background()).String(); got != "0001-01-01T00:00:00.000000Z" {
		t.Fatalf("got %s", got)
	}
}
