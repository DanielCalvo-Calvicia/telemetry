package services_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
)

// The three event types of these tests are all routed to the log destinations (the first segment is log). By default
// log.entry.recorded is bulk at DEBUG and normal at INFO; log.other.thing follows the same rule.
const (
	entry_type = "log.entry.recorded"
	other_type = "log.other.thing"
	alert_type = "log.alert.raised"
)

func typed(id int, event_type string, severity value_objects.Severity) value_objects.Envelope {
	return value_objects.Envelope{EventId: strconv.Itoa(id), EventType: event_type, Severity: severity}
}

func normal_event(id int) value_objects.Envelope {
	return typed(id, other_type, value_objects.SeverityInfo)
}

func bulk_event(id int) value_objects.Envelope {
	return typed(id, other_type, value_objects.SeverityDebug)
}

func alert_event(id int) value_objects.Envelope {
	return typed(id, alert_type, value_objects.SeverityInfo)
}

// policy_settings is a queue of queue_size events, a batch as big as the queue and an interval so long that nothing
// leaves by itself.
func policy_settings(queue_size int, policy value_objects.QueuePolicy, block_timeout_ms int, classes map[string]value_objects.EventClass) services.DeliverySettings {
	settings := settings_of(queue_size, queue_size, 600000)
	settings.QueuePolicy = policy
	settings.BlockTimeoutMS = block_timeout_ms
	settings.EventClasses = classes
	return settings
}

func enqueue(delivery *services.DeliveryService, envelope value_objects.Envelope) dtos.EmitResult {
	return delivery.Enqueue(context.Background(), services.DestinationKindLog, envelope)
}

// delivered_ids starts the delivery and returns the ids of the first batch that the sink receives.
func delivered_ids(t *testing.T, delivery *services.DeliveryService, sink *fakes.FakeLogSink) []string {
	t.Helper()
	delivery.Start()
	waiting.Eventually(t, waiting.DefaultTimeout, "the queue is sent", func() bool { return sink.Calls() >= 1 })
	return received_ids(sink)
}

func require_strings(t *testing.T, got []string, expected ...string) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("got %v, want %v", got, expected)
	}
	for index := range expected {
		if got[index] != expected[index] {
			t.Fatalf("got %v, want %v", got, expected)
		}
	}
}

func Test_drop_newest_drops_the_new_event_and_counts_it_per_reason(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	delivery := new_delivery(t, log_destinations(sink), policy_settings(3, value_objects.QueuePolicyDropNewest, 0, nil))
	for id := 1; id <= 3; id++ {
		enqueue(delivery, normal_event(id))
	}

	first := enqueue(delivery, normal_event(4))
	second := enqueue(delivery, bulk_event(5))

	want := dtos.EmitResult{Dropped: 1, Reason: services.ReasonQueueFullNewest}
	if first != want || second != want {
		t.Fatalf("results %+v and %+v", first, second)
	}
	stats := delivery.Stats()[0]
	if stats.Accepted != 3 || stats.Dropped != 2 || stats.Queued != 3 || stats.DroppedByReason[services.ReasonQueueFullNewest] != 2 || len(stats.DroppedByReason) != 1 {
		t.Fatalf("stats %+v", stats)
	}
	require_strings(t, delivered_ids(t, delivery, sink), "1", "2", "3")
}

func Test_an_empty_policy_is_drop_newest(t *testing.T) {
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(1, "", 0, nil))
	enqueue(delivery, normal_event(1))
	if result := enqueue(delivery, normal_event(2)); result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonQueueFullNewest}) {
		t.Fatalf("result %+v", result)
	}
}

func Test_drop_oldest_takes_the_place_of_the_oldest_and_the_result_stays_accepted(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	delivery := new_delivery(t, log_destinations(sink), policy_settings(3, value_objects.QueuePolicyDropOldest, 0, nil))
	for id := 1; id <= 3; id++ {
		enqueue(delivery, normal_event(id))
	}

	first := enqueue(delivery, normal_event(4))
	second := enqueue(delivery, normal_event(5))

	if first != (dtos.EmitResult{Accepted: 1}) || second != (dtos.EmitResult{Accepted: 1}) {
		t.Fatalf("results %+v and %+v", first, second)
	}
	stats := delivery.Stats()[0]
	if stats.Accepted != 5 || stats.Dropped != 2 || stats.Queued != 3 || stats.LastReason != services.ReasonQueueFullOldest ||
		stats.DroppedByReason[services.ReasonQueueFullOldest] != 2 {
		t.Fatalf("stats %+v", stats)
	}
	require_strings(t, delivered_ids(t, delivery, sink), "3", "4", "5")
}

