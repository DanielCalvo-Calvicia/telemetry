package composition_root_test

import (
	"context"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/containers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

func events_received(sink *fakes.FakeLogSink) int {
	total := 0
	for _, batch := range sink.Batches() {
		total += len(batch)
	}
	return total
}

func Test_flush_sends_what_is_queued_at_once_and_the_core_keeps_running(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	cfg := service_config("service")
	cfg.Delivery.FlushIntervalMS = 600000
	core := new_core(t, cfg, config.WithLogSink(sink))
	t.Cleanup(func() { core.Shutdown(context.Background()) })
	for count := 0; count < 3; count++ {
		core.Emit(context.Background(), log_input())
	}

	flush_result := core.Flush(context.Background())

	if flush_result != (dtos.ShutdownResult{Sent: 3}) || events_received(sink) != 3 {
		t.Fatalf("flush %+v, the sink received %d", flush_result, events_received(sink))
	}
	if result := core.Emit(context.Background(), log_input()); result != (dtos.EmitResult{Accepted: 1}) {
		t.Fatalf("emit after flush %+v", result)
	}
	if shutdown_result := core.Shutdown(context.Background()); shutdown_result != (dtos.ShutdownResult{Sent: 1}) || events_received(sink) != 4 {
		t.Fatalf("shutdown %+v, the sink received %d", shutdown_result, events_received(sink))
	}
}

func Test_shutdown_sends_the_queue_and_after_it_every_emit_and_log_call_is_dropped_with_the_reason_shutdown(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	cfg := service_config("service")
	cfg.Delivery.FlushIntervalMS = 600000
	core := new_core(t, cfg, config.WithLogSink(sink))
	core.Logger().Info(context.Background(), dtos.LogEntryInput{Category: "http", Message: "one"})
	core.Logger().Warn(context.Background(), dtos.LogEntryInput{Category: "http", Message: "two"})

	first := core.Shutdown(context.Background())
	second := core.Shutdown(context.Background())

	if first != (dtos.ShutdownResult{Sent: 2}) || second != first || events_received(sink) != 2 {
		t.Fatalf("results %+v and %+v, the sink received %d", first, second, events_received(sink))
	}
	if result := core.Flush(context.Background()); result != (dtos.ShutdownResult{}) {
		t.Fatalf("flush after shutdown %+v", result)
	}
	if result := core.Emit(context.Background(), log_input()); result != (dtos.EmitResult{Dropped: 1, Reason: "shutdown"}) {
		t.Fatalf("emit after shutdown %+v", result)
	}
	core.Logger().Error(context.Background(), dtos.LogEntryInput{Category: "http", Message: "late"})
	if events_received(sink) != 2 {
		t.Fatal("an event was delivered after shutdown")
	}
}

func Test_shutdown_returns_at_its_deadline_when_a_sink_hangs_and_counts_the_events_as_dropped(t *testing.T) {
	sink := fakes.NewBlockingLogSink()
	t.Cleanup(sink.Release)
	cfg := service_config("service")
	cfg.Delivery.BatchSize = 1
	core := new_core(t, cfg, config.WithLogSink(sink))
	core.Emit(context.Background(), log_input())
	core.Emit(context.Background(), log_input())
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	started_at := time.Now()
	result := core.Shutdown(ctx)

	if elapsed := time.Since(started_at); elapsed < 150*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Fatalf("it took %v", elapsed)
	}
	if result.Sent != 0 || result.Spooled != 0 || result.Dropped != 2 {
		t.Fatalf("result %+v", result)
	}
}

func Test_flush_and_shutdown_of_a_nil_or_zero_core_are_the_zero_result(t *testing.T) {
	var nil_core *containers.Core
	for name, core := range map[string]*containers.Core{"nil": nil_core, "zero": {}} {
		if result := core.Flush(context.Background()); result != (dtos.ShutdownResult{}) {
			t.Fatalf("%s core: flush %+v", name, result)
		}
		if result := core.Shutdown(context.Background()); result != (dtos.ShutdownResult{}) {
			t.Fatalf("%s core: shutdown %+v", name, result)
		}
	}
}

func Test_flush_and_shutdown_of_a_core_with_no_sink_and_no_event_are_zero(t *testing.T) {
	core := new_core(t, service_config("service"))
	if result := core.Flush(context.Background()); result != (dtos.ShutdownResult{}) {
		t.Fatalf("flush %+v", result)
	}
	if result := core.Shutdown(context.Background()); result != (dtos.ShutdownResult{}) {
		t.Fatalf("shutdown %+v", result)
	}
}

func Test_after_shutdown_every_input_is_dropped_with_the_reason_shutdown_before_it_is_built(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	core := new_core(t, service_config("service"), config.WithLogSink(sink))
	core.Shutdown(context.Background())
	want := dtos.EmitResult{Dropped: 1, Reason: "shutdown"}

	inputs := map[string]dtos.NewEventInput{
		"valid":                    log_input(),
		"invalid event":            {EventType: "log.entry.recorded", Severity: "not-a-severity"},
		"empty input":              {},
		"kind with no destination": {EventType: "metric.counter.recorded"},
	}
	for name, input := range inputs {
		if result := core.Emit(context.Background(), input); result != want {
			t.Fatalf("%s: %+v", name, result)
		}
	}
}
