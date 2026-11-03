package services_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
)

const shutdown_fixture_file = "../../../../../integration-tests/fixtures/delivery/shutdown_results.json"

// flush_sink is a log sink with the behavior of the shared fixture: ok (accepts the whole batch after the delay), hang
// (never answers; it IGNORES its context until release), fail (fails the whole batch) and drop (drops the whole batch).
type flush_sink struct {
	behavior string
	delay    time.Duration

	release      chan struct{}
	release_once sync.Once
	received     atomic.Int64
}

func new_flush_sink(behavior string, delay time.Duration) *flush_sink {
	return &flush_sink{behavior: behavior, delay: delay, release: make(chan struct{})}
}

func (sink *flush_sink) Send(_ context.Context, events []value_objects.Envelope) dtos.SendResult {
	sink.received.Add(int64(len(events)))
	switch sink.behavior {
	case "hang":
		<-sink.release
		return dtos.SendResult{Accepted: len(events)}
	case "fail":
		return dtos.SendResult{Failed: len(events), Reason: "sink_failed"}
	case "drop":
		return dtos.SendResult{Dropped: len(events), Reason: "sink_dropped"}
	}
	if sink.delay > 0 {
		time.Sleep(sink.delay)
	}
	return dtos.SendResult{Accepted: len(events)}
}

// Release lets a hanging Send return.
func (sink *flush_sink) Release() { sink.release_once.Do(func() { close(sink.release) }) }

type shutdown_case struct {
	Name          string `json:"name"`
	BatchSize     int    `json:"batch_size"`
	FlushInterval int    `json:"flush_interval_ms"`
	TimeoutMS     int    `json:"timeout_ms"`
	Events        int    `json:"events"`
	Destinations  []struct {
		Behavior string `json:"behavior"`
		DelayMS  int    `json:"delay_ms"`
	} `json:"destinations"`
	Expected struct {
		Sent                 int  `json:"sent"`
		Spooled              int  `json:"spooled"`
		Dropped              int  `json:"dropped"`
		ReturnsBeforeTimeout bool `json:"returns_before_timeout"`
	} `json:"expected"`
}

// started_with builds and starts a delivery with the sinks. Hanging sinks are released when the test ends, before the
// service is stopped.
func started_with(test *testing.T, batch_size int, flush_interval_ms int, sinks ...*flush_sink) *services.DeliveryService {
	test.Helper()
	log_sinks := make([]outbound.LogSinkPort, 0, len(sinks))
	for _, sink := range sinks {
		log_sinks = append(log_sinks, sink)
	}
	delivery := new_delivery(test, log_destinations(log_sinks...), settings_of(1000, batch_size, flush_interval_ms))
	test.Cleanup(func() {
		for _, sink := range sinks {
			sink.Release()
		}
	})
	delivery.Start()
	return delivery
}

func enqueue_events(delivery *services.DeliveryService, count int) {
	for id := 1; id <= count; id++ {
		enqueue(delivery, normal_event(id))
	}
}

func Test_every_shared_shutdown_case_gives_the_same_result_for_flush_and_shutdown(t *testing.T) {
	content, read_error := os.ReadFile(filepath.FromSlash(shutdown_fixture_file))
	if read_error != nil {
		t.Fatal(read_error)
	}
	var fixture struct {
		Cases []shutdown_case `json:"cases"`
	}
	if unmarshal_error := json.Unmarshal(content, &fixture); unmarshal_error != nil || len(fixture.Cases) == 0 {
		t.Fatalf("fixture: %v (%d cases)", unmarshal_error, len(fixture.Cases))
	}

	for _, shutdown_case := range fixture.Cases {
		for _, call_name := range []string{"flush", "shutdown"} {
			t.Run(shutdown_case.Name+"/"+call_name, func(t *testing.T) {
				sinks := make([]*flush_sink, 0, len(shutdown_case.Destinations))
				for _, destination := range shutdown_case.Destinations {
					sinks = append(sinks, new_flush_sink(destination.Behavior, time.Duration(destination.DelayMS)*time.Millisecond))
				}
				delivery := started_with(t, shutdown_case.BatchSize, shutdown_case.FlushInterval, sinks...)
				enqueue_events(delivery, shutdown_case.Events)

				timeout := time.Duration(shutdown_case.TimeoutMS) * time.Millisecond
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				started_at := time.Now()
				var result dtos.ShutdownResult
				if call_name == "flush" {
					result = delivery.Flush(ctx)
				} else {
					result = delivery.Shutdown(ctx)
				}
				elapsed := time.Since(started_at)

				want := dtos.ShutdownResult{Sent: shutdown_case.Expected.Sent, Spooled: shutdown_case.Expected.Spooled, Dropped: shutdown_case.Expected.Dropped}
				if result != want {
					t.Fatalf("result %+v, want %+v, stats %+v", result, want, delivery.Stats())
				}
				if shutdown_case.Expected.ReturnsBeforeTimeout && elapsed >= timeout {
					t.Fatalf("returned after %v, the timeout is %v", elapsed, timeout)
				}
				if !shutdown_case.Expected.ReturnsBeforeTimeout && (elapsed < timeout*8/10 || elapsed > timeout+300*time.Millisecond) {
					t.Fatalf("returned after %v, the timeout is %v", elapsed, timeout)
				}
			})
		}
	}
}

