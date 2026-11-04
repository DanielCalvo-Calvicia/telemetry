package composition_root_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/containers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	infra_config "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
)

func service_config(service_name string) infra_config.Config {
	cfg := infra_config.Microservice()
	cfg.Resource.ServiceName = service_name
	cfg.Resource.InstanceID = "instance-1"
	// The delivery is asynchronous: a short flush interval lets a batch that is not full leave in a few milliseconds.
	cfg.Delivery.FlushIntervalMS = 5
	return cfg
}

func log_input() dtos.NewEventInput {
	return dtos.NewEventInput{
		EventType: "log.entry.recorded",
		Severity:  value_objects.SeverityInfo,
		Data:      map[string]any{"category": "http", "message": "started"},
	}
}

func new_core(t *testing.T, cfg infra_config.Config, options ...config.Option) *containers.Core {
	t.Helper()
	core, core_error := containers.NewCore(context.Background(), cfg, options...)
	if core_error != nil {
		t.Fatalf("NewCore failed: %v", core_error)
	}
	return core
}

func canonical_of(t *testing.T, envelope value_objects.Envelope) string {
	t.Helper()
	canonical_text, canonical_error := canonical_json.CanonicalJSON(envelope)
	if canonical_error != nil {
		t.Fatal(canonical_error)
	}
	return string(canonical_text)
}

func Test_two_cores_in_one_process_share_nothing(t *testing.T) {
	first_sink, second_sink := &fakes.FakeLogSink{}, &fakes.FakeLogSink{}
	first_core := new_core(t, service_config("first-service"), config.WithLogSink(first_sink))
	second_core := new_core(t, service_config("second-service"), config.WithLogSink(second_sink))

	first_core.Emit(context.Background(), log_input())
	first_core.Emit(context.Background(), log_input())
	second_core.Emit(context.Background(), log_input())

	events_in := func(sink *fakes.FakeLogSink) int {
		total := 0
		for _, batch := range sink.Batches() {
			total += len(batch)
		}
		return total
	}
	waiting.Eventually(t, waiting.DefaultTimeout, "each core delivers its own events", func() bool { return events_in(first_sink) == 2 && events_in(second_sink) == 1 })
	waiting.StaysTrue(t, 30*time.Millisecond, "the events do not cross", func() bool { return events_in(first_sink) == 2 && events_in(second_sink) == 1 })
	if text := canonical_of(t, first_sink.Batches()[0][0]); !strings.Contains(text, "first-service") || strings.Contains(text, "second-service") {
		t.Fatalf("the first core used another configuration: %s", text)
	}
	if text := canonical_of(t, second_sink.Batches()[0][0]); !strings.Contains(text, "second-service") || strings.Contains(text, "first-service") {
		t.Fatalf("the second core used another configuration: %s", text)
	}
}

func Test_sixteen_goroutines_emit_on_one_core_and_the_counts_add_up(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	core := new_core(t, service_config("busy-service"), config.WithLogSink(sink))

	const goroutines, emits_per_goroutine = 16, 50
	var wait_group sync.WaitGroup
	var mutex sync.Mutex
	total := dtos.EmitResult{}
	for goroutine := 0; goroutine < goroutines; goroutine++ {
		wait_group.Add(1)
		go func() {
			defer wait_group.Done()
			for index := 0; index < emits_per_goroutine; index++ {
				result := core.Emit(context.Background(), log_input())
				mutex.Lock()
				total.Accepted += result.Accepted
				total.Dropped += result.Dropped
				total.Failed += result.Failed
				mutex.Unlock()
			}
		}()
	}
	wait_group.Wait()

	waiting.Eventually(t, waiting.DefaultTimeout, "the sink receives every event", func() bool {
		received_so_far := 0
		for _, batch := range sink.Batches() {
			received_so_far += len(batch)
		}
		return received_so_far == goroutines*emits_per_goroutine
	})
	if total.Accepted != goroutines*emits_per_goroutine || total.Dropped != 0 || total.Failed != 0 {
		t.Fatalf("total = %+v", total)
	}
	received := 0
	seen_ids := map[string]bool{}
	for _, batch := range sink.Batches() {
		for _, envelope := range batch {
			received++
			seen_ids[envelope.EventId] = true
		}
	}
	if received != goroutines*emits_per_goroutine || len(seen_ids) != received {
		t.Fatalf("the sink received %d events with %d different ids", received, len(seen_ids))
	}
}