func Test_drop_oldest_drops_the_new_event_when_the_queue_holds_only_a_higher_class(t *testing.T) {
	classes := map[string]value_objects.EventClass{alert_type: value_objects.EventClassCritical}
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(2, value_objects.QueuePolicyDropOldest, 0, classes))
	enqueue(delivery, alert_event(1))
	enqueue(delivery, alert_event(2))

	result := enqueue(delivery, bulk_event(3))

	if result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonQueueFullNewest}) {
		t.Fatalf("result %+v", result)
	}
	if stats := delivery.Stats()[0]; stats.DroppedByReason[services.ReasonQueueFullNewest] != 1 || stats.Queued != 2 {
		t.Fatalf("stats %+v", stats)
	}
}

func Test_a_critical_event_displaces_a_bulk_one_and_counts_displaced_by_critical(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	classes := map[string]value_objects.EventClass{alert_type: value_objects.EventClassCritical}
	delivery := new_delivery(t, log_destinations(sink), policy_settings(3, value_objects.QueuePolicyDropNewest, 0, classes))
	enqueue(delivery, normal_event(1))
	enqueue(delivery, bulk_event(2))
	enqueue(delivery, normal_event(3))

	result := enqueue(delivery, alert_event(4))

	if result != (dtos.EmitResult{Accepted: 1}) {
		t.Fatalf("the result of the call changed because of the displacement: %+v", result)
	}
	stats := delivery.Stats()[0]
	if stats.Accepted != 4 || stats.Dropped != 1 || stats.Queued != 3 || stats.LastReason != services.ReasonDisplacedByCritical ||
		stats.DroppedByReason[services.ReasonDisplacedByCritical] != 1 {
		t.Fatalf("stats %+v", stats)
	}
	require_strings(t, delivered_ids(t, delivery, sink), "1", "3", "4")
}

func Test_a_critical_event_with_nothing_lower_follows_the_policy(t *testing.T) {
	classes := map[string]value_objects.EventClass{alert_type: value_objects.EventClassCritical}
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(2, value_objects.QueuePolicyDropNewest, 0, classes))
	enqueue(delivery, alert_event(1))
	enqueue(delivery, alert_event(2))

	if result := enqueue(delivery, alert_event(3)); result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonQueueFullNewest}) {
		t.Fatalf("result %+v", result)
	}
}

func Test_changing_the_class_of_an_event_type_changes_where_it_is_dropped(t *testing.T) {
	// The queue holds three normal events of another type. A new log.entry.recorded at INFO is normal by default, so it
	// is dropped; configured as critical it displaces one of them.
	scenarios := []struct {
		name            string
		classes         map[string]value_objects.EventClass
		want_result     dtos.EmitResult
		want_reason     string
		want_queue      []string
		want_dropped_by int
	}{
		{"default class", nil, dtos.EmitResult{Dropped: 1, Reason: services.ReasonQueueFullNewest}, services.ReasonQueueFullNewest, []string{"1", "2", "3"}, 1},
		{"an empty map", map[string]value_objects.EventClass{}, dtos.EmitResult{Dropped: 1, Reason: services.ReasonQueueFullNewest}, services.ReasonQueueFullNewest, []string{"1", "2", "3"}, 1},
		{
			"configured critical", map[string]value_objects.EventClass{entry_type: value_objects.EventClassCritical},
			dtos.EmitResult{Accepted: 1}, services.ReasonDisplacedByCritical, []string{"2", "3", "100"}, 1,
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			sink := &fakes.FakeLogSink{}
			delivery := new_delivery(t, log_destinations(sink), policy_settings(3, value_objects.QueuePolicyDropNewest, 0, scenario.classes))
			for id := 1; id <= 3; id++ {
				enqueue(delivery, normal_event(id))
			}

			result := enqueue(delivery, typed(100, entry_type, value_objects.SeverityInfo))

			if result != scenario.want_result {
				t.Fatalf("result %+v", result)
			}
			if stats := delivery.Stats()[0]; stats.DroppedByReason[scenario.want_reason] != scenario.want_dropped_by {
				t.Fatalf("stats %+v", stats)
			}
			require_strings(t, delivered_ids(t, delivery, sink), scenario.want_queue...)
		})
	}
}

