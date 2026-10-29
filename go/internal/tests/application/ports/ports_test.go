package ports_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

// Every fake satisfies its port.
var (
	_ outbound.ClockPort               = (*fakes.FakeClock)(nil)
	_ outbound.IDGeneratorPort         = (*fakes.FakeIDGenerator)(nil)
	_ outbound.ConfigurationPort       = (*fakes.FakeConfiguration)(nil)
	_ outbound.LogSinkPort             = (*fakes.FakeLogSink)(nil)
	_ outbound.EventSinkPort           = (*fakes.FakeEventSink)(nil)
	_ outbound.MetricsSinkPort         = (*fakes.FakeMetricsSink)(nil)
	_ outbound.TraceSinkPort           = (*fakes.FakeTraceSink)(nil)
	_ outbound.DeviceTelemetrySinkPort = (*fakes.FakeDeviceTelemetrySink)(nil)
	_ outbound.AITelemetrySinkPort     = (*fakes.FakeAITelemetrySink)(nil)
	_ outbound.LocalSpoolPort          = (*fakes.FakeLocalSpool)(nil)
)

func envelope_of(event_id string) value_objects.Envelope {
	return value_objects.Envelope{EventId: event_id}
}

// sink_under_test is what the six fake sinks have in common.
type sink_under_test interface {
	Send(ctx context.Context, events []value_objects.Envelope) dtos.SendResult
	SetResult(result dtos.SendResult)
	SetDelay(delay time.Duration)
	Batches() [][]value_objects.Envelope
	Calls() int
}

func TestFakeSinks(t *testing.T) {
	sinks := map[string]func() sink_under_test{
		"log":    func() sink_under_test { return &fakes.FakeLogSink{} },
		"event":  func() sink_under_test { return &fakes.FakeEventSink{} },
		"metric": func() sink_under_test { return &fakes.FakeMetricsSink{} },
		"trace":  func() sink_under_test { return &fakes.FakeTraceSink{} },
		"device": func() sink_under_test { return &fakes.FakeDeviceTelemetrySink{} },
		"ai":     func() sink_under_test { return &fakes.FakeAITelemetrySink{} },
	}
	ctx := context.Background()
	for name, make_sink := range sinks {
		t.Run(name, func(t *testing.T) {
			sink := make_sink()

			if result := sink.Send(ctx, []value_objects.Envelope{envelope_of("a"), envelope_of("b")}); result != (dtos.SendResult{Accepted: 2}) {
				t.Fatalf("default result = %+v, want 2 accepted", result)
			}
			if result := sink.Send(ctx, nil); result != (dtos.SendResult{}) {
				t.Fatalf("nil slice result = %+v, want empty", result)
			}
			if sink.Calls() != 2 || len(sink.Batches()) != 2 || len(sink.Batches()[1]) != 0 {
				t.Fatalf("calls = %d, batches = %v", sink.Calls(), sink.Batches())
			}

			failure := dtos.SendResult{Failed: 1, Reason: "down"}
			sink.SetResult(failure)
			if result := sink.Send(ctx, []value_objects.Envelope{envelope_of("c")}); result != failure {
				t.Fatalf("result = %+v, want %+v", result, failure)
			}

			delay := 5 * time.Millisecond
			sink.SetDelay(delay)
			started := time.Now()
			sink.Send(ctx, nil)
			if elapsed := time.Since(started); elapsed < delay {
				t.Fatalf("elapsed %v is less than the delay %v", elapsed, delay)
			}
		})
	}
}

func TestFakeSinkCopiesWhatItReceives(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	events := []value_objects.Envelope{envelope_of("original")}
	sink.Send(context.Background(), events)
	events[0] = envelope_of("changed")

	batches := sink.Batches()
	if batches[0][0].EventId != "original" {
		t.Fatalf("the fake recorded %q, want the copy taken at the call", batches[0][0].EventId)
	}
	batches[0][0] = envelope_of("mutated")
	if sink.Batches()[0][0].EventId != "original" {
		t.Fatal("changing the returned batches changed the recording")
	}
}

