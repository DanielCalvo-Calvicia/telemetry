package e2e_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

func new_logging_core(t *testing.T) (*telemetry.Telemetry, *fakes.FakeLogSink) {
	t.Helper()
	sink := &fakes.FakeLogSink{}
	config := fast_delivery(telemetry.Microservice())
	config.Resource.ServiceName = "sensor-hub"
	core, new_error := telemetry.New(config, telemetry.WithLogSink(sink))
	if new_error != nil {
		t.Fatal(new_error)
	}
	return core, sink
}

func Test_a_field_named_password_never_reaches_the_sink(t *testing.T) {
	core, sink := new_logging_core(t)

	core.Logger().Info(context.Background(), telemetry.LogEntry{
		Category: "auth",
		Message:  "login",
		Fields:   map[string]any{"password": "hunter2-secret", "nested": map[string]any{"password": "hunter2-secret"}},
	})
	slog.New(telemetry.NewSlogHandler(core.Logger(), "auth")).Info("login", "password", "hunter2-secret")

	waiting.Eventually(t, waiting.DefaultTimeout, "the sink receives both events", func() bool { return count_events(sink) == 2 })
	waiting.StaysTrue(t, 30*time.Millisecond, "the sink receives no third event", func() bool { return count_events(sink) == 2 })
	events := canonical_events(t, sink)
	if len(events) != 2 {
		t.Fatalf("%d events, want 2", len(events))
	}
	for _, event := range events {
		if strings.Contains(event, "hunter2-secret") {
			t.Errorf("a secret reached the sink: %s", event)
		}
	}
}

func Test_no_request_body_is_captured_by_default(t *testing.T) {
	core, sink := new_logging_core(t)

	core.Logger().Info(context.Background(), telemetry.LogEntry{Category: "http", Message: "request done"})

	waiting.Eventually(t, waiting.DefaultTimeout, "the sink receives the event", func() bool { return count_events(sink) == 1 })
	batches := sink.Batches()
	if len(batches) != 1 || len(batches[0]) != 1 {
		t.Fatalf("batches %v", batches)
	}
	envelope := batches[0][0]
	if len(envelope.Data) != 2 {
		t.Errorf("data %v, want exactly category and message", envelope.Data)
	}
	for _, key := range []string{"category", "message"} {
		if _, has_key := envelope.Data[key]; !has_key {
			t.Errorf("data has no %s", key)
		}
	}
	if len(envelope.Attributes) != 0 {
		t.Errorf("attributes %v, want none apart from the clock ones", envelope.Attributes)
	}
}

func Test_a_nil_core_and_an_empty_core_give_a_logger_that_does_nothing(t *testing.T) {
	var nil_core *telemetry.Telemetry
	nil_core.Logger().Critical(context.Background(), telemetry.LogEntry{Category: "c", Message: "m"})

	empty_core := &telemetry.Telemetry{}
	if empty_core.Logger() == nil {
		t.Fatal("the logger of a core must never be nil")
	}
	empty_core.Logger().Error(context.Background(), telemetry.LogEntry{Category: "c", Message: "m"})
}

func Test_a_log_call_with_no_sink_does_not_fail_the_caller(t *testing.T) {
	config := telemetry.Microservice()
	config.Resource.ServiceName = "sensor-hub"
	core, new_error := telemetry.New(config)
	if new_error != nil {
		t.Fatal(new_error)
	}
	core.Logger().Info(context.Background(), telemetry.LogEntry{Category: "c", Message: "m"})
}