func Test_the_severity_of_a_log_event_gives_the_default_class(t *testing.T) {
	// A debug log is bulk, so a new critical event displaces it; an info log is normal and is displaced only when no
	// bulk event is queued. The OLDEST bulk goes first.
	sink := &fakes.FakeLogSink{}
	classes := map[string]value_objects.EventClass{alert_type: value_objects.EventClassCritical}
	delivery := new_delivery(t, log_destinations(sink), policy_settings(3, value_objects.QueuePolicyDropNewest, 0, classes))
	enqueue(delivery, typed(1, entry_type, value_objects.SeverityInfo))
	enqueue(delivery, typed(2, entry_type, value_objects.SeverityDebug))
	enqueue(delivery, typed(3, entry_type, value_objects.SeverityTrace))

	enqueue(delivery, alert_event(4))
	enqueue(delivery, alert_event(5))

	require_strings(t, delivered_ids(t, delivery, sink), "1", "4", "5")
}

func Test_block_with_timeout_blocks_the_caller_for_about_the_timeout_when_no_room_appears(t *testing.T) {
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(2, value_objects.QueuePolicyBlockWithTimeout, 150, nil))
	enqueue(delivery, normal_event(1))
	enqueue(delivery, normal_event(2))

	started_at := time.Now()
	result := enqueue(delivery, normal_event(3)) // the delivery is not started, so no room appears
	elapsed := time.Since(started_at)

	if result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonBlockTimeout}) {
		t.Fatalf("result %+v", result)
	}
	if elapsed < 130*time.Millisecond || elapsed > 3*time.Second {
		t.Fatalf("the call waited %v for a timeout of 150 ms", elapsed)
	}
	stats := delivery.Stats()[0]
	if stats.Accepted != 2 || stats.Dropped != 1 || stats.Queued != 2 || stats.LastReason != services.ReasonBlockTimeout || stats.DroppedByReason[services.ReasonBlockTimeout] != 1 {
		t.Fatalf("stats %+v", stats)
	}
}

func Test_block_with_timeout_does_not_wait_when_there_is_room_or_a_critical_event_can_displace(t *testing.T) {
	classes := map[string]value_objects.EventClass{alert_type: value_objects.EventClassCritical}
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(2, value_objects.QueuePolicyBlockWithTimeout, 60000, classes))

	started_at := time.Now()
	first := enqueue(delivery, normal_event(1))
	second := enqueue(delivery, bulk_event(2))
	displacing := enqueue(delivery, alert_event(3))

	if first != (dtos.EmitResult{Accepted: 1}) || second != (dtos.EmitResult{Accepted: 1}) || displacing != (dtos.EmitResult{Accepted: 1}) {
		t.Fatalf("results %+v %+v %+v", first, second, displacing)
	}
	if time.Since(started_at) > time.Second {
		t.Fatal("a call waited although it did not have to")
	}
	if stats := delivery.Stats()[0]; stats.DroppedByReason[services.ReasonDisplacedByCritical] != 1 {
		t.Fatalf("stats %+v", stats)
	}
}