func Test_an_invalid_configuration_returns_a_validation_error_that_names_the_field(t *testing.T) {
	cfg := service_config("")
	cfg.Delivery.QueueSize = 0

	core, core_error := containers.NewCore(context.Background(), cfg)

	var validation_error *infra_config.ValidationError
	if core != nil || !errors.As(core_error, &validation_error) {
		t.Fatalf("expected a *ValidationError and no core, got %v", core_error)
	}
	var fields []string
	for _, problem := range validation_error.Problems {
		fields = append(fields, problem.Field)
	}
	if strings.Join(fields, ",") != "resource.service_name,delivery.queue_size" {
		t.Fatalf("fields = %v", fields)
	}
	if !errors.Is(core_error, domain.ErrConfigFieldInvalid) {
		t.Fatal("the error does not wrap ErrConfigFieldInvalid")
	}
}

func Test_the_durable_mode_is_refused_at_start_with_one_problem_with_or_without_a_spool(t *testing.T) {
	for _, profile := range []func() infra_config.Config{infra_config.Robotics, infra_config.OfflineDevice} {
		cfg := profile()
		cfg.Resource.ServiceName = "robot"
		cfg.Spool.Directory = "spool-dir"

		_, without_spool_error := containers.NewCore(context.Background(), cfg)
		_, with_spool_error := containers.NewCore(context.Background(), cfg, config.WithLocalSpool(&fakes.FakeLocalSpool{}))

		for label, core_error := range map[string]error{"without a spool adapter": without_spool_error, "with a spool adapter": with_spool_error} {
			var validation_error *infra_config.ValidationError
			if !errors.As(core_error, &validation_error) || len(validation_error.Problems) != 1 ||
				validation_error.Problems[0].Field != "delivery.mode" || validation_error.Problems[0].Code != value_objects.ErrorCodeConfigFieldInvalid {
				t.Fatalf("%s: %v", label, core_error)
			}
			if !strings.Contains(validation_error.Problems[0].Reason, "not available yet") {
				t.Fatalf("%s: the reason does not say it is not available: %q", label, validation_error.Problems[0].Reason)
			}
		}
	}
}

func Test_a_durable_mode_without_a_spool_directory_adds_the_mode_problem_after_the_one_of_the_configuration(t *testing.T) {
	cfg := infra_config.Robotics()
	cfg.Resource.ServiceName = "robot"

	_, core_error := containers.NewCore(context.Background(), cfg)

	var validation_error *infra_config.ValidationError
	if !errors.As(core_error, &validation_error) || len(validation_error.Problems) != 2 ||
		validation_error.Problems[0].Field != "spool.directory" || validation_error.Problems[1].Field != "delivery.mode" {
		t.Fatalf("both: %v", core_error)
	}
}

func Test_the_options_append_and_a_nil_option_or_argument_is_ignored(t *testing.T) {
	first_sink, second_sink := &fakes.FakeLogSink{}, &fakes.FakeLogSink{}
	options := []config.Option{
		config.WithLogSink(first_sink), nil, config.WithLogSink(nil), config.WithLogSink(second_sink),
		config.WithEventSink(nil), config.WithMetricsSink(nil), config.WithTraceSink(nil),
		config.WithDeviceTelemetrySink(nil), config.WithAITelemetrySink(nil), config.WithLocalSpool(nil),
		config.WithClock(nil), config.WithIDGenerator(nil),
	}
	core := new_core(t, service_config("service"), options...)

	result := core.Emit(context.Background(), log_input())

	if result.Accepted != 2 {
		t.Fatalf("result = %+v", result)
	}
	waiting.Eventually(t, waiting.DefaultTimeout, "both sinks receive the event", func() bool { return first_sink.Calls() == 1 && second_sink.Calls() == 1 })
}

