package services_test

import (
	"context"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/dependencies"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
)

func numbered(id int) value_objects.Envelope {
	return value_objects.Envelope{EventId: strconv.Itoa(id)}
}

// new_delivery builds a delivery service with the system clock and the memory queue. It does not start it. It is stopped
// when the test ends.
func new_delivery(test *testing.T, destinations services.Destinations, settings services.DeliverySettings) *services.DeliveryService {
	test.Helper()
	delivery := services.NewDeliveryService(destinations, settings, dependencies.NewSystemClock(), fakes.NewFakeRandom(), nil, dependencies.NewMemoryQueue)
	test.Cleanup(delivery.Stop)
	return delivery
}

func settings_of(queue_size int, batch_size int, flush_interval_ms int) services.DeliverySettings {
	return services.DeliverySettings{QueueSize: queue_size, BatchSize: batch_size, FlushIntervalMS: flush_interval_ms, TimeoutMS: 5000}
}

func log_destinations(sinks ...outbound.LogSinkPort) services.Destinations {
	return services.Destinations{Log: sinks}
}

func received_ids(sink *fakes.FakeLogSink) []string {
	ids := []string{}
	for _, batch := range sink.Batches() {
		for _, envelope := range batch {
			ids = append(ids, envelope.EventId)
		}
	}
	return ids
}

func Test_nothing_starts_before_start_and_start_is_idempotent(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	goroutines_before := runtime.NumGoroutine()
	delivery := new_delivery(t, log_destinations(sink), settings_of(100, 2, 5))
	if runtime.NumGoroutine() > goroutines_before {
		t.Fatal("the constructor started a goroutine")
	}

	for id := 1; id <= 4; id++ {
		if result := delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id)); result != (dtos.EmitResult{Accepted: 1}) {
			t.Fatalf("result %+v", result)
		}
	}
	waiting.StaysTrue(t, 60*time.Millisecond, "no Send happens before Start", func() bool { return sink.Calls() == 0 })
	if stats := delivery.Stats(); stats[0].Queued != 4 || stats[0].Batches != 0 {
		t.Fatalf("stats %+v", stats[0])
	}

	delivery.Start()
	delivery.Start()
	waiting.Eventually(t, waiting.DefaultTimeout, "the four events are delivered", func() bool { return len(received_ids(sink)) == 4 })
	waiting.StaysTrue(t, 30*time.Millisecond, "nothing is delivered twice", func() bool { return len(received_ids(sink)) == 4 })
}

func Test_stop_is_idempotent_returns_and_stops_the_workers(t *testing.T) {
	goroutines_before := runtime.NumGoroutine()
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}, &fakes.FakeLogSink{}), settings_of(100, 2, 5))
	delivery.Start()

	done := make(chan struct{})
	go func() {
		delivery.Stop()
		delivery.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return")
	}
	waiting.Eventually(t, waiting.DefaultTimeout, "the workers are gone", func() bool { return runtime.NumGoroutine() <= goroutines_before })

	delivery.Start() // after Stop it does nothing
	delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(1))
	waiting.StaysTrue(t, 30*time.Millisecond, "a stopped service starts no worker", func() bool { return runtime.NumGoroutine() <= goroutines_before })
}

func Test_stop_before_start_returns(t *testing.T) {
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), settings_of(10, 2, 5))
	delivery.Stop()
}

func Test_ten_thousand_enqueues_against_a_destination_that_never_answers_return_at_once(t *testing.T) {
	blocking_sink := fakes.NewBlockingLogSink()
	t.Cleanup(blocking_sink.Release)
	settings := services.DeliverySettings{QueueSize: 20000, BatchSize: 100, FlushIntervalMS: 10, TimeoutMS: 60000}
	delivery := new_delivery(t, log_destinations(blocking_sink), settings)
	delivery.Start()
	delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(0))
	select {
	case <-blocking_sink.Started():
	case <-time.After(5 * time.Second):
		t.Fatal("the worker never called the sink")
	}

	started_at := time.Now()
	accepted := 0
	for id := 1; id <= 10000; id++ {
		accepted += delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id)).Accepted
	}
	elapsed := time.Since(started_at)

	if accepted != 10000 {
		t.Fatalf("accepted %d", accepted)
	}
	if elapsed > time.Second {
		t.Fatalf("10,000 calls took %v against a destination that never answers", elapsed)
	}
	if stats := delivery.Stats(); stats[0].Queued != 10000 {
		t.Fatalf("queued %d", stats[0].Queued)
	}
}

func Test_a_full_batch_leaves_without_waiting_for_the_interval(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	delivery := new_delivery(t, log_destinations(sink), settings_of(100, 5, 10000))
	delivery.Start()
	started_at := time.Now()

	for id := 1; id <= 5; id++ {
		delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id))
	}

	waiting.Eventually(t, waiting.DefaultTimeout, "the full batch is sent", func() bool { return sink.Calls() == 1 })
	if time.Since(started_at) > 3*time.Second {
		t.Fatal("the full batch waited for the interval")
	}
	if batches := sink.Batches(); len(batches[0]) != 5 {
		t.Fatalf("batch of %d", len(batches[0]))
	}
}

