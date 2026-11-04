package services_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/dependencies"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
)

// diagnostics_scenarios_file is the case file that the Python tests read too (the same file).
const diagnostics_scenarios_file = "../../../../../integration-tests/fixtures/delivery/diagnostics_snapshot.json"

type expected_counters struct {
	Attempts              int            `json:"attempts"`
	Retries               int            `json:"retries"`
	Batches               int            `json:"batches"`
	Delivered             int            `json:"delivered"`
	Dropped               int            `json:"dropped"`
	FailedEvents          int            `json:"failed_events"`
	ExportFailures        int            `json:"export_failures"`
	SerializationFailures int            `json:"serialization_failures"`
	ConfigurationErrors   int            `json:"configuration_errors"`
	LastBatchSize         int            `json:"last_batch_size"`
	MaxBatchSize          int            `json:"max_batch_size"`
	DroppedByReason       map[string]int `json:"dropped_by_reason"`
}

type expected_diagnostic struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type diagnostics_scenario struct {
	Name             string        `json:"name"`
	Mode             string        `json:"mode"`
	RetryMaxAttempts int           `json:"retry_max_attempts"`
	Events           int           `json:"events"`
	Answers          []mode_answer `json:"answers"`
	Expected         struct {
		Counters       expected_counters `json:"counters"`
		HasLastSuccess bool              `json:"has_last_success"`
		LastFlush      struct {
			Sent    int `json:"sent"`
			Spooled int `json:"spooled"`
			Dropped int `json:"dropped"`
		} `json:"last_flush"`
		Diagnostics []expected_diagnostic `json:"diagnostics"`
	} `json:"expected"`
}

// diagnostics_collector is a callback that keeps what it receives.
type diagnostics_collector struct {
	mutex  sync.Mutex
	events []dtos.DiagnosticEvent
}

func (collector *diagnostics_collector) callback(event dtos.DiagnosticEvent) {
	collector.mutex.Lock()
	defer collector.mutex.Unlock()
	collector.events = append(collector.events, event)
}

func (collector *diagnostics_collector) snapshot() []dtos.DiagnosticEvent {
	collector.mutex.Lock()
	defer collector.mutex.Unlock()
	return slices.Clone(collector.events)
}

func (collector *diagnostics_collector) count() int { return len(collector.snapshot()) }

// text_writer is an io.Writer that keeps what it receives.
type text_writer struct {
	mutex sync.Mutex
	text  strings.Builder
}

func (writer *text_writer) Write(content []byte) (int, error) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.text.Write(content)
}

func (writer *text_writer) String() string {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.text.String()
}

// diagnostics_settings are the numbers of the shared file: a batch of 10 that leaves only on a flush, a backoff from 1 to
// 5 ms.
func diagnostics_settings(mode value_objects.DeliveryMode, retry_max_attempts int) services.DeliverySettings {
	return services.DeliverySettings{
		QueueSize: 100, BatchSize: 10, FlushIntervalMS: 60000, TimeoutMS: 5000,
		Mode: mode, RetryMaxAttempts: retry_max_attempts, RetryBaseDelayMS: 1, RetryMaxDelayMS: 5,
	}
}

// new_diagnostics_delivery builds a started delivery service over one sink, with the real diagnostics channel and the
// callbacks. It is stopped when the test ends.
func new_diagnostics_delivery(
	test *testing.T,
	sink *fakes.ScriptedLogSink,
	settings services.DeliverySettings,
	callbacks ...func(dtos.DiagnosticEvent),
) *services.DeliveryService {
	test.Helper()
	clock := dependencies.NewSystemClock()
	channel := dependencies.NewDiagnosticsChannel(clock, callbacks, nil)
	delivery := services.NewDeliveryService(log_destinations(sink), settings, clock, fakes.NewFakeRandom(0.5), channel, dependencies.NewMemoryQueue)
	delivery.Start()
	test.Cleanup(delivery.Stop)
	return delivery
}

