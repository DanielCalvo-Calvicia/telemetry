package domain_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/event_validation"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/limits"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

// The shared table of the codes of the errors of the core: integration-tests/fixtures/errors/codes.json. Each case is
// a failure that the core produces; the producer of the same name in this file makes it through the real code.

const (
	error_codes_table_path = "../../../../integration-tests/fixtures/errors/codes.json"
	error_catalog_path     = "../../../../contracts/errors/catalog.yaml"
)

type error_codes_case struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ExpectedCode string `json:"expected_code"`
	ValueInInput bool   `json:"value_in_input"`
}

type error_codes_table struct {
	Description string             `json:"description"`
	Secret      string             `json:"secret"`
	Cases       []error_codes_case `json:"cases"`
}

func load_error_codes_table(t *testing.T) error_codes_table {
	t.Helper()
	content, read_error := os.ReadFile(error_codes_table_path)
	if read_error != nil {
		t.Fatalf("the shared table of the error codes cannot be read: %v", read_error)
	}
	var table error_codes_table
	if decode_error := json.Unmarshal(content, &table); decode_error != nil {
		t.Fatalf("the shared table of the error codes is not valid JSON: %v", decode_error)
	}
	if table.Secret == "" || len(table.Cases) == 0 {
		t.Fatal("the shared table of the error codes has no secret or no cases")
	}
	return table
}

// error_only drops the first result of a call and keeps its error.
func error_only[Result any](_ Result, call_error error) error {
	return call_error
}

func producer_envelope(message string) value_objects.Envelope {
	return value_objects.Envelope{
		SchemaVersion: "0.1",
		EventId:       "01900000-0000-7000-8000-000000000001",
		EventType:     "log.entry.recorded",
		OccurredAt:    "2026-10-28T10:00:00.123456Z",
		RecordedAt:    "2026-10-28T10:00:00.123456Z",
		Severity:      "INFO",
		Resource:      value_objects.Resource{ServiceName: "sensor-hub", InstanceId: "hub-01"},
		Data:          map[string]any{"category": "c", "message": message},
	}
}

func producer_config() config.Config {
	valid := config.Microservice()
	valid.Resource.ServiceName = "sensor-hub"
	return valid
}

