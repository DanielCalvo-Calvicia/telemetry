package services_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/dependencies"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
)

// mode_scenarios_file is the case file that the Python tests read too (the same file).
const mode_scenarios_file = "../../../../../integration-tests/fixtures/delivery/mode_scenarios.json"

type mode_answer struct {
	Accepted     int    `json:"accepted"`
	Dropped      int    `json:"dropped"`
	Failed       int    `json:"failed"`
	Reason       string `json:"reason"`
	RetryAfterMS int64  `json:"retry_after_ms"`
}

type mode_scenario struct {
	Name             string        `json:"name"`
	Mode             string        `json:"mode"`
	RetryMaxAttempts int           `json:"retry_max_attempts"`
	Events           int           `json:"events"`
	Answers          []mode_answer `json:"answers"`
	Expected         struct {
		Attempts        int            `json:"attempts"`
		Retries         int            `json:"retries"`
		Delivered       int            `json:"delivered"`
		Dropped         int            `json:"dropped"`
		Failed          int            `json:"failed"`
		DroppedByReason map[string]int `json:"dropped_by_reason"`
		MinElapsedMS    *int64         `json:"min_elapsed_ms"`
	} `json:"expected"`
}

// The numbers the description of the shared file gives: a batch of 3 events, a backoff from 1 ms to 5 ms and a random
// source that always answers 0.5.
func mode_settings(mode value_objects.DeliveryMode, retry_max_attempts int) services.DeliverySettings {
	return services.DeliverySettings{
		QueueSize: 100, BatchSize: 3, FlushIntervalMS: 5, TimeoutMS: 5000,
		Mode: mode, RetryMaxAttempts: retry_max_attempts, RetryBaseDelayMS: 1, RetryMaxDelayMS: 5,
	}
}

// new_mode_delivery builds a delivery service over one scripted sink and a fixed random source. It is started and stopped
// when the test ends.
func new_mode_delivery(test *testing.T, sink *fakes.ScriptedLogSink, settings services.DeliverySettings) *services.DeliveryService {
	test.Helper()
	delivery := services.NewDeliveryService(
		log_destinations(sink), settings, dependencies.NewSystemClock(), fakes.NewFakeRandom(0.5), dependencies.NewMemoryQueue)
	test.Cleanup(delivery.Stop)
	delivery.Start()
	return delivery
}

func enqueue_batch_events(test *testing.T, delivery *services.DeliveryService, count int) {
	test.Helper()
	for id := 1; id <= count; id++ {
		if result := delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id)); result.Accepted < 1 {
			test.Fatalf("event %d: %+v", id, result)
		}
	}
}

func Test_every_shared_mode_scenario_gives_the_same_attempts_and_counters(t *testing.T) {
	content, read_error := os.ReadFile(mode_scenarios_file)
	if read_error != nil {
		t.Fatal(read_error)
	}
	var file struct {
		Cases []mode_scenario `json:"cases"`
	}
	if decode_error := json.Unmarshal(content, &file); decode_error != nil {
		t.Fatal(decode_error)
	}
	if len(file.Cases) != 10 {
		t.Fatalf("expected 10 cases, got %d", len(file.Cases))
	}
	for _, scenario := range file.Cases {
		t.Run(scenario.Name, func(t *testing.T) {
			answers := make([]dtos.SendResult, 0, len(scenario.Answers))
			for _, answer := range scenario.Answers {
				answers = append(answers, dtos.SendResult{
					Accepted: answer.Accepted, Dropped: answer.Dropped, Failed: answer.Failed, Reason: answer.Reason, RetryAfterMS: answer.RetryAfterMS,
				})
			}
			sink := fakes.NewScriptedLogSink(answers...)
			delivery := new_mode_delivery(t, sink, mode_settings(value_objects.DeliveryMode(scenario.Mode), scenario.RetryMaxAttempts))

			enqueue_batch_events(t, delivery, scenario.Events)
			waiting.Eventually(t, waiting.DefaultTimeout, "the batch is finished", func() bool { return delivery.Stats()[0].Batches == 1 })
			waiting.StaysTrue(t, 30*time.Millisecond, "the destination is not called again", func() bool { return sink.Calls() == scenario.Expected.Attempts })

			stats := delivery.Stats()[0]
			if stats.Attempts != scenario.Expected.Attempts || stats.Retries != scenario.Expected.Retries {
				t.Errorf("attempts %d retries %d, expected %d and %d", stats.Attempts, stats.Retries, scenario.Expected.Attempts, scenario.Expected.Retries)
			}
			if sink.Calls() != scenario.Expected.Attempts {
				t.Errorf("the sink was called %d times, expected %d", sink.Calls(), scenario.Expected.Attempts)
			}
			if stats.Delivered != scenario.Expected.Delivered || stats.Dropped != scenario.Expected.Dropped || stats.Failed != scenario.Expected.Failed {
				t.Errorf("delivered %d dropped %d failed %d, expected %d %d %d", stats.Delivered, stats.Dropped, stats.Failed,
					scenario.Expected.Delivered, scenario.Expected.Dropped, scenario.Expected.Failed)
			}
			if !reflect.DeepEqual(stats.DroppedByReason, nonnil_counts(scenario.Expected.DroppedByReason)) {
				t.Errorf("dropped by reason %v, expected %v", stats.DroppedByReason, scenario.Expected.DroppedByReason)
			}
			if scenario.Expected.MinElapsedMS != nil && sink.Elapsed() < time.Duration(*scenario.Expected.MinElapsedMS)*time.Millisecond {
				t.Errorf("the calls were %v apart, expected at least %d ms", sink.Elapsed(), *scenario.Expected.MinElapsedMS)
			}
			for _, batch := range sink.Batches() {
				if len(batch) != scenario.Events {
					t.Errorf("a call carried %d events, expected the whole batch of %d", len(batch), scenario.Events)
				}
			}
		})
	}
}