func Test_block_with_timeout_returns_at_once_when_the_worker_frees_room(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	delivery := new_delivery(t, log_destinations(sink), policy_settings(2, value_objects.QueuePolicyBlockWithTimeout, 30000, nil))
	enqueue(delivery, normal_event(1))
	enqueue(delivery, normal_event(2))

	results := make(chan dtos.EmitResult, 1)
	finished_after := make(chan time.Duration, 1)
	go func() {
		started_at := time.Now()
		results <- enqueue(delivery, normal_event(3))
		finished_after <- time.Since(started_at)
	}()
	select {
	case <-results:
		t.Fatal("the call did not wait although the queue is full and no worker runs")
	case <-time.After(100 * time.Millisecond):
	}

	delivery.Start() // the worker takes the full batch of two events and frees room

	select {
	case result := <-results:
		if result != (dtos.EmitResult{Accepted: 1}) {
			t.Fatalf("result %+v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the waiting call was not woken when room appeared")
	}
	if elapsed := <-finished_after; elapsed > 5*time.Second {
		t.Fatalf("the call took %v, the timeout is 30 s", elapsed)
	}
	if stats := delivery.Stats()[0]; stats.Accepted != 3 || stats.Dropped != 0 || len(stats.DroppedByReason) != 0 {
		t.Fatalf("stats %+v", stats)
	}
}

func Test_several_callers_that_wait_are_all_woken_when_the_worker_frees_room(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	delivery := new_delivery(t, log_destinations(sink), policy_settings(3, value_objects.QueuePolicyBlockWithTimeout, 30000, nil))
	for id := 1; id <= 3; id++ {
		enqueue(delivery, normal_event(id))
	}

	var group sync.WaitGroup
	var results_mutex sync.Mutex
	accepted := 0
	for id := 4; id <= 6; id++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result := enqueue(delivery, normal_event(id))
			results_mutex.Lock()
			accepted += result.Accepted
			results_mutex.Unlock()
		}()
	}
	waiting.StaysTrue(t, 100*time.Millisecond, "nothing is admitted while the queue is full", func() bool { return delivery.Stats()[0].Accepted == 3 })

	delivery.Start()
	done := make(chan struct{})
	go func() {
		group.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a waiting caller was not woken")
	}
	if accepted != 3 {
		t.Fatalf("%d of the three waiting events were accepted", accepted)
	}
}

func Test_stop_wakes_a_caller_that_waits_for_room(t *testing.T) {
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(1, value_objects.QueuePolicyBlockWithTimeout, 60000, nil))
	enqueue(delivery, normal_event(1))
	results := make(chan dtos.EmitResult, 1)
	go func() { results <- enqueue(delivery, normal_event(2)) }()
	select {
	case <-results:
		t.Fatal("the call did not wait")
	case <-time.After(100 * time.Millisecond):
	}

	delivery.Stop()

	select {
	case result := <-results:
		if result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonBlockTimeout}) {
			t.Fatalf("result %+v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not wake the waiting call")
	}
	// A call after Stop that finds no room does not wait either.
	started_at := time.Now()
	if result := enqueue(delivery, normal_event(3)); result.Dropped != 1 || time.Since(started_at) > time.Second {
		t.Fatalf("result %+v after %v", result, time.Since(started_at))
	}
}

func Test_a_cancelled_context_ends_the_wait_for_room(t *testing.T) {
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(1, value_objects.QueuePolicyBlockWithTimeout, 60000, nil))
	enqueue(delivery, normal_event(1))
	call_context, cancel_call := context.WithCancel(context.Background())
	results := make(chan dtos.EmitResult, 1)
	go func() { results <- delivery.Enqueue(call_context, services.DestinationKindLog, normal_event(2)) }()
	select {
	case <-results:
		t.Fatal("the call did not wait")
	case <-time.After(100 * time.Millisecond):
	}

	cancel_call()

	select {
	case result := <-results:
		if result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonBlockTimeout}) {
			t.Fatalf("result %+v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cancelled context did not end the wait")
	}
}

func Test_block_with_timeout_and_a_timeout_of_zero_drops_at_once(t *testing.T) {
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(1, value_objects.QueuePolicyBlockWithTimeout, 0, nil))
	enqueue(delivery, normal_event(1))

	started_at := time.Now()
	result := enqueue(delivery, normal_event(2))

	if result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonBlockTimeout}) || time.Since(started_at) > time.Second {
		t.Fatalf("result %+v after %v", result, time.Since(started_at))
	}
}

func Test_every_destination_of_the_kind_counts_its_own_drops(t *testing.T) {
	first_sink, second_sink := &fakes.FakeLogSink{}, &fakes.FakeLogSink{}
	delivery := new_delivery(t, log_destinations(first_sink, second_sink), policy_settings(1, value_objects.QueuePolicyDropNewest, 0, nil))
	enqueue(delivery, normal_event(1))

	result := enqueue(delivery, normal_event(2))

	if result != (dtos.EmitResult{Dropped: 2, Reason: services.ReasonQueueFullNewest}) {
		t.Fatalf("result %+v", result)
	}
	for _, stats := range delivery.Stats() {
		if stats.DroppedByReason[services.ReasonQueueFullNewest] != 1 {
			t.Fatalf("stats %+v", stats)
		}
	}
}