func Test_a_flush_keeps_the_service_running_and_a_later_event_is_sent_and_reported_by_shutdown(t *testing.T) {
	sink := new_flush_sink("ok", 0)
	delivery := started_with(t, 100, 600000, sink)
	enqueue_events(delivery, 3)

	if result := delivery.Flush(context.Background()); result != (dtos.ShutdownResult{Sent: 3}) {
		t.Fatalf("flush %+v", result)
	}
	if result := enqueue(delivery, normal_event(4)); result != (dtos.EmitResult{Accepted: 1}) {
		t.Fatalf("emit after flush %+v", result)
	}
	if result := delivery.Shutdown(context.Background()); result != (dtos.ShutdownResult{Sent: 1}) {
		t.Fatalf("shutdown %+v", result)
	}
	if sink.received.Load() != 4 {
		t.Fatalf("the sink received %d events", sink.received.Load())
	}
}

func Test_a_second_shutdown_returns_the_first_result_and_a_flush_after_shutdown_is_zero(t *testing.T) {
	sink := new_flush_sink("ok", 0)
	delivery := started_with(t, 100, 600000, sink)
	enqueue_events(delivery, 5)

	first := delivery.Shutdown(context.Background())
	second := delivery.Shutdown(context.Background())
	cancelled_context, cancel := context.WithCancel(context.Background())
	cancel()
	third := delivery.Shutdown(cancelled_context)

	if first != (dtos.ShutdownResult{Sent: 5}) || second != first || third != first {
		t.Fatalf("results %+v, %+v, %+v", first, second, third)
	}
	if result := delivery.Flush(context.Background()); result != (dtos.ShutdownResult{}) {
		t.Fatalf("flush after shutdown %+v", result)
	}
}

func Test_an_event_after_shutdown_is_dropped_with_the_reason_shutdown_and_counted_in_every_destination(t *testing.T) {
	first_sink, second_sink := new_flush_sink("ok", 0), new_flush_sink("ok", 0)
	delivery := started_with(t, 100, 600000, first_sink, second_sink)
	delivery.Shutdown(context.Background())

	result := enqueue(delivery, normal_event(1))

	if result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonShutdown}) || services.ReasonShutdown != "shutdown" {
		t.Fatalf("result %+v", result)
	}
	for _, stats := range delivery.Stats() {
		if stats.Dropped != 1 || stats.DroppedByReason["shutdown"] != 1 || stats.Queued != 0 || stats.Accepted != 0 {
			t.Fatalf("stats %+v", stats)
		}
	}
	if first_sink.received.Load() != 0 || second_sink.received.Load() != 0 {
		t.Fatal("a sink received an event after shutdown")
	}
	if result := delivery.Enqueue(context.Background(), services.DestinationKindMetrics, normal_event(2)); result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonShutdown}) {
		t.Fatalf("a kind with no destination after shutdown: %+v", result)
	}
}