func nonnil_counts(counts map[string]int) map[string]int {
	if counts == nil {
		return map[string]int{}
	}
	return counts
}

func Test_a_partial_answer_at_the_limit_delivers_what_it_accepted_and_drops_the_rest_under_the_limit_reason(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Accepted: 1, Failed: 2, Reason: "status_503"})
	delivery := new_mode_delivery(t, sink, mode_settings(value_objects.DeliveryModeRetry, 2))

	enqueue_batch_events(t, delivery, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the batch is finished", func() bool { return delivery.Stats()[0].Batches == 1 })

	stats := delivery.Stats()[0]
	if stats.Attempts != 2 || stats.Retries != 1 || stats.Delivered != 1 || stats.Dropped != 2 || stats.Failed != 0 ||
		stats.DroppedByReason[services.ReasonRetryLimitReached] != 2 || stats.LastReason != services.ReasonRetryLimitReached {
		t.Fatalf("%+v", stats)
	}
}

func Test_a_mixed_answer_with_dropped_events_is_final_and_keeps_its_failed_events_as_failed(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Accepted: 1, Dropped: 1, Failed: 1, Reason: "status_400"})
	delivery := new_mode_delivery(t, sink, mode_settings(value_objects.DeliveryModeRetry, 5))

	enqueue_batch_events(t, delivery, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the batch is finished", func() bool { return delivery.Stats()[0].Batches == 1 })

	stats := delivery.Stats()[0]
	if stats.Attempts != 1 || stats.Delivered != 1 || stats.Dropped != 1 || stats.Failed != 1 || stats.DroppedByReason["status_400"] != 1 {
		t.Fatalf("%+v", stats)
	}
}

func Test_an_empty_mode_is_best_effort_and_the_retry_settings_are_not_used(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 3, Reason: "status_503", RetryAfterMS: 50})
	delivery := new_mode_delivery(t, sink, mode_settings("", 5))

	enqueue_batch_events(t, delivery, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the batch is finished", func() bool { return delivery.Stats()[0].Batches == 1 })
	waiting.StaysTrue(t, 100*time.Millisecond, "a failed batch is not sent again", func() bool { return sink.Calls() == 1 })

	stats := delivery.Stats()[0]
	if stats.Failed != 3 || stats.Delivered != 0 || stats.Dropped != 0 || stats.Retries != 0 || stats.LastReason != "status_503" {
		t.Fatalf("%+v", stats)
	}
}

func Test_a_service_built_by_hand_with_no_retry_settings_uses_the_defaults_and_no_random_source(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 3, Reason: "status_503"}, dtos.SendResult{Accepted: 3})
	settings := services.DeliverySettings{QueueSize: 100, BatchSize: 3, FlushIntervalMS: 5, TimeoutMS: 5000, Mode: value_objects.DeliveryModeRetry}
	delivery := services.NewDeliveryService(log_destinations(sink), settings, dependencies.NewSystemClock(), nil, dependencies.NewMemoryQueue)
	t.Cleanup(delivery.Stop)
	delivery.Start()

	enqueue_batch_events(t, delivery, 3)
	// The default base is 200 ms, so the first retry comes between 100 and 200 ms after the first call.
	waiting.Eventually(t, waiting.DefaultTimeout, "the batch is delivered at the second attempt", func() bool { return delivery.Stats()[0].Delivered == 3 })

	if stats := delivery.Stats()[0]; stats.Attempts != 2 || stats.Retries != 1 {
		t.Fatalf("%+v", stats)
	}
	if sink.Elapsed() < 90*time.Millisecond {
		t.Fatalf("the retry came after %v, expected at least the half of the default base", sink.Elapsed())
	}
}