func Test_every_shared_diagnostics_scenario_gives_the_same_counters_and_the_same_diagnostics(t *testing.T) {
	content, read_error := os.ReadFile(diagnostics_scenarios_file)
	if read_error != nil {
		t.Fatal(read_error)
	}
	var file struct {
		Cases []diagnostics_scenario `json:"cases"`
	}
	if decode_error := json.Unmarshal(content, &file); decode_error != nil {
		t.Fatal(decode_error)
	}
	if len(file.Cases) != 40 {
		t.Fatalf("expected 40 cases, got %d", len(file.Cases))
	}
	for _, scenario := range file.Cases {
		t.Run(scenario.Name, func(t *testing.T) {
			answers := make([]dtos.SendResult, 0, len(scenario.Answers))
			for _, answer := range scenario.Answers {
				answers = append(answers, dtos.SendResult{
					Accepted: answer.Accepted, Dropped: answer.Dropped, Failed: answer.Failed, Reason: answer.Reason, RetryAfterMS: answer.RetryAfterMS,
				})
			}
			collector := &diagnostics_collector{}
			sink := fakes.NewScriptedLogSink(answers...)
			delivery := new_diagnostics_delivery(t, sink, diagnostics_settings(value_objects.DeliveryMode(scenario.Mode), scenario.RetryMaxAttempts), collector.callback)
			enqueue_batch_events(t, delivery, scenario.Events)

			flush_context, cancel_flush := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel_flush()
			flush_result := delivery.Flush(flush_context)

			expected := scenario.Expected
			waiting.Eventually(t, waiting.DefaultTimeout, "the diagnostics arrived", func() bool { return collector.count() >= len(expected.Diagnostics) })
			waiting.StaysTrue(t, 20*time.Millisecond, "no other diagnostic arrives", func() bool { return collector.count() == len(expected.Diagnostics) })

			snapshot := delivery.Diagnostics()
			if len(snapshot.Destinations) != 1 {
				t.Fatalf("%d destinations", len(snapshot.Destinations))
			}
			destination := snapshot.Destinations[0]
			counters := expected_counters{
				Attempts: destination.Attempts, Retries: destination.Retries, Batches: destination.Batches, Delivered: destination.Delivered,
				Dropped: destination.Dropped, FailedEvents: destination.FailedEvents, ExportFailures: destination.ExportFailures,
				SerializationFailures: destination.SerializationFailures, ConfigurationErrors: destination.ConfigurationErrors,
				LastBatchSize: destination.LastBatchSize, MaxBatchSize: destination.MaxBatchSize, DroppedByReason: destination.DroppedByReason,
			}
			if counters.DroppedByReason == nil || expected.Counters.DroppedByReason == nil {
				t.Fatalf("a nil map: %v and %v", counters.DroppedByReason, expected.Counters.DroppedByReason)
			}
			if !reflect.DeepEqual(counters, expected.Counters) {
				t.Errorf("counters %+v, expected %+v", counters, expected.Counters)
			}
			if has_last_success := destination.LastSuccessAtMS > 0; has_last_success != expected.HasLastSuccess {
				t.Errorf("last success at %d, expected has_last_success %v", destination.LastSuccessAtMS, expected.HasLastSuccess)
			}
			last_flush := destination.LastFlush
			if !last_flush.Done || last_flush.Sent != expected.LastFlush.Sent || last_flush.Spooled != expected.LastFlush.Spooled ||
				last_flush.Dropped != expected.LastFlush.Dropped || last_flush.AtMS <= 0 {
				t.Errorf("last flush %+v, expected %+v", last_flush, expected.LastFlush)
			}
			if flush_result.Sent != expected.LastFlush.Sent || flush_result.Dropped != expected.LastFlush.Dropped {
				t.Errorf("the flush answered %+v", flush_result)
			}
			if destination.Accepted != scenario.Events || destination.QueueDepth != 0 || destination.QueueCapacity != 100 {
				t.Errorf("accepted %d, queue %d of %d", destination.Accepted, destination.QueueDepth, destination.QueueCapacity)
			}
			if destination.LastExportLatencyMS < 0 || destination.MaxExportLatencyMS < destination.LastExportLatencyMS ||
				destination.TotalExportLatencyMS < destination.MaxExportLatencyMS {
				t.Errorf("latency last %d max %d total %d", destination.LastExportLatencyMS, destination.MaxExportLatencyMS, destination.TotalExportLatencyMS)
			}
			if destination.SpoolEvents != 0 || destination.SpoolBytes != 0 || destination.SpoolQuotaBytes != 0 {
				t.Errorf("the spool figures are not zero: %+v", destination)
			}

			received := collector.snapshot()
			if len(received) != len(expected.Diagnostics) {
				t.Fatalf("%d diagnostics, expected %d: %+v", len(received), len(expected.Diagnostics), received)
			}
			for position, diagnostic := range received {
				wanted := expected.Diagnostics[position]
				if string(diagnostic.Code) != wanted.Code || diagnostic.Reason != wanted.Reason || diagnostic.Count != wanted.Count ||
					diagnostic.Suppressed != 0 || diagnostic.Kind != "log" || diagnostic.Index != 0 || diagnostic.AtMS <= 0 {
					t.Errorf("diagnostic %d is %+v, expected %+v", position, diagnostic, wanted)
				}
			}
			if channel := snapshot.Channel; channel.Published != len(expected.Diagnostics) || channel.Suppressed != 0 || channel.Dropped != 0 || channel.CallbackErrors != 0 {
				t.Errorf("channel %+v", channel)
			}
		})
	}
}