// producers has one function per case of the table. Each one makes the REAL failure through the real code and returns
// its error; the secret is the value of the caller where the case says so.
var producers = map[string]func(secret string) error{
	"event_id_invalid": func(secret string) error {
		return error_only(value_objects.NewEventID(secret))
	},
	"severity_invalid": func(secret string) error {
		return error_only(value_objects.NewSeverity(secret))
	},
	"actor_kind_invalid": func(secret string) error {
		return error_only(value_objects.NewActorKind(secret))
	},
	"identifier_blank": func(_ string) error {
		return error_only(base_value_objects.NewIdentifier("   "))
	},
	"identifier_control_characters": func(secret string) error {
		return error_only(base_value_objects.NewIdentifier(secret + "\n"))
	},
	"hex_digits_invalid": func(secret string) error {
		return error_only(base_value_objects.NewHexDigits(secret, 8))
	},
	"time_zone_invalid": func(secret string) error {
		return value_objects.ValidateTimeZone(secret + " x")
	},
	"utc_instant_not_utc": func(_ string) error {
		return error_only(value_objects.NewUTCInstant(time.Date(2026, 10, 28, 10, 0, 0, 0, time.FixedZone("plus_two", 2*60*60))))
	},
	"utc_instant_text_invalid": func(secret string) error {
		return error_only(value_objects.ParseUTCInstant(secret))
	},
	"attribute_key_invalid": func(_ string) error {
		return error_only(value_objects.NewEventAttributes(map[string]any{"": "v"}))
	},
	"attribute_value_invalid": func(secret string) error {
		return error_only(value_objects.NewEventAttributes(map[string]any{"k": map[string]any{"nested": secret}}))
	},
	"resource_identity_invalid": func(secret string) error {
		return error_only(value_objects.NewResourceIdentity("sensor-hub", "hub-01", value_objects.WithEnvironment(secret+"\n")))
	},
	"actor_identity_invalid": func(secret string) error {
		return error_only(value_objects.NewActorIdentity(value_objects.ActorKindDevice, secret+"\n"))
	},
	"correlation_context_invalid": func(secret string) error {
		return error_only(value_objects.NewCorrelationContext(value_objects.WithTraceID(secret)))
	},
	"number_out_of_range": func(_ string) error {
		return error_only(canonical_json.CanonicalJSON(math.NaN()))
	},
	"unsupported_value": func(secret string) error {
		return error_only(canonical_json.CanonicalJSON(map[string]any{"text": "\xff" + secret, "channel": make(chan int)}))
	},
	"event_schema_invalid": func(secret string) error {
		return event_validation.ValidateEvent("log.entry.recorded", map[string]any{"extra": secret})
	},
	"event_type_unknown": func(_ string) error {
		return event_validation.ValidateEvent("nothing.here.recorded", map[string]any{})
	},
	"limit_exceeded": func(secret string) error {
		small_limits, limits_error := value_objects.NewLimits(10, 64, 256, 300, 1000)
		if limits_error != nil {
			return limits_error
		}
		_, _, apply_error := limits.ApplyLimits(producer_envelope(strings.Repeat(secret, 50)), small_limits)
		return apply_error
	},
	"limit_invalid": func(_ string) error {
		return error_only(value_objects.NewLimits(0, 64, 256, 1000, 2000))
	},
	"batch_limit_below_event_limit": func(_ string) error {
		return error_only(value_objects.NewLimits(10, 64, 256, 1000, 500))
	},
	"mask_pattern_invalid": func(secret string) error {
		return error_only(masking.NewSensitiveDataMasker(masking.MaskerOptions{ExtraPatterns: []string{secret + "("}}))
	},
	"mask_rule_invalid": func(secret string) error {
		return error_only(masking.NewSensitiveDataMasker(masking.MaskerOptions{ClassRules: map[string]string{"c": secret}}))
	},
	"delivery_mode_unknown": func(secret string) error {
		return error_only(value_objects.NewDeliveryMode(secret))
	},
	"config_field_invalid": func(secret string) error {
		invalid := producer_config()
		invalid.Resource.ServiceName = secret + "\n"
		invalid.Delivery.QueueSize = -1
		return error_only(config.NewConfigurationProvider(context.Background(), invalid, fakes.NewFakeIDGenerator()))
	},
	"instance_id_unavailable": func(_ string) error {
		failing_generator := fakes.NewFakeIDGenerator()
		failing_generator.FailWith(errors.New("the generator is down"))
		return error_only(config.NewConfigurationProvider(context.Background(), producer_config(), failing_generator))
	},
	"event_build_failed": func(_ string) error {
		failing_generator := fakes.NewFakeIDGenerator()
		failing_generator.FailWith(errors.New("the generator is down"))
		resource, resource_error := value_objects.NewResourceIdentity("sensor-hub", "hub-01")
		if resource_error != nil {
			return resource_error
		}
		instant, instant_error := value_objects.ParseUTCInstant("2026-10-28T10:00:00.123456Z")
		if instant_error != nil {
			return instant_error
		}
		builder := services.NewEventBuilderService(
			fakes.NewFakeClock(instant), failing_generator,
			fakes.NewFakeConfiguration(dtos.Configuration{Resource: resource}),
			func(string, any) error { return nil })
		return error_only(builder.Build(context.Background(), dtos.NewEventInput{
			EventType: "log.entry.recorded", Severity: value_objects.SeverityInfo, Data: map[string]any{},
		}))
	},
	"masking_failed": func(_ string) error {
		var missing_service *services.MaskingService
		return error_only(missing_service.Mask(context.Background(), value_objects.Envelope{}))
	},
	"limits_failed": func(_ string) error {
		var missing_service *services.LimitsService
		_, _, apply_error := missing_service.Apply(context.Background(), value_objects.Envelope{})
		return apply_error
	},
}