func Test_a_sink_that_panics_is_retried_and_never_panics_the_service(t *testing.T) {
	sink := fakes.NewScriptedLogSink()
	sink.PanicOnCalls(0, 1)
	delivery := new_mode_delivery(t, sink, mode_settings(value_objects.DeliveryModeRetry, 5))

	enqueue_batch_events(t, delivery, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the batch is delivered at the third attempt", func() bool { return delivery.Stats()[0].Delivered == 3 })

	if stats := delivery.Stats()[0]; stats.Attempts != 3 || stats.Retries != 2 || stats.Failed != 0 || stats.Dropped != 0 {
		t.Fatalf("%+v", stats)
	}

	always_panics := fakes.NewScriptedLogSink()
	always_panics.PanicOnCalls(0, 1, 2)
	second := new_mode_delivery(t, always_panics, mode_settings(value_objects.DeliveryModeRetry, 3))
	enqueue_batch_events(t, second, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the batch gives up", func() bool { return second.Stats()[0].Batches == 1 })
	if stats := second.Stats()[0]; stats.Attempts != 3 || stats.DroppedByReason[services.ReasonRetryLimitReached] != 3 || stats.Delivered != 0 {
		t.Fatalf("%+v", stats)
	}
}

func Test_a_shutdown_during_a_backoff_wait_returns_within_its_timeout_and_counts_the_batch_as_dropped(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 3, Reason: "status_503", RetryAfterMS: 60_000})
	delivery := new_mode_delivery(t, sink, mode_settings(value_objects.DeliveryModeRetry, 5))
	enqueue_batch_events(t, delivery, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the first attempt is made", func() bool { return delivery.Stats()[0].Attempts == 1 })

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	started_at := time.Now()
	result := delivery.Shutdown(ctx)
	elapsed := time.Since(started_at)

	if elapsed > 3*time.Second {
		t.Fatalf("Shutdown took %v with a wait of 60 s pending", elapsed)
	}
	if result.Sent != 0 || result.Dropped != 3 {
		t.Fatalf("result %+v", result)
	}
	waiting.Eventually(t, waiting.DefaultTimeout, "the batch is counted as dropped under shutdown", func() bool {
		return delivery.Stats()[0].DroppedByReason[services.ReasonShutdown] == 3
	})
	stats := delivery.Stats()[0]
	if stats.Dropped != 3 || stats.Delivered != 0 || stats.Failed != 0 || stats.Attempts != 1 || stats.Batches != 1 {
		t.Fatalf("%+v", stats)
	}
	if sink.Calls() != 1 {
		t.Fatalf("the destination was called %d times", sink.Calls())
	}
}

func Test_a_stop_during_a_backoff_wait_abandons_the_batch(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 3, Reason: "status_503", RetryAfterMS: 60_000})
	delivery := new_mode_delivery(t, sink, mode_settings(value_objects.DeliveryModeRetry, 5))
	enqueue_batch_events(t, delivery, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the first attempt is made", func() bool { return delivery.Stats()[0].Attempts == 1 })

	stopped := make(chan struct{})
	go func() {
		delivery.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not wake the wait")
	}
	if stats := delivery.Stats()[0]; stats.DroppedByReason[services.ReasonShutdown] != 3 || stats.Attempts != 1 {
		t.Fatalf("%+v", stats)
	}
}

func Test_a_flush_waits_for_a_batch_that_is_waiting_to_be_sent_again(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 3, Reason: "status_503", RetryAfterMS: 150}, dtos.SendResult{Accepted: 3})
	delivery := new_mode_delivery(t, sink, mode_settings(value_objects.DeliveryModeRetry, 5))
	enqueue_batch_events(t, delivery, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the first attempt is made", func() bool { return delivery.Stats()[0].Attempts == 1 })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := delivery.Flush(ctx)

	if result.Sent != 3 || result.Dropped != 0 {
		t.Fatalf("result %+v", result)
	}
	if stats := delivery.Stats()[0]; stats.Delivered != 3 || stats.Attempts != 2 {
		t.Fatalf("%+v", stats)
	}
}