func Test_the_snapshot_is_a_copy_that_the_caller_may_change(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Accepted: 1, Dropped: 2, Reason: "status_400"})
	delivery := new_diagnostics_delivery(t, sink, diagnostics_settings(value_objects.DeliveryModeBestEffort, 5))
	enqueue_batch_events(t, delivery, 3)
	delivery.Flush(context.Background())

	first := delivery.Diagnostics()
	first.Destinations[0].DroppedByReason["status_400"] = 99
	first.Destinations[0].DroppedByReason["invented"] = 1
	first.Destinations[0].Delivered = 99
	first.Destinations = append(first.Destinations[:0], dtos.DestinationDiagnostics{})
	first.Channel.Published = 99
	second := delivery.Diagnostics()

	if len(second.Destinations) != 1 || second.Destinations[0].Delivered != 1 || second.Destinations[0].Dropped != 2 ||
		!reflect.DeepEqual(second.Destinations[0].DroppedByReason, map[string]int{"status_400": 2}) || second.Channel.Published != 1 {
		t.Fatalf("%+v", second)
	}
}

func Test_the_diagnostics_show_the_live_queue_and_accepted_is_not_delivered(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 3, Reason: "timeout"})
	clock := dependencies.NewSystemClock()
	delivery := services.NewDeliveryService(log_destinations(sink), diagnostics_settings(value_objects.DeliveryModeBestEffort, 5), clock,
		fakes.NewFakeRandom(0.5), nil, dependencies.NewMemoryQueue)
	t.Cleanup(delivery.Stop)
	enqueue_batch_events(t, delivery, 3)

	waiting_snapshot := delivery.Diagnostics()
	destination := waiting_snapshot.Destinations[0]
	if destination.QueueDepth != 3 || destination.QueueCapacity != 100 || destination.Accepted != 3 || destination.Delivered != 0 || destination.LastFlush.Done {
		t.Fatalf("%+v", destination)
	}
	if waiting_snapshot.Channel != (dtos.ChannelDiagnostics{}) {
		t.Fatalf("a service with no channel has channel counters: %+v", waiting_snapshot.Channel)
	}

	delivery.Start()
	delivery.Flush(context.Background())
	destination = delivery.Diagnostics().Destinations[0]
	if destination.QueueDepth != 0 || destination.Accepted != 3 || destination.Delivered != 0 || destination.FailedEvents != 3 {
		t.Fatalf("%+v", destination)
	}
}

func Test_a_service_with_no_destination_answers_an_empty_snapshot(t *testing.T) {
	delivery := new_delivery(t, services.Destinations{}, settings_of(10, 5, 20))
	if snapshot := delivery.Diagnostics(); len(snapshot.Destinations) != 0 || snapshot.Channel != (dtos.ChannelDiagnostics{}) {
		t.Fatalf("%+v", snapshot)
	}
}