func Test_every_option_sets_its_part_of_the_options(t *testing.T) {
	options := config.Options{}
	spool := &fakes.FakeLocalSpool{}
	clock := fakes.NewFakeClock(value_objects.UTCInstant{})
	id_generator := fakes.NewFakeIDGenerator()
	for _, option := range []config.Option{
		config.WithLogSink(&fakes.FakeLogSink{}), config.WithEventSink(&fakes.FakeEventSink{}), config.WithMetricsSink(&fakes.FakeMetricsSink{}),
		config.WithTraceSink(&fakes.FakeTraceSink{}), config.WithDeviceTelemetrySink(&fakes.FakeDeviceTelemetrySink{}),
		config.WithAITelemetrySink(&fakes.FakeAITelemetrySink{}), config.WithLocalSpool(spool), config.WithClock(clock),
		config.WithIDGenerator(id_generator),
	} {
		option(&options)
	}
	destinations := options.Destinations
	if len(destinations.Log) != 1 || len(destinations.Event) != 1 || len(destinations.Metrics) != 1 || len(destinations.Trace) != 1 ||
		len(destinations.DeviceTelemetry) != 1 || len(destinations.AITelemetry) != 1 {
		t.Fatalf("destinations = %+v", destinations)
	}
	if options.Spool != spool || options.Clock != clock || options.IDGenerator != id_generator {
		t.Fatalf("options = %+v", options)
	}
}

func Test_the_clock_and_the_id_generator_that_are_given_are_used_and_the_defaults_work(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	event_id, id_error := value_objects.NewEventID("01900000-0000-7000-8000-0000000000aa")
	if id_error != nil {
		t.Fatal(id_error)
	}
	wall_time, time_error := value_objects.ParseUTCInstant("2026-10-28T10:00:00.000000Z")
	if time_error != nil {
		t.Fatal(time_error)
	}
	fake_clock := fakes.NewFakeClock(wall_time)
	fake_clock.SetStep(10_000_000) // the delivery reads this clock too: 10 ms a reading moves it faster than the flush interval
	core := new_core(t, service_config("service"), config.WithLogSink(sink),
		config.WithClock(fake_clock), config.WithIDGenerator(fakes.NewFakeIDGenerator(event_id)))
	core.Emit(context.Background(), log_input())
	waiting.Eventually(t, waiting.DefaultTimeout, "the sink receives the event", func() bool { return sink.Calls() == 1 })
	envelope := sink.Batches()[0][0]
	if envelope.EventId != event_id.String() || envelope.RecordedAt != wall_time.String() {
		t.Fatalf("the fakes were not used: %s", canonical_of(t, envelope))
	}

	default_sink := &fakes.FakeLogSink{}
	default_core := new_core(t, service_config("service"), config.WithLogSink(default_sink))
	default_core.Emit(context.Background(), log_input())
	waiting.Eventually(t, waiting.DefaultTimeout, "the default sink receives the event", func() bool { return default_sink.Calls() == 1 })
	default_envelope := default_sink.Batches()[0][0]
	if validation_error := value_objects.ValidateEventID(default_envelope.EventId); validation_error != nil {
		t.Fatalf("the default id is not a UUIDv7: %v", validation_error)
	}
	if _, parse_error := value_objects.ParseUTCInstant(default_envelope.RecordedAt); parse_error != nil {
		t.Fatalf("the default clock gave %q", default_envelope.RecordedAt)
	}
}

func Test_a_failure_to_generate_the_instance_id_is_returned_by_new(t *testing.T) {
	failing_id_generator := fakes.NewFakeIDGenerator()
	failing_id_generator.FailWith(errors.New("no randomness"))
	cfg := service_config("service")
	cfg.Resource.InstanceID = ""

	_, core_error := containers.NewCore(context.Background(), cfg, config.WithIDGenerator(failing_id_generator))

	if core_error == nil || value_objects.CodeOf(core_error) != value_objects.ErrorCodeInstanceIdUnavailable {
		t.Fatalf("expected instance_id_unavailable, got %v", core_error)
	}
}

func Test_shutdown_returns_the_zero_result_and_never_panics_also_on_a_nil_core(t *testing.T) {
	core := new_core(t, service_config("service"))
	if result := core.Shutdown(context.Background()); result != (dtos.ShutdownResult{}) {
		t.Fatalf("result = %+v", result)
	}
	var nil_core *containers.Core
	if result := nil_core.Shutdown(context.Background()); result != (dtos.ShutdownResult{}) {
		t.Fatalf("nil core: result = %+v", result)
	}
}

