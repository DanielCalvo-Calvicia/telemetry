package operations_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain"
	entities "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/entities"
	operations "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/operations"
	value_objects "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

func new_masker(t *testing.T, extra_patterns ...string) operations.SensitiveDataMasker {
	t.Helper()
	masker, err := operations.NewSensitiveDataMasker(extra_patterns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return masker
}

// =============================================================================
// SHAPE 1: A SINGLE VALUE (by the name of its field, or by what the value is)
// =============================================================================

func Test_a_value_is_hidden_by_the_name_of_its_field_whatever_it_holds(t *testing.T) {
	masker := new_masker(t)
	test_cases := []struct {
		name       string
		field_name string
		value      any
	}{
		{"text", "password", "hunter2"},
		{"text of a secret value", "secret_string", "plain text with no pattern at all"},
		{"bytes", "secret_binary", []byte{1, 2, 3}},
		{"number", "token", 123456},
		{"map", "credentials", map[string]any{"user": "bob"}},
		{"list", "api_key", []any{"a", "b"}},
		{"body of an object", "body", "some file content"},
		{"body of a message", "message_body", `{"order": 17}`},
		{"camel case name", "accessKey", "whatever"},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			if masked_value := masker.MaskValue(test_case.field_name, test_case.value); masked_value != domain.MaskedText {
				t.Errorf("MaskValue(%q, %v) = %v, want the mask", test_case.field_name, test_case.value, masked_value)
			}
		})
	}
}

func Test_a_value_is_hidden_by_what_it_is_even_in_an_innocent_field(t *testing.T) {
	masker := new_masker(t)
	test_cases := []struct {
		name  string
		value string
	}{
		{"access key", fake_access_key_id},
		{"jwt", fake_jwt},
		{"private key", fake_private_key},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			if masked_value := masker.MaskValue("detail", test_case.value); masked_value != domain.MaskedText {
				t.Errorf("MaskValue() = %v, want the mask", masked_value)
			}
		})
	}
}

func Test_innocent_values_are_kept(t *testing.T) {
	masker := new_masker(t)
	moment := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

	test_cases := []struct {
		name       string
		field_name string
		value      any
	}{
		{"nil", "anything", nil},
		{"text", "bucket", "my-bucket"},
		{"secret id", "secret_id", "prod/db/credentials"},
		{"int", "count", 42},
		{"int64", "size", int64(1024)},
		{"uint", "length", uint(7)},
		{"float", "ratio", 0.5},
		{"bool", "found", true},
		{"time", "created_at", moment},
		{"duration", "elapsed", 3 * time.Second},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			if masked_value := masker.MaskValue(test_case.field_name, test_case.value); masked_value != test_case.value {
				t.Errorf("MaskValue() = %v, want %v unchanged", masked_value, test_case.value)
			}
		})
	}
}

// =============================================================================
// SHAPE 2 AND 3: A WHOLE TEXT, AND A SEGMENT INSIDE A LONGER TEXT
// =============================================================================

func Test_a_whole_text_that_is_sensitive_is_hidden(t *testing.T) {
	masker := new_masker(t)

	for _, sensitive_text := range []string{fake_access_key_id, fake_jwt, fake_private_key} {
		if masked_text := masker.MaskText(sensitive_text); masked_text != domain.MaskedText {
			t.Errorf("MaskText(%q) = %q, want exactly the mask", sensitive_text, masked_text)
		}
	}
}