func Test_the_last_flush_is_set_by_a_flush_and_by_the_shutdown_and_the_snapshot_still_answers_after_it(t *testing.T) {
	sink := fakes.NewScriptedLogSink()
	delivery := new_diagnostics_delivery(t, sink, diagnostics_settings(value_objects.DeliveryModeBestEffort, 5))
	if delivery.Diagnostics().Destinations[0].LastFlush.Done {
		t.Fatal("a flush that never happened is done")
	}
	enqueue_batch_events(t, delivery, 2)

	delivery.Flush(context.Background())
	after_flush := delivery.Diagnostics().Destinations[0].LastFlush
	if !after_flush.Done || after_flush.Sent != 2 || after_flush.Dropped != 0 || after_flush.Spooled != 0 || after_flush.AtMS <= 0 {
		t.Fatalf("after flush %+v", after_flush)
	}

	enqueue_batch_events(t, delivery, 3)
	delivery.Shutdown(context.Background())
	after_shutdown := delivery.Diagnostics().Destinations[0].LastFlush
	if !after_shutdown.Done || after_shutdown.Sent != 3 || after_shutdown.Dropped != 0 || after_shutdown.AtMS < after_flush.AtMS {
		t.Fatalf("after shutdown %+v", after_shutdown)
	}
	if destination := delivery.Diagnostics().Destinations[0]; destination.Delivered != 5 || destination.Batches != 2 || destination.MaxBatchSize != 3 || destination.LastBatchSize != 3 {
		t.Fatalf("%+v", destination)
	}
}

func Test_a_flush_that_cannot_run_records_the_whole_target_as_dropped(t *testing.T) {
	sink := fakes.NewScriptedLogSink()
	delivery := new_delivery(t, log_destinations(sink), diagnostics_settings(value_objects.DeliveryModeBestEffort, 5))
	enqueue_batch_events(t, delivery, 4) // never started: nobody can send

	result := delivery.Flush(context.Background())

	last_flush := delivery.Diagnostics().Destinations[0].LastFlush
	if result != (dtos.ShutdownResult{Dropped: 4}) || !last_flush.Done || last_flush.Sent != 0 || last_flush.Dropped != 4 {
		t.Fatalf("result %+v, last flush %+v", result, last_flush)
	}
}

func Test_a_callback_that_panics_never_affects_the_delivery_and_creates_no_event(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 1, Reason: "timeout"})
	settings := diagnostics_settings(value_objects.DeliveryModeBestEffort, 5)
	settings.BatchSize = 1
	panicking := func(dtos.DiagnosticEvent) { panic("a secret inside a panic") }
	delivery := new_diagnostics_delivery(t, sink, settings, panicking)

	enqueue_batch_events(t, delivery, 20)
	result := delivery.Flush(context.Background())

	if result.Dropped != 20 || sink.Calls() != 20 {
		t.Fatalf("result %+v, the sink was called %d times", result, sink.Calls())
	}
	waiting.Eventually(t, waiting.DefaultTimeout, "every published diagnostic panicked once", func() bool {
		channel := delivery.Diagnostics().Channel
		return channel.Published > 0 && channel.CallbackErrors == channel.Published
	})
	events_at_the_sink := 0
	for _, batch := range sink.Batches() {
		events_at_the_sink += len(batch)
	}
	if events_at_the_sink != 20 {
		t.Fatalf("the sink received %d events: a diagnostic must never become an event", events_at_the_sink)
	}
	channel := delivery.Diagnostics().Channel
	if channel.Published+channel.Suppressed != 20 {
		t.Fatalf("every failing batch published one diagnostic: %+v", channel)
	}
}