func Test_the_cases_of_the_shared_table_are_the_producers_of_this_language(t *testing.T) {
	table := load_error_codes_table(t)
	in_table := map[string]bool{}
	for _, table_case := range table.Cases {
		if in_table[table_case.Name] {
			t.Errorf("the case %q is twice in the table", table_case.Name)
		}
		in_table[table_case.Name] = true
		if _, has_producer := producers[table_case.Name]; !has_producer {
			t.Errorf("the case %q of the table has no producer in Go", table_case.Name)
		}
	}
	for name := range producers {
		if !in_table[name] {
			t.Errorf("the producer %q has no case in the shared table", name)
		}
	}
}

func Test_every_case_of_the_shared_table_gives_the_expected_code_and_no_value_of_the_caller(t *testing.T) {
	table := load_error_codes_table(t)
	for _, table_case := range table.Cases {
		producer, has_producer := producers[table_case.Name]
		if !has_producer {
			continue
		}
		t.Run(table_case.Name, func(t *testing.T) {
			produced := producer(table.Secret)
			if produced == nil {
				t.Fatal("the producer must make a real failure and got no error")
			}
			if code := value_objects.CodeOf(produced); code != table_case.ExpectedCode {
				t.Errorf("CodeOf = %q, want %q; error: %v", code, table_case.ExpectedCode, produced)
			}
			if table_case.ValueInInput && strings.Contains(produced.Error(), table.Secret) {
				t.Errorf("the message of the error holds the value of the caller: %v", produced)
			}
			if strings.Contains(produced.Error(), table.Secret) && !table_case.ValueInInput {
				t.Errorf("a case without a value of the caller has the secret in its message: %v", produced)
			}
		})
	}
}

func Test_the_root_cause_of_the_services_is_reachable_with_errors_is_and_the_code_with_errors_as(t *testing.T) {
	cases := map[string]error{
		"event_build_failed":      producers["event_build_failed"]("x"),
		"masking_failed":          producers["masking_failed"]("x"),
		"limits_failed":           producers["limits_failed"]("x"),
		"instance_id_unavailable": producers["instance_id_unavailable"]("x"),
	}
	for code, produced := range cases {
		var classified *domain.ClassifiedError
		if !errors.As(produced, &classified) || classified.Code != code {
			t.Errorf("%s: errors.As found %v", code, classified)
		}
	}
}

func Test_every_code_of_the_catalog_is_in_the_table_and_every_code_of_the_table_is_in_the_catalog(t *testing.T) {
	table := load_error_codes_table(t)
	catalog_codes := map[string]bool{}
	for _, info := range value_objects.ErrorCodes() {
		catalog_codes[info.Code] = true
	}
	used := map[string]bool{}
	for _, table_case := range table.Cases {
		used[table_case.ExpectedCode] = true
		if !catalog_codes[table_case.ExpectedCode] {
			t.Errorf("the case %q expects %q, which is not in the catalog", table_case.Name, table_case.ExpectedCode)
		}
	}
	for code := range catalog_codes {
		if !used[code] {
			t.Errorf("the code %q of the catalog has no case in the table", code)
		}
	}
}

func Test_the_codes_of_the_generated_catalog_are_the_codes_of_the_yaml_catalog(t *testing.T) {
	content, read_error := os.ReadFile(error_catalog_path)
	if read_error != nil {
		t.Fatalf("the catalog cannot be read: %v", read_error)
	}
	yaml_codes := []string{}
	for _, line := range strings.Split(string(content), "\n") {
		if code, is_code := strings.CutPrefix(strings.TrimSpace(line), "- code:"); is_code {
			yaml_codes = append(yaml_codes, strings.TrimSpace(code))
		}
	}
	generated_codes := []string{}
	for _, info := range value_objects.ErrorCodes() {
		generated_codes = append(generated_codes, info.Code)
	}
	sort.Strings(yaml_codes)
	sort.Strings(generated_codes)
	if len(yaml_codes) == 0 || !slices.Equal(yaml_codes, generated_codes) {
		t.Errorf("the generated catalog %v differs from contracts/errors/catalog.yaml %v", generated_codes, yaml_codes)
	}
}