func Test_a_sensitive_segment_inside_a_longer_text_is_hidden_and_the_rest_is_kept(t *testing.T) {
	masker := new_masker(t)
	test_cases := []struct {
		name     string
		text     string
		expected string
	}{
		{"password pair", "login failed for user=bob password=hunter2 from 10.0.0.1", "login failed for user=bob password=*****MASKED***** from 10.0.0.1"},
		{"connection string", "cannot connect to postgres://admin:hunter2@db.local:5432/app: timeout", "cannot connect to postgres://admin:*****MASKED*****@db.local:5432/app: timeout"},
		{"bearer", "request with Authorization: Bearer abcdef123456 failed", "request with Authorization: Bearer *****MASKED***** failed"},
		{"access key", "using " + fake_access_key_id + " in eu-west-1", "using *****MASKED***** in eu-west-1"},
		{"secret key pair", "aws_secret_access_key=" + fake_secret_key + " set", "aws_secret_access_key=*****MASKED***** set"},
		{"jwt", "got " + fake_jwt + " from the client", "got *****MASKED***** from the client"},
		{"private key block", "key: " + fake_private_key + "\nend", "key: *****MASKED*****\nend"},
		{"presigned url", "https://bucket.s3.eu-west-1.amazonaws.com/file?X-Amz-Credential=" + fake_access_key_id + "%2F20261007%2Feu-west-1%2Fs3%2Faws4_request&X-Amz-Signature=0123456789abcdef&X-Amz-Expires=300",
			"https://bucket.s3.eu-west-1.amazonaws.com/file?X-Amz-Credential=*****MASKED*****&X-Amz-Signature=*****MASKED*****&X-Amz-Expires=300"},
		{"several in one text", "password=a token=b", "password=*****MASKED***** token=*****MASKED*****"},
		{"nothing sensitive", "object copied from bucket a to bucket b", "object copied from bucket a to bucket b"},
		{"empty", "", ""},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			if masked_text := masker.MaskText(test_case.text); masked_text != test_case.expected {
				t.Errorf("MaskText() = %q, want %q", masked_text, test_case.expected)
			}
		})
	}
}

func Test_masking_a_masked_text_changes_nothing(t *testing.T) {
	masker := new_masker(t, `order-[0-9]+`)
	text := "password=hunter2 Authorization: Bearer abcdef123456 " + fake_access_key_id + " order-123 postgres://u:p@h/db"

	once := masker.MaskText(text)
	twice := masker.MaskText(once)

	if once != twice {
		t.Errorf("masking is not idempotent:\n once  = %q\n twice = %q", once, twice)
	}
}

// =============================================================================
// EXTRA PATTERNS FROM THE CONFIGURATION
// =============================================================================

func Test_extra_patterns_hide_every_match_whole(t *testing.T) {
	masker := new_masker(t, `order-[0-9]{2,5}`, `employee:[a-z]+`)

	masked_text := masker.MaskText("shipped order-12345 for employee:bob and order-7")

	if masked_text != "shipped *****MASKED***** for *****MASKED***** and order-7" {
		t.Errorf("MaskText() = %q", masked_text)
	}
}

func Test_an_empty_extra_pattern_is_ignored_and_does_not_hide_everything(t *testing.T) {
	masker := new_masker(t, "", "never-matches-[0-9]+")

	if masked_text := masker.MaskText("hello world"); masked_text != "hello world" {
		t.Errorf("MaskText() = %q, want the text unchanged", masked_text)
	}
}

func Test_an_invalid_extra_pattern_is_an_error_that_keeps_the_cause(t *testing.T) {
	masker, err := operations.NewSensitiveDataMasker([]string{`valid-[0-9]+`, `broken(`})

	if !errors.Is(err, operations.ErrMaskPatternInvalid) || !errors.Is(err, domain.ErrMaskPatternInvalid) {
		t.Errorf("error = %v, want ErrMaskPatternInvalid", err)
	}
	if err != nil && !strings.Contains(err.Error(), "broken(") {
		t.Errorf("the error must say which pattern failed (through the compile error): %v", err)
	}
	if masker.MaskText("password=x") != "password=x" {
		t.Error("a failed masker must be the zero masker and hide nothing by text")
	}
}

// =============================================================================
// CONTAINERS, ERRORS AND STRUCTURES
// =============================================================================

type fake_credentials struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Note     string `json:"note"`
}

func Test_containers_are_masked_element_by_element(t *testing.T) {
	masker := new_masker(t)

	masked_value := masker.MaskValue("details", map[string]any{
		"bucket":     "my-bucket",
		"password":   "hunter2",
		"error":      "failed with token=abc123",
		"nested":     map[string]any{"api_key": "k", "count": 3},
		"list":       []any{"password=x", 5, map[string]any{"secret": "y"}},
		"texts":      []string{"safe", "password=z"},
		"typed_maps": map[string]string{"auth_token": "t", "name": "n"},
	})

	masked_map, is_map := masked_value.(map[string]any)
	if !is_map {
		t.Fatalf("MaskValue() = %T, want a map", masked_value)
	}
	if masked_map["bucket"] != "my-bucket" || masked_map["password"] != domain.MaskedText || masked_map["error"] != "failed with token=*****MASKED*****" {
		t.Errorf("top level = %v", masked_map)
	}
	if nested := masked_map["nested"].(map[string]any); nested["api_key"] != domain.MaskedText || nested["count"] != 3 {
		t.Errorf("nested = %v", nested)
	}
	masked_list := masked_map["list"].([]any)
	if masked_list[0] != "password=*****MASKED*****" || masked_list[1] != 5 || masked_list[2].(map[string]any)["secret"] != domain.MaskedText {
		t.Errorf("list = %v", masked_list)
	}
	if masked_texts := masked_map["texts"].([]string); masked_texts[0] != "safe" || masked_texts[1] != "password=*****MASKED*****" {
		t.Errorf("texts = %v", masked_texts)
	}
	if typed_map := masked_map["typed_maps"].(map[string]any); typed_map["auth_token"] != domain.MaskedText || typed_map["name"] != "n" {
		t.Errorf("typed map = %v", typed_map)
	}
}