func Test_a_callback_that_never_returns_does_not_hold_the_delivery_the_flush_or_the_shutdown(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 1, Reason: "timeout"})
	settings := diagnostics_settings(value_objects.DeliveryModeBestEffort, 5)
	settings.BatchSize = 1
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	var releasing sync.Once
	release_callback := func() { releasing.Do(func() { close(release) }) }
	hanging := func(dtos.DiagnosticEvent) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
	}
	delivery := new_diagnostics_delivery(t, sink, settings, hanging)
	t.Cleanup(release_callback)

	enqueue_batch_events(t, delivery, 20)
	flush_context, cancel_flush := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel_flush()
	started_at := time.Now()
	result := delivery.Flush(flush_context)
	flush_elapsed := time.Since(started_at)

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the callback was never called")
	}
	if result.Dropped != 20 || sink.Calls() != 20 || flush_elapsed > 3*time.Second {
		t.Fatalf("result %+v, %d calls, flush took %v", result, sink.Calls(), flush_elapsed)
	}

	shutdown_context, cancel_shutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel_shutdown()
	started_at = time.Now()
	delivery.Shutdown(shutdown_context)
	if elapsed := time.Since(started_at); elapsed > 3*time.Second {
		t.Fatalf("Shutdown took %v with a callback that hangs", elapsed)
	}
	if delivery.Diagnostics().Channel.CallbackErrors != 0 {
		t.Fatal("a callback that hangs is not an error")
	}
	release_callback()
}

func Test_no_diagnostic_holds_a_message_a_field_or_a_token(t *testing.T) {
	const secret = "s3cr3t-token-91823"
	const message = "the customer card was refused for order 77"
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 2, Reason: "status 503 from https://collector.example/ingest"}, dtos.SendResult{Dropped: 2, Reason: "serialization_failed"})
	collector := &diagnostics_collector{}
	writer := &text_writer{}
	clock := dependencies.NewSystemClock()
	channel := dependencies.NewDiagnosticsChannel(clock, []func(dtos.DiagnosticEvent){collector.callback}, writer)
	delivery := services.NewDeliveryService(log_destinations(sink), diagnostics_settings(value_objects.DeliveryModeRetry, 3), clock,
		fakes.NewFakeRandom(0.5), channel, dependencies.NewMemoryQueue)
	delivery.Start()
	t.Cleanup(delivery.Stop)
	for id := 1; id <= 2; id++ {
		envelope := numbered(id)
		envelope.EventType = "log.entry.recorded"
		envelope.Data = map[string]any{"message": message, "token": secret}
		envelope.Attributes = value_objects.Attributes{"authorization": secret}
		delivery.Enqueue(context.Background(), services.DestinationKindLog, envelope)
	}

	delivery.Flush(context.Background())

	waiting.Eventually(t, waiting.DefaultTimeout, "the diagnostics arrived", func() bool { return collector.count() == 2 })
	waiting.Eventually(t, waiting.DefaultTimeout, "the lines were written", func() bool { return strings.Count(writer.String(), "\n") == 2 })
	received, marshal_error := json.Marshal(collector.snapshot())
	if marshal_error != nil {
		t.Fatal(marshal_error)
	}
	snapshot_json, marshal_error := json.Marshal(delivery.Diagnostics())
	if marshal_error != nil {
		t.Fatal(marshal_error)
	}
	for _, text := range []string{string(received), string(snapshot_json), writer.String()} {
		for _, forbidden := range []string{secret, message, "customer", "order 77", "://", "status 503"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%q appears in %s", forbidden, text)
			}
		}
	}
	if first := collector.snapshot()[0]; first.Reason != "status_503_from_https:__collector.example_ingest" {
		t.Fatalf("the reason was not cleaned: %q", first.Reason)
	}
}

func Test_the_drops_of_a_queue_are_published_one_per_event_with_the_reason_of_the_drop(t *testing.T) {
	fake_channel := &fakes.FakeDiagnosticsChannel{}
	classes := map[string]value_objects.EventClass{alert_type: value_objects.EventClassCritical}
	settings := policy_settings(2, value_objects.QueuePolicyDropNewest, 0, classes)
	delivery := services.NewDeliveryService(log_destinations(&fakes.FakeLogSink{}), settings, dependencies.NewSystemClock(), fakes.NewFakeRandom(),
		fake_channel, dependencies.NewMemoryQueue)
	t.Cleanup(delivery.Stop)

	enqueue(delivery, normal_event(1))
	enqueue(delivery, normal_event(2))
	enqueue(delivery, normal_event(3))
	enqueue(delivery, normal_event(4))
	enqueue(delivery, alert_event(5))

	expected := []struct {
		reason string
	}{{services.ReasonQueueFullNewest}, {services.ReasonQueueFullNewest}, {services.ReasonDisplacedByCritical}}
	events := fake_channel.Events()
	if len(events) != len(expected) {
		t.Fatalf("%+v", events)
	}
	for position, event := range events {
		if event.Code != value_objects.DiagnosticCodeEventsDropped || event.Reason != expected[position].reason || event.Count != 1 ||
			event.Kind != "log" || event.Index != 0 || event.AtMS <= 0 {
			t.Errorf("diagnostic %d: %+v", position, event)
		}
	}
}