func Test_shutdown_before_any_event_and_on_a_service_that_was_not_started_are_zero_and_do_not_panic(t *testing.T) {
	delivery := started_with(t, 100, 600000, new_flush_sink("ok", 0))
	if result := delivery.Shutdown(context.Background()); result != (dtos.ShutdownResult{}) {
		t.Fatalf("before any event %+v", result)
	}

	not_started := new_delivery(t, log_destinations(new_flush_sink("ok", 0)), settings_of(10, 5, 1000))
	if result := not_started.Flush(context.Background()); result != (dtos.ShutdownResult{}) {
		t.Fatalf("flush of an empty service that was not started %+v", result)
	}
	enqueue_events(not_started, 2)
	if result := not_started.Flush(context.Background()); result != (dtos.ShutdownResult{Dropped: 2}) {
		t.Fatalf("flush with nobody to send %+v", result)
	}
	if result := not_started.Shutdown(context.Background()); result != (dtos.ShutdownResult{Dropped: 2}) {
		t.Fatalf("shutdown with nobody to send %+v", result)
	}
	if stats := not_started.Stats()[0]; stats.Queued != 0 || stats.DroppedByReason["shutdown"] != 2 {
		t.Fatalf("what was queued is removed and counted: %+v", stats)
	}
}

func Test_a_context_that_is_already_cancelled_returns_at_once_with_everything_pending_as_dropped(t *testing.T) {
	sink := new_flush_sink("hang", 0)
	delivery := started_with(t, 100, 600000, sink)
	enqueue_events(delivery, 3)
	cancelled_context, cancel := context.WithCancel(context.Background())
	cancel()

	started_at := time.Now()
	flush_result := delivery.Flush(cancelled_context)
	shutdown_result := delivery.Shutdown(cancelled_context)

	if elapsed := time.Since(started_at); elapsed > 200*time.Millisecond {
		t.Fatalf("it took %v", elapsed)
	}
	if flush_result != (dtos.ShutdownResult{Dropped: 3}) {
		t.Fatalf("flush %+v", flush_result)
	}
	if shutdown_result.Sent != 0 || shutdown_result.Spooled != 0 || shutdown_result.Dropped < 0 || shutdown_result.Dropped > 3 {
		t.Fatalf("shutdown %+v", shutdown_result)
	}
}

func Test_a_nil_context_is_a_background_context_with_the_default_timeout(t *testing.T) {
	delivery := started_with(t, 100, 600000, new_flush_sink("ok", 0))
	enqueue_events(delivery, 2)
	var nil_context context.Context

	if result := delivery.Flush(nil_context); result != (dtos.ShutdownResult{Sent: 2}) {
		t.Fatalf("flush %+v", result)
	}
	if result := delivery.Shutdown(nil_context); result != (dtos.ShutdownResult{}) {
		t.Fatalf("shutdown %+v", result)
	}
}

