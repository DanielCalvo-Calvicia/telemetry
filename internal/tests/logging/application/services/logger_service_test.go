package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
	call_depth "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/call_depth"
	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	ports_inbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/inbound"
	ports_outbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
	services "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/services"
	operations "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/operations"
	value_objects "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

var reference_time = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

type fake_clock struct {
	current_time time.Time
}

func (clock fake_clock) Now(_ context.Context) time.Time {
	return clock.current_time
}

type fake_log_sink struct {
	received_entries []dtos.LogEntryOutboundDTO
	received_context context.Context
	error_to_return  error
}

func (log_sink *fake_log_sink) Send(ctx context.Context, log_entry dtos.LogEntryOutboundDTO) error {
	log_sink.received_context = ctx
	log_sink.received_entries = append(log_sink.received_entries, log_entry)
	return log_sink.error_to_return
}

func new_masker() operations.SensitiveDataMasker {
	masker, err := operations.NewSensitiveDataMasker(nil)
	if err != nil {
		panic(err)
	}
	return masker
}

var (
	_ ports_outbound.LogSinkPort = (*fake_log_sink)(nil)
	_ ports_outbound.ClockPort   = fake_clock{}
)

func new_logger(log_sinks ...ports_outbound.LogSinkPort) ports_inbound.LoggerPort {
	return services.NewLoggerService(log_sinks, nil, fake_clock{current_time: reference_time}, value_objects.LevelTrace, new_masker())
}

func new_logger_with_fallback(fallback_sink ports_outbound.LogSinkPort, log_sinks ...ports_outbound.LogSinkPort) ports_inbound.LoggerPort {
	return services.NewLoggerService(log_sinks, fallback_sink, fake_clock{current_time: reference_time}, value_objects.LevelTrace, new_masker())
}

func Test_every_level_method_sends_its_own_level(t *testing.T) {
	test_cases := []struct {
		expected_level string
		log_method     func(ports_inbound.LoggerPort, context.Context, dtos.LogEntryInboundDTO) error
	}{
		{"TRACE", ports_inbound.LoggerPort.Trace},
		{"DEBUG", ports_inbound.LoggerPort.Debug},
		{"INFO", ports_inbound.LoggerPort.Info},
		{"WARN", ports_inbound.LoggerPort.Warn},
		{"ERROR", ports_inbound.LoggerPort.Error},
		{"FATAL", ports_inbound.LoggerPort.Fatal},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.expected_level, func(t *testing.T) {
			log_sink := &fake_log_sink{}
			logger := new_logger(log_sink)

			if err := test_case.log_method(logger, context.Background(), dtos.LogEntryInboundDTO{Message: "hello"}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(log_sink.received_entries) != 1 || log_sink.received_entries[0].Level != test_case.expected_level {
				t.Errorf("received %+v, want one entry at %s", log_sink.received_entries, test_case.expected_level)
			}
		})
	}
}

func Test_entries_below_the_minimum_level_are_dropped(t *testing.T) {
	test_cases := []struct {
		name          string
		environment   string
		expected_kept []string
	}{
		{"development keeps DEBUG and above", "development", []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}},
		{"production keeps INFO and above", "production", []string{"INFO", "WARN", "ERROR", "FATAL"}},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			minimum_level, err := value_objects.MinimumLevelFor(test_case.environment)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			log_sink := &fake_log_sink{}
			logger := services.NewLoggerService([]ports_outbound.LogSinkPort{log_sink}, nil, fake_clock{current_time: reference_time}, minimum_level, new_masker())

			entry := dtos.LogEntryInboundDTO{Message: "m"}
			for _, log_method := range []func(context.Context, dtos.LogEntryInboundDTO) error{logger.Trace, logger.Debug, logger.Info, logger.Warn, logger.Error, logger.Fatal} {
				if err := log_method(context.Background(), entry); err != nil {
					t.Fatalf("a dropped or kept entry must not fail: %v", err)
				}
			}

			kept_levels := []string{}
			for _, received_entry := range log_sink.received_entries {
				kept_levels = append(kept_levels, received_entry.Level)
			}
			if strings.Join(kept_levels, ",") != strings.Join(test_case.expected_kept, ",") {
				t.Errorf("kept levels = %v, want %v", kept_levels, test_case.expected_kept)
			}
		})
	}
}