func Test_a_drop_oldest_and_a_call_after_the_shutdown_are_published_too(t *testing.T) {
	fake_channel := &fakes.FakeDiagnosticsChannel{}
	settings := policy_settings(1, value_objects.QueuePolicyDropOldest, 0, nil)
	delivery := services.NewDeliveryService(log_destinations(&fakes.FakeLogSink{}), settings, dependencies.NewSystemClock(), fakes.NewFakeRandom(),
		fake_channel, dependencies.NewMemoryQueue)
	t.Cleanup(delivery.Stop)
	enqueue(delivery, normal_event(1))
	enqueue(delivery, normal_event(2)) // removes the oldest

	delivery.Start()
	delivery.Shutdown(context.Background())
	closed_result := enqueue(delivery, normal_event(3))

	if closed_result.Reason != services.ReasonShutdown {
		t.Fatalf("%+v", closed_result)
	}
	events := fake_channel.Events()
	reasons := make([]string, 0, len(events))
	for _, event := range events {
		if event.Code != value_objects.DiagnosticCodeEventsDropped || event.Count != 1 {
			t.Errorf("%+v", event)
		}
		reasons = append(reasons, event.Reason)
	}
	if !reflect.DeepEqual(reasons, []string{services.ReasonQueueFullOldest, services.ReasonShutdown}) {
		t.Fatalf("%v", reasons)
	}
	starts, stops := fake_channel.Calls()
	if starts != 1 || stops < 1 {
		t.Fatalf("the channel was started %d and stopped %d times", starts, stops)
	}
}

func Test_a_destination_kind_with_no_sink_and_a_call_after_shutdown_publish_for_the_destinations_only(t *testing.T) {
	fake_channel := &fakes.FakeDiagnosticsChannel{}
	delivery := services.NewDeliveryService(log_destinations(&fakes.FakeLogSink{}), settings_of(10, 5, 20), dependencies.NewSystemClock(),
		fakes.NewFakeRandom(), fake_channel, dependencies.NewMemoryQueue)
	t.Cleanup(delivery.Stop)

	delivery.Enqueue(context.Background(), services.DestinationKindMetrics, numbered(1)) // no destination of the kind
	if len(fake_channel.Events()) != 0 {
		t.Fatalf("%+v", fake_channel.Events())
	}
	delivery.Shutdown(context.Background())
	delivery.RejectIfClosed(services.DestinationKindMetrics)
	if len(fake_channel.Events()) != 0 {
		t.Fatalf("%+v", fake_channel.Events())
	}
	delivery.RejectIfClosed(services.DestinationKindLog)
	events := fake_channel.Events()
	if len(events) != 1 || events[0].Reason != services.ReasonShutdown || events[0].Code != value_objects.DiagnosticCodeEventsDropped {
		t.Fatalf("%+v", events)
	}
}