func Test_a_deadline_shorter_than_the_default_is_the_time_of_the_call(t *testing.T) {
	sink := new_flush_sink("hang", 0)
	delivery := started_with(t, 100, 600000, sink)
	enqueue_events(delivery, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	started_at := time.Now()
	result := delivery.Shutdown(ctx)

	if elapsed := time.Since(started_at); elapsed < 120*time.Millisecond || elapsed > 450*time.Millisecond {
		t.Fatalf("it took %v", elapsed)
	}
	if result != (dtos.ShutdownResult{Dropped: 2}) {
		t.Fatalf("result %+v", result)
	}
}

func Test_a_context_without_a_deadline_uses_the_default_of_five_seconds_and_a_healthy_destination_does_not_wait_for_it(t *testing.T) {
	if domain.DefaultShutdownTimeoutMilliseconds != 5000 {
		t.Fatalf("the default is %d ms", domain.DefaultShutdownTimeoutMilliseconds)
	}
	delivery := started_with(t, 100, 600000, new_flush_sink("ok", 0))
	enqueue_events(delivery, 4)

	started_at := time.Now()
	result := delivery.Shutdown(context.Background())

	if result != (dtos.ShutdownResult{Sent: 4}) || time.Since(started_at) > time.Second {
		t.Fatalf("result %+v after %v", result, time.Since(started_at))
	}
}

func Test_a_caller_that_waits_for_room_is_woken_by_shutdown_with_the_reason_shutdown(t *testing.T) {
	sink := new_flush_sink("hang", 0)
	settings := settings_of(1, 1, 600000)
	settings.QueuePolicy = value_objects.QueuePolicyBlockWithTimeout
	settings.BlockTimeoutMS = 10000
	delivery := new_delivery(t, log_destinations(sink), settings)
	t.Cleanup(sink.Release)
	delivery.Start()
	enqueue(delivery, normal_event(1))
	waiting.Eventually(t, waiting.DefaultTimeout, "the first event is in flight", func() bool { return sink.received.Load() == 1 })
	enqueue(delivery, normal_event(2))

	waiter_result := make(chan dtos.EmitResult, 1)
	go func() { waiter_result <- enqueue(delivery, normal_event(3)) }()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	shutdown_result := delivery.Shutdown(ctx)

	select {
	case result := <-waiter_result:
		if result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonShutdown}) {
			t.Fatalf("the waiter got %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("the waiter was not woken")
	}
	if shutdown_result != (dtos.ShutdownResult{Dropped: 2}) {
		t.Fatalf("shutdown %+v", shutdown_result)
	}
}

func Test_emits_during_shutdown_are_sent_or_dropped_and_never_lost_silently(t *testing.T) {
	sink := new_flush_sink("ok", time.Millisecond)
	delivery := started_with(t, 10, 5, sink)

	var accepted, dropped_after_shutdown, other_drops atomic.Int64
	var emitters sync.WaitGroup
	stop_emitting := make(chan struct{})
	for emitter := 0; emitter < 8; emitter++ {
		emitters.Add(1)
		go func() {
			defer emitters.Done()
			for id := 0; ; id++ {
				select {
				case <-stop_emitting:
					return
				default:
				}
				result := delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id))
				switch {
				case result.Accepted == 1:
					accepted.Add(1)
				case result.Reason == services.ReasonShutdown:
					dropped_after_shutdown.Add(1)
				default:
					other_drops.Add(1)
				}
			}
		}()
	}
	waiting.Eventually(t, waiting.DefaultTimeout, "some events are accepted", func() bool { return accepted.Load() > 20 })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := delivery.Shutdown(ctx)
	waiting.Eventually(t, waiting.DefaultTimeout, "an emit meets the closed service", func() bool { return dropped_after_shutdown.Load() > 0 })
	close(stop_emitting)
	emitters.Wait()

	if result.Sent < 0 || result.Dropped < 0 || result.Spooled != 0 {
		t.Fatalf("result %+v", result)
	}
	if sent_total := sink.received.Load(); sent_total+int64(result.Dropped) < accepted.Load() || int64(result.Sent) > sent_total {
		t.Fatalf("accepted %d, the sink received %d, result %+v: events were lost silently", accepted.Load(), sent_total, result)
	}
	if stats := delivery.Stats()[0]; stats.Queued != 0 || int64(stats.DroppedByReason["shutdown"]) < dropped_after_shutdown.Load() {
		t.Fatalf("stats %+v, drops after shutdown %d", stats, dropped_after_shutdown.Load())
	}
	if stats := delivery.Stats()[0]; stats.Accepted != int(accepted.Load()) {
		t.Fatalf("accepted %d in the stats, %d in the results", stats.Accepted, accepted.Load())
	}
}

func Test_concurrent_flushes_and_a_shutdown_do_not_panic_and_each_event_is_counted_by_the_sink_once(t *testing.T) {
	sink := new_flush_sink("ok", time.Millisecond)
	delivery := started_with(t, 5, 600000, sink)
	enqueue_events(delivery, 40)

	var callers sync.WaitGroup
	results := make([]dtos.ShutdownResult, 6)
	for index := range results {
		callers.Add(1)
		go func() {
			defer callers.Done()
			if index == 5 {
				results[index] = delivery.Shutdown(context.Background())
				return
			}
			results[index] = delivery.Flush(context.Background())
		}()
	}
	callers.Wait()

	if sink.received.Load() != 40 {
		t.Fatalf("the sink received %d events", sink.received.Load())
	}
	for _, result := range results {
		if result.Sent < 0 || result.Sent > 40 || result.Dropped < 0 || result.Spooled != 0 {
			t.Fatalf("result %+v", result)
		}
	}
}
