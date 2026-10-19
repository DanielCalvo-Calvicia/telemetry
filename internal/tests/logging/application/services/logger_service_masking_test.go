package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	ports_outbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
	services "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/services"
	operations "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/operations"
	value_objects "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

// The values below are fake: the AWS examples published in its documentation or obvious placeholders. The
// tests prove that none of them reaches a sink.
const (
	fake_access_key_id  = "AKIAIOSFODNN7EXAMPLE"
	fake_secret_key     = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	fake_secret_value   = "correct-horse-battery-staple"
	fake_message_body   = "customer 4711 ordered 3 units"
	fake_object_content = "confidential spreadsheet content"
	fake_bearer_token   = "abcdef0123456789"
	fake_db_password    = "hunter2"
)

func Test_realistic_aws_payloads_reach_the_sinks_without_sensitive_text(t *testing.T) {
	extra_masker, err := operations.NewSensitiveDataMasker([]string{`customer [0-9]+`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	log_sink := &fake_log_sink{}
	logger := services.NewLoggerService([]ports_outbound.LogSinkPort{log_sink}, nil, fake_clock{current_time: reference_time}, value_objects.LevelTrace, extra_masker)

	payloads := []struct {
		name         string
		message      string
		fields       map[string]any
		must_not_see []string
	}{
		{
			name:    "secrets manager create and get",
			message: "secret created",
			fields: map[string]any{
				"secret_name":   "prod/db",
				"secret_string": fake_secret_value,
				"response":      map[string]any{"SecretString": fake_secret_value, "ARN": "arn:aws:secretsmanager:eu-west-1:123456789012:secret:prod/db-AbCdEf"},
			},
			must_not_see: []string{fake_secret_value},
		},
		{
			name:    "sqs enqueue and dequeue",
			message: "message dequeued: " + fake_message_body,
			fields: map[string]any{
				"queue_url":    "https://sqs.eu-west-1.amazonaws.com/123456789012/orders",
				"message_body": fake_message_body,
				"messages":     []any{map[string]any{"Body": fake_message_body, "MessageId": "m-1"}},
			},
			must_not_see: []string{fake_message_body, "4711"},
		},
		{
			name:    "s3 upload and presign",
			message: "presigned https://b.s3.eu-west-1.amazonaws.com/k?X-Amz-Credential=" + fake_access_key_id + "&X-Amz-Signature=0123abcd",
			fields: map[string]any{
				"bucket": "b",
				"key":    "reports/2026.xlsx",
				"body":   fake_object_content,
				"error":  "access denied for aws_secret_access_key=" + fake_secret_key,
			},
			must_not_see: []string{fake_object_content, fake_access_key_id, fake_secret_key, "0123abcd"},
		},
		{
			name:    "cloudwatch logs add",
			message: "events added",
			fields: map[string]any{
				"log_group":  "/app/prod",
				"log_stream": "s-1",
				"events":     []any{map[string]any{"message": "login with password=" + fake_db_password + " and Authorization: Bearer " + fake_bearer_token}},
			},
			must_not_see: []string{fake_db_password, fake_bearer_token},
		},
	}

	for _, payload := range payloads {
		t.Run(payload.name, func(t *testing.T) {
			log_sink.received_entries = nil

			if err := logger.Info(context.Background(), dtos.LogEntryInboundDTO{Category: "service", Function: "Test", Message: payload.message, Fields: payload.fields}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(log_sink.received_entries) != 1 {
				t.Fatalf("the sink must receive one entry, got %d", len(log_sink.received_entries))
			}
			encoded_entry, err := json.Marshal(log_sink.received_entries[0])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, forbidden_text := range payload.must_not_see {
				if strings.Contains(string(encoded_entry), forbidden_text) {
					t.Errorf("the sink received %q: %s", forbidden_text, encoded_entry)
				}
			}
		})
	}
}

func Test_the_innocent_part_of_the_payload_is_kept_for_diagnosis(t *testing.T) {
	log_sink := &fake_log_sink{}
	logger := services.NewLoggerService([]ports_outbound.LogSinkPort{log_sink}, nil, fake_clock{current_time: reference_time}, value_objects.LevelTrace, new_masker())

	err := logger.Info(context.Background(), dtos.LogEntryInboundDTO{
		Message: "object stored", Fields: map[string]any{"bucket": "b", "key": "reports/2026.xlsx", "size": 1024, "secret_id": "prod/db", "body": fake_object_content},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent_fields := log_sink.received_entries[0].Fields
	if sent_fields["bucket"] != "b" || sent_fields["key"] != "reports/2026.xlsx" || sent_fields["size"] != 1024 || sent_fields["secret_id"] != "prod/db" {
		t.Errorf("sent fields = %v, the identifiers and sizes must be kept", sent_fields)
	}
	if sent_fields["body"] != "*****MASKED*****" || log_sink.received_entries[0].Message != "object stored" {
		t.Errorf("sent entry = %+v", log_sink.received_entries[0])
	}
}

func Test_the_callers_fields_are_not_modified_by_the_masking(t *testing.T) {
	log_sink := &fake_log_sink{}
	logger := services.NewLoggerService([]ports_outbound.LogSinkPort{log_sink}, nil, fake_clock{current_time: reference_time}, value_objects.LevelTrace, new_masker())
	callers_fields := map[string]any{"secret_string": fake_secret_value}

	if err := logger.Info(context.Background(), dtos.LogEntryInboundDTO{Message: "m", Fields: callers_fields}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callers_fields["secret_string"] != fake_secret_value {
		t.Error("the fields of the caller must not be modified")
	}
}

func Test_a_sink_failure_report_does_not_leak_sensitive_text_from_the_cause(t *testing.T) {
	fallback_sink := &fake_log_sink{}
	failing_sink := &fake_log_sink{error_to_return: errors.New("post failed for https://user:" + fake_db_password + "@receiver.local/logs")}
	logger := services.NewLoggerService([]ports_outbound.LogSinkPort{failing_sink}, fallback_sink, fake_clock{current_time: reference_time}, value_objects.LevelTrace, new_masker())

	if err := logger.Info(context.Background(), dtos.LogEntryInboundDTO{Message: "m"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reported_error, _ := fallback_sink.received_entries[0].Fields["error"].(string)
	if strings.Contains(reported_error, fake_db_password) || !strings.Contains(reported_error, "*****MASKED*****") {
		t.Errorf("reported error = %q", reported_error)
	}
}