func Test_the_entry_carries_everything_the_caller_reported(t *testing.T) {
	log_sink := &fake_log_sink{}
	logger := new_logger(log_sink)
	reported_fields := map[string]any{"bucket": "b"}

	err := logger.Info(context.Background(), dtos.LogEntryInboundDTO{
		ProducerID:    "producer",
		ApplicationID: "application-id",
		ProcessID:     "process-id",
		Application:   "app",
		Category:      "service",
		Function:      "Put",
		Indentation:   2,
		Depth:         3,
		Message:       "stored",
		Fields:        reported_fields,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent_entry := log_sink.received_entries[0]
	if sent_entry.ProducerID != "producer" || sent_entry.ApplicationID != "application-id" || sent_entry.ProcessID != "process-id" ||
		sent_entry.Application != "app" || sent_entry.Category != "service" || sent_entry.Function != "Put" ||
		sent_entry.Indentation != 2 || sent_entry.Depth != 3 || sent_entry.Message != "stored" || sent_entry.Fields["bucket"] != "b" {
		t.Errorf("sent entry = %+v", sent_entry)
	}
	if !sent_entry.Timestamp.Equal(reference_time) {
		t.Errorf("Timestamp = %v, want the time of the clock %v", sent_entry.Timestamp, reference_time)
	}

	reported_fields["bucket"] = "changed"
	if sent_entry.Fields["bucket"] != "b" {
		t.Error("the entry must not share the caller's map")
	}
}

func Test_the_call_context_overrides_the_identity_and_the_indentation(t *testing.T) {
	log_sink := &fake_log_sink{}
	ctx := call_depth.WithCallContext(context.Background(), call_depth.CallContext{
		Application: "app-from-context", ApplicationID: "instance-from-context", ProcessID: "request-from-context", Indentation: 3,
	})

	err := new_logger(log_sink).Info(ctx, dtos.LogEntryInboundDTO{
		ProducerID: "producer", Application: "ignored", ApplicationID: "ignored", ProcessID: "ignored", Indentation: 9, Depth: 2, Message: "m",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent_entry := log_sink.received_entries[0]
	if sent_entry.Application != "app-from-context" || sent_entry.ApplicationID != "instance-from-context" ||
		sent_entry.ProcessID != "request-from-context" || sent_entry.Indentation != 3 {
		t.Errorf("sent entry = %+v, want the values of the call context", sent_entry)
	}
	if sent_entry.ProducerID != "producer" || sent_entry.Depth != 2 {
		t.Errorf("the producer and the depth reported by the caller must be kept: %+v", sent_entry)
	}
}

func Test_a_context_without_call_context_keeps_the_values_the_caller_reported(t *testing.T) {
	log_sink := &fake_log_sink{}

	err := new_logger(log_sink).Info(context.Background(), dtos.LogEntryInboundDTO{
		Application: "app", ApplicationID: "instance", ProcessID: "request", Indentation: 4, Message: "m",
	})
	if err != nil {
		t.Fatalf("a missing call context must not be an error: %v", err)
	}

	sent_entry := log_sink.received_entries[0]
	if sent_entry.Application != "app" || sent_entry.ApplicationID != "instance" || sent_entry.ProcessID != "request" || sent_entry.Indentation != 4 {
		t.Errorf("sent entry = %+v", sent_entry)
	}
}

func Test_the_context_reaches_the_sinks(t *testing.T) {
	type context_key struct{}
	log_sink := &fake_log_sink{}
	ctx := context.WithValue(context.Background(), context_key{}, "marker")

	if err := new_logger(log_sink).Debug(ctx, dtos.LogEntryInboundDTO{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log_sink.received_context.Value(context_key{}) != "marker" {
		t.Error("the sink must receive the caller's context")
	}
}

func Test_every_sink_receives_the_entry(t *testing.T) {
	first_sink := &fake_log_sink{}
	second_sink := &fake_log_sink{}

	if err := new_logger(first_sink, second_sink).Warn(context.Background(), dtos.LogEntryInboundDTO{Message: "m"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(first_sink.received_entries) != 1 || len(second_sink.received_entries) != 1 {
		t.Error("both sinks must receive the entry")
	}
}

func Test_a_failing_sink_does_not_stop_the_others_and_does_not_fail_the_caller(t *testing.T) {
	failing_sink := &fake_log_sink{error_to_return: errors.New("first sink down")}
	working_sink := &fake_log_sink{}
	other_failing_sink := &fake_log_sink{error_to_return: errors.New("second sink down")}

	err := new_logger(failing_sink, working_sink, other_failing_sink).Error(context.Background(), dtos.LogEntryInboundDTO{Message: "m"})

	if err != nil {
		t.Errorf("a failing sink must not fail the caller, got %v", err)
	}
	if len(working_sink.received_entries) != 1 {
		t.Error("a failing sink must not stop the next sinks")
	}
}

func Test_every_sink_failure_is_reported_to_the_fallback_sink_without_the_dropped_content(t *testing.T) {
	fallback_sink := &fake_log_sink{}
	failing_sink := &fake_log_sink{error_to_return: errors.New("receiver unreachable")}
	other_failing_sink := &fake_log_sink{error_to_return: errors.New("disk full")}
	logger := new_logger_with_fallback(fallback_sink, failing_sink, other_failing_sink)

	err := logger.Warn(context.Background(), dtos.LogEntryInboundDTO{
		ProcessID: "process-id", Application: "app", Indentation: 2, Depth: 4,
		Message: "secret-looking message", Fields: map[string]any{"password": "hunter2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fallback_sink.received_entries) != 2 {
		t.Fatalf("the fallback sink must receive one report per failure, got %d", len(fallback_sink.received_entries))
	}
	report := fallback_sink.received_entries[0]
	if report.Level != "ERROR" || report.Category != "logger" || report.ProcessID != "process-id" || report.Application != "app" ||
		report.Indentation != 2 || report.Depth != 4 || !report.Timestamp.Equal(reference_time) {
		t.Errorf("report = %+v", report)
	}
	if report.Fields["error"] != "receiver unreachable" || report.Fields["dropped_level"] != "WARN" {
		t.Errorf("report fields = %v", report.Fields)
	}
	if fallback_sink.received_entries[1].Fields["error"] != "disk full" {
		t.Errorf("second report fields = %v", fallback_sink.received_entries[1].Fields)
	}
	for _, reported_entry := range fallback_sink.received_entries {
		if strings.Contains(reported_entry.Message, "secret") || len(reported_entry.Fields) != 2 {
			t.Errorf("a report must not repeat the dropped message or fields: %+v", reported_entry)
		}
	}
}

func Test_a_failing_fallback_sink_is_ignored(t *testing.T) {
	fallback_sink := &fake_log_sink{error_to_return: errors.New("console closed")}
	failing_sink := &fake_log_sink{error_to_return: errors.New("receiver unreachable")}

	err := new_logger_with_fallback(fallback_sink, failing_sink).Info(context.Background(), dtos.LogEntryInboundDTO{Message: "m"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(fallback_sink.received_entries) != 1 {
		t.Error("the failure must have been reported once")
	}
}

func Test_no_sinks_is_not_an_error(t *testing.T) {
	if err := new_logger().Info(context.Background(), dtos.LogEntryInboundDTO{Message: "m"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func Test_application_errors(t *testing.T) {
	if services.ErrLoggerEntryBuildFailed != application.ErrLoggerEntryBuildFailed {
		t.Error("the service must re-export the application sentinel")
	}

	joined := application.JoinErrors(application.ErrLoggerEntryBuildFailed, application.ErrLogSinkDeliveryFailed)
	if !errors.Is(joined, application.ErrLoggerEntryBuildFailed) || !errors.Is(joined, application.ErrLogSinkDeliveryFailed) {
		t.Error("JoinErrors lost a cause")
	}
	if application.ErrLoggerEntryBuildFailed.Error() == application.ErrLogSinkDeliveryFailed.Error() ||
		application.ErrLogSinkEndpointMissing.Error() == application.ErrLogSinkDeliveryFailed.Error() {
		t.Error("the sentinels must be distinct")
	}
}
