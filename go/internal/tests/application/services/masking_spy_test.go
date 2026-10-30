package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/event_validation"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

// All the credentials below are fake: the AWS examples published in its documentation or obvious placeholders.
const (
	spy_access_key_id = "AKIAIOSFODNN7EXAMPLE"
	spy_jwt           = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.c2lnbmF0dXJl"
	spy_pem_body      = "FAKEFAKEFAKEFAKE"
	spy_pem           = "-----BEGIN RSA PRIVATE KEY-----\n" + spy_pem_body + "\n-----END RSA PRIVATE KEY-----"
	spy_password      = "hunter2"
	spy_bearer_value  = "abcdef123456"
	spy_plain_text    = "connection retried twice"
)

func spy_secrets() []string {
	return []string{spy_access_key_id, spy_jwt, spy_pem_body, spy_password, spy_bearer_value}
}

func spy_masker(test *testing.T, options masking.MaskerOptions) masking.SensitiveDataMasker {
	test.Helper()
	masker, masker_error := masking.NewSensitiveDataMasker(options)
	if masker_error != nil {
		test.Fatal(masker_error)
	}
	return masker
}

// spy_sink is one of the sink fakes, seen through the two methods that the chain and the assertions need.
type spy_sink struct {
	name    string
	send    func(context.Context, []value_objects.Envelope) dtos.SendResult
	batches func() [][]value_objects.Envelope
}

func spy_sinks() []spy_sink {
	log_sink, event_sink, metrics_sink := &fakes.FakeLogSink{}, &fakes.FakeEventSink{}, &fakes.FakeMetricsSink{}
	trace_sink, device_sink, ai_sink := &fakes.FakeTraceSink{}, &fakes.FakeDeviceTelemetrySink{}, &fakes.FakeAITelemetrySink{}
	return []spy_sink{
		{"log", log_sink.Send, log_sink.Batches},
		{"event", event_sink.Send, event_sink.Batches},
		{"metrics", metrics_sink.Send, metrics_sink.Batches},
		{"trace", trace_sink.Send, trace_sink.Batches},
		{"device_telemetry", device_sink.Send, device_sink.Batches},
		{"ai_telemetry", ai_sink.Send, ai_sink.Batches},
	}
}

// secret_input is an event that carries every kind of secret in the message, in the attributes and in the context.
func secret_input(test *testing.T) dtos.NewEventInput {
	test.Helper()
	input := log_input(map[string]any{
		"category": "http",
		"message": spy_plain_text + " key " + spy_access_key_id + " jwt " + spy_jwt + " pem " + spy_pem +
			" url postgres://admin:" + spy_password + "@db:5432/app header Authorization: Bearer " + spy_bearer_value +
			" pair password=" + spy_password + " end",
	})
	attributes, attributes_error := value_objects.NewEventAttributes(map[string]any{
		"note":      "see " + spy_access_key_id + " and " + spy_jwt,
		"api_key":   spy_password,
		"tags":      []any{"Bearer " + spy_bearer_value, spy_plain_text},
		"plain":     spy_plain_text,
		"pem_field": spy_pem,
	})
	if attributes_error != nil {
		test.Fatal(attributes_error)
	}
	correlation_context, context_error := value_objects.NewCorrelationContext(value_objects.WithCorrelationID(spy_jwt))
	if context_error != nil {
		test.Fatal(context_error)
	}
	input.Attributes = &attributes
	input.Context = &correlation_context
	return input
}

func Test_no_sink_ever_receives_a_secret_through_the_chain_builder_masking_sink(t *testing.T) {
	for _, sink := range spy_sinks() {
		t.Run(sink.name, func(t *testing.T) {
			builder := new_test_services(t, event_validation.ValidateEvent).service
			masking_service := services.NewMaskingService(spy_masker(t, masking.MaskerOptions{}))

			built_envelope, build_error := builder.Build(context.Background(), secret_input(t))
			if build_error != nil {
				t.Fatal(build_error)
			}
			masked_envelope, mask_error := masking_service.Mask(context.Background(), built_envelope)
			if mask_error != nil {
				t.Fatal(mask_error)
			}
			sink.send(context.Background(), []value_objects.Envelope{masked_envelope})

			batches := sink.batches()
			if len(batches) != 1 || len(batches[0]) != 1 {
				t.Fatalf("batches = %v, want one batch of one event", batches)
			}
			sent_text := canonical_text(t, batches[0][0])
			for _, secret := range spy_secrets() {
				if strings.Contains(sent_text, secret) {
					t.Errorf("the sink received the secret %q: %s", secret, sent_text)
				}
			}
			if !strings.Contains(sent_text, spy_plain_text) || !strings.Contains(sent_text, domain.MaskedText) {
				t.Errorf("the text that is not secret must survive, and the mask must be there: %s", sent_text)
			}
			if !strings.Contains(canonical_text(t, built_envelope), spy_password) {
				t.Error("the test is not proving anything: the built envelope must hold the secret before masking")
			}
		})
	}
}