func Test_a_flush_with_a_short_time_counts_a_batch_that_is_still_retrying_as_dropped(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 3, Reason: "status_503", RetryAfterMS: 60_000})
	delivery := new_mode_delivery(t, sink, mode_settings(value_objects.DeliveryModeRetry, 5))
	enqueue_batch_events(t, delivery, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the first attempt is made", func() bool { return delivery.Stats()[0].Attempts == 1 })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started_at := time.Now()
	result := delivery.Flush(ctx)

	if time.Since(started_at) > 3*time.Second || result.Sent != 0 || result.Dropped != 3 {
		t.Fatalf("result %+v after %v", result, time.Since(started_at))
	}
}

func Test_a_destination_that_retries_never_delays_the_healthy_one(t *testing.T) {
	failing_sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 1, Reason: "status_503", RetryAfterMS: 60_000})
	healthy_sink := fakes.NewScriptedLogSink()
	settings := mode_settings(value_objects.DeliveryModeRetry, 5)
	settings.BatchSize = 1
	delivery := services.NewDeliveryService(
		log_destinations(failing_sink, healthy_sink), settings, dependencies.NewSystemClock(), fakes.NewFakeRandom(0.5), dependencies.NewMemoryQueue)
	t.Cleanup(delivery.Stop)
	delivery.Start()

	enqueue_batch_events(t, delivery, 5)

	waiting.Eventually(t, 2*time.Second, "the healthy destination delivers the five events while the other one waits", func() bool {
		return delivery.Stats()[1].Delivered == 5
	})
	stats := delivery.Stats()
	if stats[0].Attempts != 1 || stats[0].Delivered != 0 || stats[0].Queued != 4 {
		t.Fatalf("the failing destination: %+v", stats[0])
	}
	if stats[1].Retries != 0 || stats[1].Failed != 0 || stats[1].Dropped != 0 {
		t.Fatalf("the healthy destination: %+v", stats[1])
	}
}

func Test_new_events_keep_entering_the_queue_while_a_batch_is_retried_under_the_queue_policy(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 1, Reason: "status_503", RetryAfterMS: 60_000})
	settings := mode_settings(value_objects.DeliveryModeRetry, 5)
	settings.QueueSize = 3
	settings.BatchSize = 1
	delivery := new_mode_delivery(t, sink, settings)

	enqueue_batch_events(t, delivery, 1)
	waiting.Eventually(t, waiting.DefaultTimeout, "the first batch is taken and tried", func() bool { return delivery.Stats()[0].Attempts == 1 })

	for id := 2; id <= 4; id++ {
		if result := delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(id)); result != (dtos.EmitResult{Accepted: 1}) {
			t.Fatalf("event %d entered the queue while a batch was retried: %+v", id, result)
		}
	}
	// The queue holds 3 events now; the policy drop_newest (the default) applies to the next one, at once.
	started_at := time.Now()
	result := delivery.Enqueue(context.Background(), services.DestinationKindLog, numbered(5))
	if result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonQueueFullNewest}) || time.Since(started_at) > time.Second {
		t.Fatalf("the full queue: %+v after %v", result, time.Since(started_at))
	}
	if stats := delivery.Stats()[0]; stats.Queued != 3 || stats.DroppedByReason[services.ReasonQueueFullNewest] != 1 {
		t.Fatalf("%+v", stats)
	}
}

func Test_the_wait_between_attempts_uses_the_backoff_of_the_domain_with_the_random_source(t *testing.T) {
	// Base 40 ms, ceiling 40 ms and a random of 0: every wait is 20 ms, so 3 attempts are at least 40 ms apart in all.
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 3, Reason: "status_503"})
	settings := mode_settings(value_objects.DeliveryModeRetry, 3)
	settings.RetryBaseDelayMS, settings.RetryMaxDelayMS = 40, 40
	fake_random := fakes.NewFakeRandom(0)
	delivery := services.NewDeliveryService(log_destinations(sink), settings, dependencies.NewSystemClock(), fake_random, dependencies.NewMemoryQueue)
	t.Cleanup(delivery.Stop)
	delivery.Start()

	enqueue_batch_events(t, delivery, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the batch gives up", func() bool { return delivery.Stats()[0].Batches == 1 })

	if sink.Elapsed() < 40*time.Millisecond {
		t.Fatalf("the waits added up to %v, expected at least 40 ms", sink.Elapsed())
	}
	if fake_random.Calls() != 2 {
		t.Fatalf("the random source was asked %d times, expected once for each of the 2 waits", fake_random.Calls())
	}
}
