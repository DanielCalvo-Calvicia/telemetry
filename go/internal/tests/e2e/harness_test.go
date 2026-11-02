package e2e_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

// This file is the harness that the shared cases of the core and of the logging use: the setup that both kinds of case
// have (config, clock, event ids, sinks, call context) and the functions that build a core from it.

type core_send_result struct {
	Accepted int    `json:"accepted"`
	Dropped  int    `json:"dropped"`
	Failed   int    `json:"failed"`
	Reason   string `json:"reason"`
}

type core_sink struct {
	Result *core_send_result `json:"result"`
}

type core_call_context struct {
	TraceID       string `json:"trace_id"`
	SpanID        string `json:"span_id"`
	ExecutionID   string `json:"execution_id"`
	CorrelationID string `json:"correlation_id"`
}

// case_setup is what every shared case gives to build its core.
type case_setup struct {
	Config struct {
		Profile   string                     `json:"profile"`
		Overrides map[string]json.RawMessage `json:"overrides"`
	} `json:"config"`
	Clock struct {
		WallTime string `json:"wall_time"`
	} `json:"clock"`
	EventIDs    []string               `json:"event_ids"`
	Sinks       map[string][]core_sink `json:"sinks"`
	Spool       bool                   `json:"spool"`
	CallContext *core_call_context     `json:"call_context"`
}

// delivery_reader returns the events a fake sink received, flattened from its batches, as canonical JSON.
type delivery_reader interface {
	Batches() [][]telemetry.Envelope
}

func case_config(t *testing.T, profile string, overrides map[string]json.RawMessage) telemetry.Config {
	t.Helper()
	profiles := map[string]func() telemetry.Config{
		"microservice":   telemetry.Microservice,
		"cronjob":        telemetry.Cronjob,
		"worker":         telemetry.Worker,
		"robotics":       telemetry.Robotics,
		"offline_device": telemetry.OfflineDevice,
	}
	make_profile, known := profiles[profile]
	if !known {
		t.Fatalf("the profile %q is not known", profile)
	}
	base := make_profile()
	overrides_text, marshal_error := json.Marshal(overrides)
	if marshal_error != nil {
		t.Fatal(marshal_error)
	}
	if decode_error := json.Unmarshal(overrides_text, &base); decode_error != nil {
		t.Fatal(decode_error)
	}
	return base
}

func parse_instant(t *testing.T, text string) telemetry.UTCInstant {
	t.Helper()
	parsed_time, parse_error := time.Parse(time.RFC3339Nano, text)
	if parse_error != nil {
		t.Fatalf("the time %q: %v", text, parse_error)
	}
	instant, instant_error := telemetry.NewUTCInstant(parsed_time.UTC())
	if instant_error != nil {
		t.Fatal(instant_error)
	}
	return instant
}

// case_sinks builds the fake sinks of the case and returns the options that pass them, and the readers by port.
func case_sinks(t *testing.T, sinks map[string][]core_sink) ([]telemetry.Option, map[string][]delivery_reader) {
	t.Helper()
	var options []telemetry.Option
	readers := map[string][]delivery_reader{}
	for port, entries := range sinks {
		for _, entry := range entries {
			result := telemetry.SendResult{}
			if entry.Result != nil {
				result = telemetry.SendResult{Accepted: entry.Result.Accepted, Dropped: entry.Result.Dropped, Failed: entry.Result.Failed, Reason: entry.Result.Reason}
			}
			switch port {
			case "log":
				sink := &fakes.FakeLogSink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithLogSink(sink))
				readers[port] = append(readers[port], sink)
			case "event":
				sink := &fakes.FakeEventSink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithEventSink(sink))
				readers[port] = append(readers[port], sink)
			case "metrics":
				sink := &fakes.FakeMetricsSink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithMetricsSink(sink))
				readers[port] = append(readers[port], sink)
			case "trace":
				sink := &fakes.FakeTraceSink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithTraceSink(sink))
				readers[port] = append(readers[port], sink)
			case "device_telemetry":
				sink := &fakes.FakeDeviceTelemetrySink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithDeviceTelemetrySink(sink))
				readers[port] = append(readers[port], sink)
			case "ai_telemetry":
				sink := &fakes.FakeAITelemetrySink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithAITelemetrySink(sink))
				readers[port] = append(readers[port], sink)
			default:
				t.Fatalf("the sink port %q is not known", port)
			}
		}
	}
	return options, readers
}

// build_case_core builds the core of the setup, with the fake sinks, the fake clock and the fake id generator. It
// returns the readers of the sinks by port and the error of New, which the caller checks.
func build_case_core(t *testing.T, setup case_setup) (*telemetry.Telemetry, map[string][]delivery_reader, error) {
	t.Helper()
	event_ids := make([]telemetry.EventID, 0, len(setup.EventIDs))
	for _, event_id_text := range setup.EventIDs {
		event_id, id_error := telemetry.NewEventID(event_id_text)
		if id_error != nil {
			t.Fatal(id_error)
		}
		event_ids = append(event_ids, event_id)
	}
	options, readers := case_sinks(t, setup.Sinks)
	options = append(options,
		telemetry.WithClock(fakes.NewFakeClock(parse_instant(t, setup.Clock.WallTime))),
		telemetry.WithIDGenerator(fakes.NewFakeIDGenerator(event_ids...)),
	)
	if setup.Spool {
		options = append(options, telemetry.WithLocalSpool(&fakes.FakeLocalSpool{}))
	}
	core, new_error := telemetry.New(case_config(t, setup.Config.Profile, setup.Config.Overrides), options...)
	return core, readers, new_error
}

// case_context returns a context that carries the call context of the setup, when it has one.
func case_context(t *testing.T, setup case_setup) context.Context {
	t.Helper()
	ctx := context.Background()
	if setup.CallContext == nil {
		return ctx
	}
	call_context, context_error := telemetry.NewCallContext(
		telemetry.WithTraceID(setup.CallContext.TraceID),
		telemetry.WithSpanID(setup.CallContext.SpanID),
		telemetry.WithExecutionID(setup.CallContext.ExecutionID),
		telemetry.WithCorrelationID(setup.CallContext.CorrelationID),
	)
	if context_error != nil {
		t.Fatal(context_error)
	}
	return telemetry.WithCallContext(ctx, call_context)
}

// canonical_events returns what a sink received, in order, as canonical JSON (never nil).
func canonical_events(t *testing.T, reader delivery_reader) []string {
	t.Helper()
	got_events := []string{}
	for _, batch := range reader.Batches() {
		for _, envelope := range batch {
			canonical_text, canonical_error := canonical_json.CanonicalJSON(envelope)
			if canonical_error != nil {
				t.Fatal(canonical_error)
			}
			got_events = append(got_events, string(canonical_text))
		}
	}
	return got_events
}