// =============================================================================
// THE ENVELOPE THAT MASK RECEIVES IS NEVER MODIFIED; WHAT IT RETURNS IS A COPY
// =============================================================================

func nested_envelope() value_objects.Envelope {
	return value_objects.Envelope{
		SchemaVersion: domain.SchemaVersion,
		EventType:     "log.entry.recorded",
		Data: map[string]any{
			"category": "c",
			"message":  "password=hunter2",
			"nested":   map[string]any{"token": "t", "list": []any{"Bearer abcdef123456", map[string]any{"secret": "s"}}},
		},
		Attributes: value_objects.Attributes{"api_key": "k", "inner": []any{"password=x"}},
		Context:    &value_objects.Context{TraceId: "0af7651916cd43dd8448eb211c80319c", CorrelationId: "token=abc12345678"},
		Actor:      &value_objects.Actor{Id: "a-1", Kind: value_objects.ActorKindDevice},
	}
}

func deep_copy_by_json(test *testing.T, envelope value_objects.Envelope) value_objects.Envelope {
	test.Helper()
	encoded, encode_error := json.Marshal(envelope)
	if encode_error != nil {
		test.Fatal(encode_error)
	}
	var copied value_objects.Envelope
	if decode_error := json.Unmarshal(encoded, &copied); decode_error != nil {
		test.Fatal(decode_error)
	}
	return copied
}

func Test_mask_never_modifies_the_envelope_it_receives_and_returns_copies(t *testing.T) {
	service := services.NewMaskingService(spy_masker(t, masking.MaskerOptions{}))
	original := nested_envelope()
	before := deep_copy_by_json(t, original)

	masked_envelope, mask_error := service.Mask(context.Background(), original)
	if mask_error != nil {
		t.Fatal(mask_error)
	}

	if !reflect.DeepEqual(before, deep_copy_by_json(t, original)) {
		t.Error("the envelope that Mask received was modified")
	}
	masked_envelope.Data["nested"].(map[string]any)["added"] = "x"
	masked_envelope.Attributes["added"] = "x"
	masked_envelope.Context.TraceId = "changed"
	masked_envelope.Actor.Id = "changed"
	if !reflect.DeepEqual(before, deep_copy_by_json(t, original)) {
		t.Error("the result shares data, attributes, context or actor with the input")
	}
	if masked_envelope.Context == original.Context || masked_envelope.Actor == original.Actor {
		t.Error("the context and the actor of the result must be copies")
	}
}

func Test_mask_changes_only_data_attributes_and_context(t *testing.T) {
	service := services.NewMaskingService(spy_masker(t, masking.MaskerOptions{}))
	original := nested_envelope()
	original.EventId, original.OccurredAt, original.RecordedAt = "id", "occurred", "recorded"
	original.Severity = value_objects.SeverityError
	original.Resource = value_objects.Resource{ServiceName: "svc", InstanceId: "i"}

	masked_envelope, _ := service.Mask(context.Background(), original)

	if masked_envelope.EventId != "id" || masked_envelope.OccurredAt != "occurred" || masked_envelope.RecordedAt != "recorded" ||
		masked_envelope.EventType != original.EventType || masked_envelope.SchemaVersion != original.SchemaVersion ||
		masked_envelope.Severity != original.Severity || masked_envelope.Resource != original.Resource || *masked_envelope.Actor != *original.Actor {
		t.Errorf("a field that Mask must not change was changed: %+v", masked_envelope)
	}
	if masked_envelope.Data["message"] != "password=*****MASKED*****" || masked_envelope.Attributes["api_key"] != domain.MaskedText ||
		masked_envelope.Context.CorrelationId != "token=*****MASKED*****" || masked_envelope.Context.TraceId != original.Context.TraceId {
		t.Errorf("data, attributes or context were not masked: %+v", masked_envelope)
	}
}