func Test_the_events_left_in_the_queue_at_the_shutdown_are_published_as_one_diagnostic_with_their_number(t *testing.T) {
	fake_channel := &fakes.FakeDiagnosticsChannel{}
	blocking_sink := fakes.NewBlockingLogSink()
	t.Cleanup(blocking_sink.Release)
	settings := diagnostics_settings(value_objects.DeliveryModeBestEffort, 5)
	settings.BatchSize = 1
	settings.FlushIntervalMS = 1
	delivery := services.NewDeliveryService(log_destinations(blocking_sink), settings, dependencies.NewSystemClock(), fakes.NewFakeRandom(),
		fake_channel, dependencies.NewMemoryQueue)
	delivery.Start()
	t.Cleanup(delivery.Stop)
	enqueue_batch_events(t, delivery, 1)
	waiting.Eventually(t, waiting.DefaultTimeout, "the first call is blocked in the sink", func() bool { return blocking_sink.Calls() == 1 })
	enqueue_batch_events(t, delivery, 2)

	shutdown_context, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	delivery.Shutdown(shutdown_context)

	found := false
	for _, event := range fake_channel.Events() {
		if event.Code == value_objects.DiagnosticCodeEventsDropped && event.Reason == services.ReasonShutdown && event.Count == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", fake_channel.Events())
	}
}

func Test_a_batch_that_is_abandoned_while_it_waits_for_a_retry_is_published_as_dropped_by_shutdown(t *testing.T) {
	fake_channel := &fakes.FakeDiagnosticsChannel{}
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 3, Reason: "status_503", RetryAfterMS: 60_000})
	delivery := services.NewDeliveryService(log_destinations(sink), mode_settings(value_objects.DeliveryModeRetry, 5), dependencies.NewSystemClock(),
		fakes.NewFakeRandom(0.5), fake_channel, dependencies.NewMemoryQueue)
	delivery.Start()
	t.Cleanup(delivery.Stop)
	enqueue_batch_events(t, delivery, 3)
	waiting.Eventually(t, waiting.DefaultTimeout, "the first attempt is made", func() bool { return delivery.Stats()[0].Attempts == 1 })

	shutdown_context, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	delivery.Shutdown(shutdown_context)

	waiting.Eventually(t, waiting.DefaultTimeout, "the abandoned batch was published", func() bool { return len(fake_channel.Events()) >= 2 })
	events := fake_channel.Events()
	if events[0].Code != value_objects.DiagnosticCodeExportFailed || events[0].Reason != "status_503" || events[0].Count != 3 {
		t.Fatalf("%+v", events[0])
	}
	if events[1].Code != value_objects.DiagnosticCodeEventsDropped || events[1].Reason != services.ReasonShutdown || events[1].Count != 3 {
		t.Fatalf("%+v", events[1])
	}
}

func Test_a_sink_that_panics_is_an_export_failure_with_the_reason_sink_panicked(t *testing.T) {
	fake_channel := &fakes.FakeDiagnosticsChannel{}
	sink := fakes.NewScriptedLogSink()
	sink.PanicOnCalls(0)
	delivery := services.NewDeliveryService(log_destinations(sink), diagnostics_settings(value_objects.DeliveryModeBestEffort, 5), dependencies.NewSystemClock(),
		fakes.NewFakeRandom(0.5), fake_channel, dependencies.NewMemoryQueue)
	delivery.Start()
	t.Cleanup(delivery.Stop)
	enqueue_batch_events(t, delivery, 4)

	delivery.Flush(context.Background())

	events := fake_channel.Events()
	if len(events) != 1 || events[0].Code != value_objects.DiagnosticCodeExportFailed || events[0].Reason != services.ReasonSinkPanicked || events[0].Count != 4 {
		t.Fatalf("%+v", events)
	}
	if destination := delivery.Diagnostics().Destinations[0]; destination.ExportFailures != 1 || destination.FailedEvents != 4 {
		t.Fatalf("%+v", destination)
	}
}

func Test_a_retry_that_runs_out_of_attempts_publishes_one_drop_with_the_reason_retry_limit_reached(t *testing.T) {
	fake_channel := &fakes.FakeDiagnosticsChannel{}
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 2, Reason: "timeout"})
	delivery := services.NewDeliveryService(log_destinations(sink), diagnostics_settings(value_objects.DeliveryModeRetry, 2), dependencies.NewSystemClock(),
		fakes.NewFakeRandom(0.5), fake_channel, dependencies.NewMemoryQueue)
	delivery.Start()
	t.Cleanup(delivery.Stop)
	enqueue_batch_events(t, delivery, 2)

	delivery.Flush(context.Background())

	codes := []string{}
	for _, event := range fake_channel.Events() {
		codes = append(codes, string(event.Code)+":"+event.Reason)
	}
	if !reflect.DeepEqual(codes, []string{"export_failed:timeout", "export_failed:timeout", "events_dropped:retry_limit_reached"}) {
		t.Fatalf("%v", codes)
	}
}