func Test_a_smaller_batch_leaves_after_the_interval(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	delivery := new_delivery(t, log_destinations(sink), settings_of(100, 100, 40))
	delivery.Start()
	started_at := time.Now()

	for id := 1; id <= 3; id++ {
		delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id))
	}

	waiting.Eventually(t, waiting.DefaultTimeout, "the small batch is sent", func() bool { return sink.Calls() >= 1 })
	if elapsed := time.Since(started_at); elapsed < 25*time.Millisecond {
		t.Fatalf("the batch left after %v, before the interval of 40 ms", elapsed)
	}
	waiting.StaysTrue(t, 30*time.Millisecond, "the three events are one batch", func() bool { return sink.Calls() == 1 && len(sink.Batches()[0]) == 3 })
}

func Test_the_order_of_a_destination_is_kept_across_batches(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	delivery := new_delivery(t, log_destinations(sink), settings_of(2000, 7, 5))
	delivery.Start()

	for id := 1; id <= 1000; id++ {
		delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id))
	}

	waiting.Eventually(t, waiting.DefaultTimeout, "all events are delivered", func() bool { return len(received_ids(sink)) == 1000 })
	for index, id := range received_ids(sink) {
		if id != strconv.Itoa(index+1) {
			t.Fatalf("event %d is %s", index+1, id)
		}
	}
	for _, batch := range sink.Batches() {
		if len(batch) > 7 {
			t.Fatalf("a batch of %d, over the batch size", len(batch))
		}
	}
}

func Test_a_destination_that_hangs_does_not_delay_a_second_destination(t *testing.T) {
	blocking_sink := fakes.NewBlockingLogSink()
	t.Cleanup(blocking_sink.Release)
	healthy_sink := &fakes.FakeLogSink{}
	delivery := new_delivery(t, log_destinations(blocking_sink, healthy_sink), settings_of(100, 3, 5))
	delivery.Start()

	for id := 1; id <= 10; id++ {
		delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id))
	}

	waiting.Eventually(t, waiting.DefaultTimeout, "the healthy destination receives everything", func() bool { return len(received_ids(healthy_sink)) == 10 })
	stats := delivery.Stats()
	if stats[0].Index != 0 || stats[1].Index != 1 || stats[0].Queued == 0 {
		t.Fatalf("stats %+v", stats)
	}
}

func Test_a_panic_of_a_sink_is_contained_and_counted_and_the_worker_goes_on(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	sink.SetPanic(true)
	delivery := new_delivery(t, log_destinations(sink), settings_of(100, 3, 10000))
	delivery.Start()

	for id := 1; id <= 3; id++ {
		delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id))
	}

	waiting.Eventually(t, waiting.DefaultTimeout, "the panic is counted", func() bool { return delivery.Stats()[0].Failed == 3 })
	if stats := delivery.Stats()[0]; stats.LastReason != services.ReasonSinkPanicked || stats.Batches != 1 {
		t.Fatalf("stats %+v", stats)
	}
	sink.SetPanic(false)
	for id := 4; id <= 6; id++ {
		delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id))
	}
	waiting.Eventually(t, waiting.DefaultTimeout, "the worker still delivers", func() bool { return delivery.Stats()[0].Delivered == 3 })
}

func Test_the_timeout_reaches_send_as_a_deadline_of_the_context(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	settings := services.DeliverySettings{QueueSize: 10, BatchSize: 1, FlushIntervalMS: 5, TimeoutMS: 750}
	delivery := new_delivery(t, log_destinations(sink), settings)
	delivery.Start()
	before := time.Now()

	delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(1))

	waiting.Eventually(t, waiting.DefaultTimeout, "the sink is called", func() bool { return sink.Calls() == 1 })
	after := time.Now()
	deadline := sink.Deadlines()[0]
	if deadline.IsZero() || deadline.Before(before.Add(750*time.Millisecond)) || deadline.After(after.Add(750*time.Millisecond)) {
		t.Fatalf("the deadline %v is not 750 ms after the call (between %v and %v)", deadline, before, after)
	}
}

func Test_stop_cancels_a_call_in_flight_and_returns(t *testing.T) {
	blocking_sink := fakes.NewBlockingLogSink()
	t.Cleanup(blocking_sink.Release)
	settings := services.DeliverySettings{QueueSize: 10, BatchSize: 1, FlushIntervalMS: 5, TimeoutMS: 60000}
	delivery := new_delivery(t, log_destinations(blocking_sink), settings)
	delivery.Start()
	delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(1))
	<-blocking_sink.Started()

	done := make(chan struct{})
	go func() {
		delivery.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop waited for a call that never answers")
	}
}