func Test_an_error_value_is_masked_as_text(t *testing.T) {
	masker := new_masker(t)

	masked_value := masker.MaskValue("cause", errors.New("dial postgres://u:hunter2@h/db failed"))

	if masked_value != "dial postgres://u:*****MASKED*****@h/db failed" {
		t.Errorf("MaskValue() = %v", masked_value)
	}
}

func Test_structures_and_pointers_are_masked_through_their_json_form(t *testing.T) {
	masker := new_masker(t)

	masked_value := masker.MaskValue("request", &fake_credentials{User: "bob", Password: "hunter2", Note: "token=abc"})

	masked_map, is_map := masked_value.(map[string]any)
	if !is_map {
		t.Fatalf("MaskValue() = %T, want a map", masked_value)
	}
	if masked_map["user"] != "bob" || masked_map["password"] != domain.MaskedText || masked_map["note"] != "token=*****MASKED*****" {
		t.Errorf("masked structure = %v", masked_map)
	}
}

func Test_a_value_that_cannot_be_inspected_is_hidden(t *testing.T) {
	masker := new_masker(t)

	for _, uninspectable_value := range []any{func() {}, make(chan int)} {
		if masked_value := masker.MaskValue("callback", uninspectable_value); masked_value != domain.MaskedText {
			t.Errorf("MaskValue(%T) = %v, want the mask", uninspectable_value, masked_value)
		}
	}
}

// =============================================================================
// FIELDS AND LOG ENTRIES (the originals are never modified)
// =============================================================================

func Test_mask_fields_returns_a_copy_and_keeps_the_original(t *testing.T) {
	masker := new_masker(t)
	original_fields := value_objects.Fields{"password": "hunter2", "bucket": "b", "nested": map[string]any{"token": "t"}}

	masked_fields := masker.MaskFields(original_fields)

	if masked_fields["password"] != domain.MaskedText || masked_fields["bucket"] != "b" {
		t.Errorf("masked fields = %v", masked_fields)
	}
	if original_fields["password"] != "hunter2" || original_fields["nested"].(map[string]any)["token"] != "t" {
		t.Errorf("the original fields were modified: %v", original_fields)
	}
	if masker.MaskFields(nil) != nil {
		t.Error("nil fields must stay nil")
	}
}

func Test_mask_log_entry_hides_the_message_and_the_fields_without_touching_the_original(t *testing.T) {
	masker := new_masker(t)
	original_entry, err := entities.NewLogEntry("producer", "application-id", "process-id", "app", "service", "Put", 2, 3, "INFO",
		"connecting with password=hunter2", map[string]any{"secret_string": "plain", "bucket": "b"}, time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	masked_entry := masker.MaskLogEntry(original_entry)

	if masked_entry.Message.String() != "connecting with password=*****MASKED*****" {
		t.Errorf("masked message = %q", masked_entry.Message)
	}
	if masked_entry.Fields["secret_string"] != domain.MaskedText || masked_entry.Fields["bucket"] != "b" {
		t.Errorf("masked fields = %v", masked_entry.Fields)
	}
	if masked_entry.Identifiers != original_entry.Identifiers || masked_entry.Level != original_entry.Level ||
		masked_entry.Location != original_entry.Location || masked_entry.Indentation != original_entry.Indentation ||
		!masked_entry.Timestamp.Equal(original_entry.Timestamp) {
		t.Error("everything but the message and the fields must be kept")
	}
	if original_entry.Message.String() != "connecting with password=hunter2" || original_entry.Fields["secret_string"] != "plain" {
		t.Errorf("the original entry was modified: %q %v", original_entry.Message, original_entry.Fields)
	}
}