func Test_a_service_with_no_channel_publishes_nothing_and_a_service_with_one_starts_and_stops_it_once(t *testing.T) {
	fake_channel := &fakes.FakeDiagnosticsChannel{}
	delivery := services.NewDeliveryService(log_destinations(&fakes.FakeLogSink{}), settings_of(10, 5, 20), dependencies.NewSystemClock(),
		fakes.NewFakeRandom(), fake_channel, dependencies.NewMemoryQueue)
	goroutines_before := runtime.NumGoroutine()
	if starts, _ := fake_channel.Calls(); starts != 0 {
		t.Fatal("the constructor started the channel")
	}
	waiting.StaysTrue(t, 30*time.Millisecond, "the constructor started no goroutine", func() bool { return runtime.NumGoroutine() <= goroutines_before })

	delivery.Start()
	delivery.Start()
	delivery.Stop()
	delivery.Stop()

	if starts, stops := fake_channel.Calls(); starts != 1 || stops != 2 {
		t.Fatalf("started %d times and stopped %d times", starts, stops)
	}
}

func Test_the_drop_reasons_of_the_service_are_the_values_of_the_generated_enum(t *testing.T) {
	in_the_code := []string{
		services.ReasonQueueFullNewest, services.ReasonQueueFullOldest, services.ReasonDisplacedByCritical, services.ReasonBlockTimeout,
		services.ReasonUnspecified, services.ReasonShutdown, services.ReasonRetryLimitReached, services.ReasonNoDestination, services.ReasonSinkPanicked,
	}
	in_the_enum := []string{}
	for _, reason := range value_objects.DropReasonValues() {
		in_the_enum = append(in_the_enum, string(reason))
	}
	slices.Sort(in_the_code)
	slices.Sort(in_the_enum)
	if !reflect.DeepEqual(in_the_code, in_the_enum) {
		t.Fatalf("the code produces %v and the enum lists %v", in_the_code, in_the_enum)
	}
	if got := value_objects.DiagnosticCodeValues(); len(got) != 4 {
		t.Fatalf("%v", got)
	}
}

func Test_the_snapshot_shows_the_reasons_cleaned_and_sums_the_keys_that_collapse_while_the_stats_stay_raw(t *testing.T) {
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Dropped: 1, Reason: "bad reason://x"}, dtos.SendResult{Dropped: 2, Reason: "bad/reason//x"})
	settings := diagnostics_settings(value_objects.DeliveryModeBestEffort, 5)
	settings.BatchSize = 1
	delivery := new_diagnostics_delivery(t, sink, settings)
	enqueue_batch_events(t, delivery, 2)
	delivery.Flush(context.Background())

	destination := delivery.Diagnostics().Destinations[0]
	if !reflect.DeepEqual(destination.DroppedByReason, map[string]int{"bad_reason:__x": 1, "bad_reason__x": 2}) {
		t.Fatalf("%v", destination.DroppedByReason)
	}
	if destination.LastReason != "bad_reason__x" && destination.LastReason != "bad_reason:__x" {
		t.Fatalf("%q", destination.LastReason)
	}
	raw := delivery.Stats()[0]
	if raw.DroppedByReason["bad reason://x"] != 1 || raw.LastReason == destination.LastReason {
		t.Fatalf("the stats were cleaned: %+v", raw)
	}

	collapsing := fakes.NewScriptedLogSink(dtos.SendResult{Dropped: 1, Reason: "a b"}, dtos.SendResult{Dropped: 2, Reason: "a?b"})
	second := new_diagnostics_delivery(t, collapsing, settings)
	enqueue_batch_events(t, second, 2)
	second.Flush(context.Background())
	if got := second.Diagnostics().Destinations[0].DroppedByReason; !reflect.DeepEqual(got, map[string]int{"a_b": 3}) {
		t.Fatalf("%v", got)
	}
}