func Test_the_stats_hand_out_a_copy_of_the_counters_per_reason(t *testing.T) {
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(1, value_objects.QueuePolicyDropNewest, 0, nil))
	enqueue(delivery, normal_event(1))
	enqueue(delivery, normal_event(2))

	snapshot := delivery.Stats()[0]
	snapshot.DroppedByReason[services.ReasonQueueFullNewest] = 99
	snapshot.DroppedByReason["invented"] = 1

	if again := delivery.Stats()[0]; again.DroppedByReason[services.ReasonQueueFullNewest] != 1 || len(again.DroppedByReason) != 1 {
		t.Fatalf("the stats share the map: %+v", again)
	}
}

func Test_no_drop_panics_with_any_policy_and_every_class(t *testing.T) {
	classes := map[string]value_objects.EventClass{alert_type: value_objects.EventClassCritical, entry_type: "odd"}
	policies := []value_objects.QueuePolicy{value_objects.QueuePolicyDropNewest, value_objects.QueuePolicyDropOldest, value_objects.QueuePolicyBlockWithTimeout, "unknown"}
	for _, policy := range policies {
		delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(1, policy, 1, classes))
		for id := 0; id < 30; id++ {
			enqueue(delivery, []value_objects.Envelope{normal_event(id), bulk_event(id), alert_event(id), typed(id, entry_type, value_objects.SeverityError)}[id%4])
		}
		stats := delivery.Stats()[0]
		if stats.Queued != 1 || stats.Accepted+stats.Dropped < 30 {
			t.Fatalf("%s: stats %+v", policy, stats)
		}
	}
}

func Test_concurrent_enqueues_to_a_full_queue_count_every_event_once(t *testing.T) {
	const callers, calls_each, queue_size = 8, 300, 10
	classes := map[string]value_objects.EventClass{alert_type: value_objects.EventClassCritical}
	delivery := new_delivery(t, log_destinations(&fakes.FakeLogSink{}), policy_settings(queue_size, value_objects.QueuePolicyDropOldest, 0, classes))
	var group sync.WaitGroup
	for caller := 0; caller < callers; caller++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := 0; index < calls_each; index++ {
				envelopes := []value_objects.Envelope{normal_event(index), bulk_event(index), alert_event(index)}
				enqueue(delivery, envelopes[(caller+index)%3])
			}
		}()
	}
	group.Wait()

	stats := delivery.Stats()[0]
	dropped_by_reason := 0
	for _, count := range stats.DroppedByReason {
		dropped_by_reason += count
	}
	if stats.Queued != queue_size || stats.Accepted-stats.Dropped+stats.DroppedByReason[services.ReasonQueueFullNewest] != queue_size {
		t.Fatalf("stats %+v", stats)
	}
	if dropped_by_reason != stats.Dropped || stats.Accepted+stats.DroppedByReason[services.ReasonQueueFullNewest] != callers*calls_each {
		t.Fatalf("stats %+v", stats)
	}
}

func Test_a_drop_reported_by_a_sink_is_counted_under_its_reason_or_unspecified(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	sink.SetResult(dtos.SendResult{Accepted: 1, Dropped: 2})
	delivery := new_delivery(t, log_destinations(sink), policy_settings(3, value_objects.QueuePolicyDropNewest, 0, nil))
	for id := 1; id <= 3; id++ {
		enqueue(delivery, normal_event(id))
	}
	delivery.Start()
	waiting.Eventually(t, waiting.DefaultTimeout, "the batch is counted", func() bool { return delivery.Stats()[0].Batches == 1 })

	stats := delivery.Stats()[0]
	if stats.Dropped != 2 || stats.DroppedByReason[services.ReasonUnspecified] != 2 || stats.LastReason != services.ReasonUnspecified || len(stats.DroppedByReason) != 1 {
		t.Fatalf("stats %+v", stats)
	}

	sink.SetResult(dtos.SendResult{Failed: 1, Reason: "status_503"})
	for id := 4; id <= 6; id++ {
		enqueue(delivery, normal_event(id))
	}
	waiting.Eventually(t, waiting.DefaultTimeout, "the second batch is counted", func() bool { return delivery.Stats()[0].Batches == 2 })
	stats = delivery.Stats()[0]
	if stats.Failed != 1 || stats.Dropped != 2 || stats.LastReason != "status_503" || stats.DroppedByReason[services.ReasonUnspecified] != 2 {
		t.Fatalf("a failure must set the last reason and count no drop: %+v", stats)
	}
}