func TestFakeClock(t *testing.T) {
	ctx := context.Background()
	wall_time, _ := value_objects.NewUTCInstant(time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC))
	clock := fakes.NewFakeClock(wall_time)

	if clock.Now(ctx) != wall_time {
		t.Fatal("Now does not return the wall time that was given")
	}
	later, _ := value_objects.NewUTCInstant(wall_time.Time().Add(time.Hour))
	clock.Set(later)
	if clock.Now(ctx) != later {
		t.Fatal("Now does not return the wall time that was set")
	}

	if clock.ClockInfo(ctx) != (dtos.ClockInfo{}) {
		t.Fatal("the default clock info must say that nothing is known")
	}
	info := dtos.ClockInfo{Source: "ntp", SourceKnown: true, Synced: true, SyncedKnown: true}
	clock.SetClockInfo(info)
	if clock.ClockInfo(ctx) != info {
		t.Fatal("ClockInfo does not return what was set")
	}

	previous := clock.MonotonicNanoseconds(ctx)
	for range 5 {
		current := clock.MonotonicNanoseconds(ctx)
		if current <= previous {
			t.Fatalf("the monotonic reading went from %d to %d", previous, current)
		}
		previous = current
	}
	clock.SetStep(250)
	if next := clock.MonotonicNanoseconds(ctx); next-previous != 250 {
		t.Fatalf("the step was %d, want 250", next-previous)
	}

	now_calls, monotonic_calls, clock_info_calls := clock.Calls()
	if now_calls != 2 || monotonic_calls != 7 || clock_info_calls != 2 {
		t.Fatalf("calls = %d, %d, %d", now_calls, monotonic_calls, clock_info_calls)
	}
}

func TestFakeIDGenerator(t *testing.T) {
	ctx := context.Background()
	queued, err := value_objects.NewEventID("01900000-0000-7000-8000-00000000ffff")
	if err != nil {
		t.Fatal(err)
	}
	generator := fakes.NewFakeIDGenerator(queued)

	first, err := generator.NewEventID(ctx)
	if err != nil || first != queued {
		t.Fatalf("first = %v, %v; want the queued id", first, err)
	}
	second, err := generator.NewEventID(ctx)
	if err != nil || second.String() != "01900000-0000-7000-8000-000000000001" {
		t.Fatalf("second = %v, %v", second, err)
	}
	third, _ := generator.NewEventID(ctx)
	if third == second || value_objects.ValidateEventID(third.String()) != nil {
		t.Fatalf("third = %v must be a new valid UUIDv7", third)
	}

	first_instance, err := generator.NewInstanceID(ctx)
	second_instance, _ := generator.NewInstanceID(ctx)
	if err != nil || first_instance == second_instance || first_instance.String() == "" {
		t.Fatalf("instance ids = %q, %q, %v", first_instance, second_instance, err)
	}
}

func TestFakeConfiguration(t *testing.T) {
	resource, err := value_objects.NewResourceIdentity("service", "instance")
	if err != nil {
		t.Fatal(err)
	}
	given := dtos.Configuration{Resource: resource}
	got, err := fakes.NewFakeConfiguration(given).Configuration(context.Background())
	if err != nil || !reflect.DeepEqual(got, given) {
		t.Fatalf("got %+v, %v; want %+v", got, err, given)
	}
}

func TestFakeLocalSpool(t *testing.T) {
	ctx := context.Background()
	spool := &fakes.FakeLocalSpool{}

	if result := spool.Append(ctx, nil); result != (dtos.SendResult{}) {
		t.Fatalf("empty append = %+v", result)
	}
	if result := spool.Append(ctx, []value_objects.Envelope{envelope_of("a"), envelope_of("b"), envelope_of("c")}); result.Accepted != 3 {
		t.Fatalf("append = %+v, want 3 accepted", result)
	}

	records, err := spool.ReadBatch(ctx, 2)
	if err != nil || len(records) != 2 || records[0].ID != "record-1" || records[1].ID != "record-2" || records[0].Event.EventId != "a" {
		t.Fatalf("batch = %+v, %v", records, err)
	}
	if empty, _ := spool.ReadBatch(ctx, 0); len(empty) != 0 {
		t.Fatalf("a batch of 0 returned %d records", len(empty))
	}

	spool.SetQuotaBytes(1000)
	usage, err := spool.Usage(ctx)
	if err != nil || usage.Events != 3 || usage.Bytes <= 0 || usage.QuotaBytes != 1000 {
		t.Fatalf("usage = %+v, %v", usage, err)
	}

	if err := spool.Acknowledge(ctx, []string{"record-1", "unknown"}); err != nil {
		t.Fatal(err)
	}
	if err := spool.Acknowledge(ctx, nil); err != nil {
		t.Fatal(err)
	}
	remaining, _ := spool.ReadBatch(ctx, 10)
	if len(remaining) != 2 || remaining[0].ID != "record-2" {
		t.Fatalf("remaining = %+v", remaining)
	}

	cleanup, err := spool.Cleanup(ctx)
	if err != nil || cleanup.RemovedEvents != 1 || cleanup.FreedBytes <= 0 {
		t.Fatalf("cleanup = %+v, %v", cleanup, err)
	}
	if usage, _ = spool.Usage(ctx); usage.Events != 2 {
		t.Fatalf("usage after cleanup = %+v", usage)
	}
	if again, _ := spool.Cleanup(ctx); again != (dtos.SpoolCleanup{}) {
		t.Fatalf("second cleanup = %+v, want nothing", again)
	}

	events := []value_objects.Envelope{envelope_of("d")}
	spool.Append(ctx, events)
	events[0] = envelope_of("changed")
	if last, _ := spool.ReadBatch(ctx, 10); last[len(last)-1].Event.EventId != "d" {
		t.Fatal("the spool did not keep its own copy")
	}
}