func Test_a_full_queue_drops_the_new_event_with_queue_full_newest(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	delivery := new_delivery(t, log_destinations(sink), settings_of(3, 3, 10000)) // not started: the queue only fills

	var results []dtos.EmitResult
	for id := 1; id <= 5; id++ {
		results = append(results, delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id)))
	}

	for index, result := range results {
		want := dtos.EmitResult{Accepted: 1}
		if index >= 3 {
			want = dtos.EmitResult{Dropped: 1, Reason: services.ReasonQueueFullNewest}
		}
		if result != want {
			t.Fatalf("result %d is %+v, want %+v", index, result, want)
		}
	}
	stats := delivery.Stats()[0]
	if stats.Accepted != 3 || stats.Dropped != 2 || stats.Queued != 3 || stats.LastReason != services.ReasonQueueFullNewest || stats.DroppedByReason[services.ReasonQueueFullNewest] != 2 {
		t.Fatalf("stats %+v", stats)
	}
}

func Test_the_result_sums_the_destinations_of_the_kind_and_the_reason_is_the_first(t *testing.T) {
	full_sink, other_sink := &fakes.FakeLogSink{}, &fakes.FakeLogSink{}
	created := 0
	new_queue := func(capacity int) outbound.EventQueuePort {
		created++
		if created == 1 {
			return dependencies.NewMemoryQueue(1)
		}
		return dependencies.NewMemoryQueue(capacity)
	}
	delivery := services.NewDeliveryService(log_destinations(full_sink, other_sink), settings_of(10, 2, 5), dependencies.NewSystemClock(), fakes.NewFakeRandom(), nil, new_queue)

	first := delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(1))
	second := delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(2))

	if first != (dtos.EmitResult{Accepted: 2}) || second != (dtos.EmitResult{Accepted: 1, Dropped: 1, Reason: services.ReasonQueueFullNewest}) {
		t.Fatalf("results %+v and %+v", first, second)
	}
	if created != 2 {
		t.Fatalf("%d queues for two destinations", created)
	}
}

func Test_a_kind_without_destination_is_dropped_and_each_kind_has_its_own_queues(t *testing.T) {
	log_sink, event_sink := &fakes.FakeLogSink{}, &fakes.FakeEventSink{}
	delivery := new_delivery(t, services.Destinations{Log: []outbound.LogSinkPort{log_sink}, Event: []outbound.EventSinkPort{event_sink}}, settings_of(10, 1, 5))
	delivery.Start()

	if result := delivery.Enqueue(context.Background(), services.DestinationKindMetrics, numbered(1)); result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonNoDestination}) {
		t.Fatalf("result %+v", result)
	}
	delivery.Enqueue(context.Background(), services.DestinationKindEvent, numbered(2))

	waiting.Eventually(t, waiting.DefaultTimeout, "the event sink receives its event", func() bool { return event_sink.Calls() == 1 })
	waiting.StaysTrue(t, 30*time.Millisecond, "the log sink receives nothing", func() bool { return log_sink.Calls() == 0 })
	stats := delivery.Stats()
	if len(stats) != 2 || stats[0].Kind != services.DestinationKindLog || stats[1].Kind != services.DestinationKindEvent {
		t.Fatalf("stats %+v", stats)
	}
}

func Test_the_stats_count_what_the_sink_answers(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	sink.SetResult(dtos.SendResult{Accepted: 1, Dropped: 1, Failed: 1, Reason: "status_503"})
	delivery := new_delivery(t, log_destinations(sink), settings_of(10, 3, 10000))
	delivery.Start()

	for id := 1; id <= 3; id++ {
		delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id))
	}

	waiting.Eventually(t, waiting.DefaultTimeout, "the batch is counted", func() bool { return delivery.Stats()[0].Batches == 1 })
	stats := delivery.Stats()[0]
	if stats.Accepted != 3 || stats.Delivered != 1 || stats.Dropped != 1 || stats.Failed != 1 || stats.LastReason != "status_503" || stats.Queued != 0 || stats.DroppedByReason["status_503"] != 1 || len(stats.DroppedByReason) != 1 {
		t.Fatalf("stats %+v", stats)
	}
}

func Test_the_decision_to_send_uses_the_clock_that_was_given(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	fake_clock := fakes.NewFakeClock(value_objects.UTCInstant{})
	fake_clock.SetStep(0) // the time of the delivery does not move
	delivery := services.NewDeliveryService(log_destinations(sink), settings_of(10, 5, 20), fake_clock, fakes.NewFakeRandom(), nil, dependencies.NewMemoryQueue)
	t.Cleanup(delivery.Stop)
	delivery.Start()

	delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(1))

	waiting.StaysTrue(t, 100*time.Millisecond, "the clock did not move, so the batch waits", func() bool { return sink.Calls() == 0 })
	fake_clock.SetStep(1_000_000_000)
	delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(2))
	waiting.Eventually(t, waiting.DefaultTimeout, "the clock moved, so the batch leaves", func() bool { return len(received_ids(sink)) == 2 })
}