func Test_a_nil_core_drops_the_event_instead_of_panicking(t *testing.T) {
	var nil_core *containers.Core
	if result := nil_core.Emit(context.Background(), log_input()); result != (dtos.EmitResult{Dropped: 1, Reason: value_objects.ErrorCodeEventBuildFailed}) {
		t.Fatalf("result = %+v", result)
	}
	if result := (&containers.Core{}).Emit(context.Background(), log_input()); result.Dropped != 1 {
		t.Fatalf("a zero core: result = %+v", result)
	}
}

// ----- THE SPY THROUGH THE WHOLE CORE ---------------------------------------

func Test_no_sink_receives_a_secret_through_the_whole_core_in_data_attributes_or_context(t *testing.T) {
	secrets := []struct{ name, text, core string }{
		{"aws access key", "AKIAIOSFODNN7EXAMPLE", "AKIAIOSFODNN7EXAMPLE"},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.c2lnbmF0dXJl", "eyJzdWIiOiJ0ZXN0In0"},
		{"connection string", "postgres://admin:hunter2@db:5432/app", "hunter2"},
		{"bearer token", "Authorization: Bearer abcdef123456", "abcdef123456"},
		{"password pair", "password=hunter2", "hunter2"},
	}
	for _, secret := range secrets {
		for _, place := range []string{"data", "attributes", "context"} {
			t.Run(secret.name+"/"+place, func(t *testing.T) {
				sink := &fakes.FakeLogSink{}
				core := new_core(t, service_config("service"), config.WithLogSink(sink))
				input := log_input()
				switch place {
				case "data":
					input.Data["message"] = "x " + secret.text + " y"
				case "attributes":
					attributes, attributes_error := value_objects.NewEventAttributes(map[string]any{"note": secret.text})
					if attributes_error != nil {
						t.Fatal(attributes_error)
					}
					input.Attributes = &attributes
				case "context":
					correlation_context, context_error := value_objects.NewCorrelationContext(value_objects.WithCorrelationID(secret.text))
					if context_error != nil {
						t.Fatal(context_error)
					}
					input.Context = &correlation_context
				}

				if result := core.Emit(context.Background(), input); result.Accepted != 1 {
					t.Fatalf("result = %+v", result)
				}
				waiting.Eventually(t, waiting.DefaultTimeout, "the sink receives the event", func() bool { return sink.Calls() == 1 })
				sent_text := canonical_of(t, sink.Batches()[0][0])
				if strings.Contains(sent_text, secret.core) {
					t.Fatalf("the sink received %q: %s", secret.core, sent_text)
				}
			})
		}
	}
}

func Test_an_invalid_delivery_setting_is_refused_at_start_with_the_field_and_the_reason(t *testing.T) {
	cases := []struct {
		name   string
		change func(cfg *infra_config.Config)
		field  string
	}{
		{"queue zero", func(cfg *infra_config.Config) { cfg.Delivery.QueueSize = 0 }, "delivery.queue_size"},
		{"batch zero", func(cfg *infra_config.Config) { cfg.Delivery.BatchSize = 0 }, "delivery.batch_size"},
		{"flush zero", func(cfg *infra_config.Config) { cfg.Delivery.FlushIntervalMS = 0 }, "delivery.flush_interval_ms"},
		{"batch over queue", func(cfg *infra_config.Config) { cfg.Delivery.QueueSize = 5; cfg.Delivery.BatchSize = 6 }, "delivery.batch_size"},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			cfg := service_config("service")
			test_case.change(&cfg)
			core, core_error := containers.NewCore(context.Background(), cfg, config.WithLogSink(&fakes.FakeLogSink{}))
			var validation_error *infra_config.ValidationError
			if core != nil || !errors.As(core_error, &validation_error) {
				t.Fatalf("expected a validation error, got %v", core_error)
			}
			found := false
			for _, problem := range validation_error.Problems {
				if problem.Field == test_case.field && problem.Code == value_objects.ErrorCodeConfigFieldInvalid && problem.Reason != "" {
					found = true
				}
			}
			if !found {
				t.Fatalf("no problem for %s with a reason: %+v", test_case.field, validation_error.Problems)
			}
		})
	}
}