func TestFakeLocalSpoolFailsAndOverridesAppend(t *testing.T) {
	ctx := context.Background()
	spool := &fakes.FakeLocalSpool{}
	spool.Append(ctx, []value_objects.Envelope{envelope_of("a")})

	broken := errors.New("disk full")
	spool.FailWith(broken)
	if _, err := spool.ReadBatch(ctx, 1); !errors.Is(err, broken) {
		t.Fatalf("ReadBatch error = %v", err)
	}
	if err := spool.Acknowledge(ctx, []string{"record-1"}); !errors.Is(err, broken) {
		t.Fatalf("Acknowledge error = %v", err)
	}
	if _, err := spool.Usage(ctx); !errors.Is(err, broken) {
		t.Fatalf("Usage error = %v", err)
	}
	if _, err := spool.Cleanup(ctx); !errors.Is(err, broken) {
		t.Fatalf("Cleanup error = %v", err)
	}

	spool.FailWith(nil)
	if records, err := spool.ReadBatch(ctx, 1); err != nil || len(records) != 1 {
		t.Fatalf("after clearing the failure: %v, %v", records, err)
	}

	override := dtos.SendResult{Dropped: 1, Reason: "quota"}
	spool.SetAppendResult(override)
	if result := spool.Append(ctx, []value_objects.Envelope{envelope_of("b")}); result != override {
		t.Fatalf("append = %+v, want %+v", result, override)
	}
	if usage, _ := spool.Usage(ctx); usage.Events != 1 {
		t.Fatalf("an overridden append must store nothing, usage = %+v", usage)
	}
}

// TestPortMethods lists the methods of every port, so that a port cannot change without this table changing.
func TestPortMethods(t *testing.T) {
	expected_methods := map[reflect.Type][]string{
		reflect.TypeOf((*outbound.ClockPort)(nil)).Elem():               {"ClockInfo", "MonotonicNanoseconds", "Now"},
		reflect.TypeOf((*outbound.IDGeneratorPort)(nil)).Elem():         {"NewEventID", "NewInstanceID"},
		reflect.TypeOf((*outbound.ConfigurationPort)(nil)).Elem():       {"Configuration"},
		reflect.TypeOf((*outbound.LogSinkPort)(nil)).Elem():             {"Send"},
		reflect.TypeOf((*outbound.EventSinkPort)(nil)).Elem():           {"Send"},
		reflect.TypeOf((*outbound.MetricsSinkPort)(nil)).Elem():         {"Send"},
		reflect.TypeOf((*outbound.TraceSinkPort)(nil)).Elem():           {"Send"},
		reflect.TypeOf((*outbound.DeviceTelemetrySinkPort)(nil)).Elem(): {"Send"},
		reflect.TypeOf((*outbound.AITelemetrySinkPort)(nil)).Elem():     {"Send"},
		reflect.TypeOf((*outbound.LocalSpoolPort)(nil)).Elem():          {"Acknowledge", "Append", "Cleanup", "ReadBatch", "Usage"},
	}
	for port_type, expected := range expected_methods {
		actual := make([]string, 0, port_type.NumMethod())
		for position := range port_type.NumMethod() {
			method := port_type.Method(position)
			actual = append(actual, method.Name)
			if method.Type.NumIn() == 0 || method.Type.In(0).String() != "context.Context" {
				t.Errorf("%s.%s does not take context.Context first", port_type.Name(), method.Name)
			}
		}
		sort.Strings(actual)
		if !slices.Equal(actual, expected) {
			t.Errorf("%s has the methods %v, want %v", port_type.Name(), actual, expected)
		}
	}
}