func Test_mask_keeps_a_nil_data_attributes_context_and_actor(t *testing.T) {
	service := services.NewMaskingService(spy_masker(t, masking.MaskerOptions{}))

	masked_envelope, mask_error := service.Mask(context.Background(), value_objects.Envelope{EventType: "x"})

	if mask_error != nil || masked_envelope.Data != nil || masked_envelope.Attributes != nil || masked_envelope.Context != nil || masked_envelope.Actor != nil {
		t.Errorf("masked = %+v, error = %v", masked_envelope, mask_error)
	}
}

func Test_masking_an_envelope_twice_gives_the_same_bytes_as_once(t *testing.T) {
	service := services.NewMaskingService(spy_masker(t, masking.MaskerOptions{ExtraPatterns: []string{`order-[0-9]+`}}))
	once, _ := service.Mask(context.Background(), nested_envelope())
	twice, _ := service.Mask(context.Background(), once)

	if canonical_text(t, once) != canonical_text(t, twice) {
		t.Errorf("not idempotent:\n once  %s\n twice %s", canonical_text(t, once), canonical_text(t, twice))
	}
}

func Test_the_options_reach_data_attributes_and_context(t *testing.T) {
	service := services.NewMaskingService(spy_masker(t, masking.MaskerOptions{
		ExtraFieldNames: []string{"trace_id"},
		FieldClasses:    map[string]string{"drop_me": "pii"},
		ClassRules:      map[string]string{"pii": domain.MaskRuleDrop},
	}))
	envelope := nested_envelope()
	envelope.Data["drop_me"] = "x"
	envelope.Attributes["drop_me"] = "x"

	masked_envelope, _ := service.Mask(context.Background(), envelope)

	if _, found := masked_envelope.Data["drop_me"]; found {
		t.Error("data must drop the field")
	}
	if _, found := masked_envelope.Attributes["drop_me"]; found {
		t.Error("attributes must drop the field")
	}
	if masked_envelope.Context.TraceId != domain.MaskedText {
		t.Errorf("trace_id = %q, want the mask (extra field name)", masked_envelope.Context.TraceId)
	}
}

// =============================================================================
// A FAILURE NEVER FAILS THE CALLER AND NEVER LEAKS THE ENVELOPE (ADR-005, ADR-006)
// =============================================================================

type panicking_value struct{}

func (panicking_value) MarshalJSON() ([]byte, error) { panic("the value cannot be read") }

func assert_masking_failed(test *testing.T, masked_envelope value_objects.Envelope, mask_error error) {
	test.Helper()
	if !errors.Is(mask_error, application.ErrEnvelopeMaskingFailed) {
		test.Errorf("error = %v, want ErrEnvelopeMaskingFailed", mask_error)
	}
	if !reflect.DeepEqual(masked_envelope, value_objects.Envelope{}) {
		test.Errorf("a failed Mask must return an empty envelope, got %+v", masked_envelope)
	}
}

func Test_a_panic_while_masking_becomes_an_error_and_an_empty_envelope(t *testing.T) {
	service := services.NewMaskingService(spy_masker(t, masking.MaskerOptions{}))
	envelope := nested_envelope()
	envelope.Data["boom"] = panicking_value{}

	masked_envelope, mask_error := service.Mask(context.Background(), envelope)

	assert_masking_failed(t, masked_envelope, mask_error)
	if mask_error != nil && strings.Contains(mask_error.Error(), "cannot be read") {
		t.Error("the panic value must not be kept in the error")
	}
}

func Test_a_service_that_cannot_run_returns_the_error_and_not_the_envelope(t *testing.T) {
	var broken_service *services.MaskingService

	masked_envelope, mask_error := broken_service.Mask(context.Background(), nested_envelope())

	assert_masking_failed(t, masked_envelope, mask_error)
}

// =============================================================================
// CONCURRENCY
// =============================================================================

func Test_the_service_masks_from_eight_goroutines(t *testing.T) {
	service := services.NewMaskingService(spy_masker(t, masking.MaskerOptions{}))
	shared := nested_envelope()

	var wait_group sync.WaitGroup
	results := make(chan string, 8)
	for worker := 0; worker < 8; worker++ {
		wait_group.Add(1)
		go func() {
			defer wait_group.Done()
			masked_envelope, _ := service.Mask(context.Background(), shared)
			bytes, _ := json.Marshal(masked_envelope)
			results <- string(bytes)
		}()
	}
	wait_group.Wait()
	close(results)

	var first string
	for result := range results {
		if first == "" {
			first = result
		}
		if result != first || strings.Contains(result, "hunter2") {
			t.Fatalf("concurrent results differ or leak: %s", result)
		}
	}
}
