package config_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

var _ outbound.ConfigurationPort = (*config.ConfigurationProvider)(nil)

func valid_config() config.Config {
	valid := config.Microservice()
	valid.Resource.ServiceName = "sensor-hub"
	return valid
}

func Test_the_defaults_are_the_documented_ones(t *testing.T) {
	expected := config.Config{
		Delivery: config.DeliveryConfig{Mode: "best_effort", QueueSize: 10000, BatchSize: 100, FlushIntervalMS: 2000, TimeoutMS: 3000},
		Limits: config.LimitsConfig{
			MaxAttributeKeys: 32, MaxAttributeKeyChars: 64, MaxAttributeValueChars: 256, MaxEventBytes: 65536, MaxBatchBytes: 1048576,
		},
		Spool: config.SpoolConfig{QuotaBytes: 268435456},
		Masking: config.MaskingConfig{
			ExtraPatterns: []string{}, FieldNames: nil, ExtraFieldNames: []string{},
			FieldClasses: map[string]string{}, ClassRules: map[string]string{},
		},
	}
	if got := config.Defaults(); !reflect.DeepEqual(got, expected) {
		t.Errorf("got %+v\nwant %+v", got, expected)
	}
	if domain.DefaultSpoolQuotaBytes != 256*1024*1024 {
		t.Errorf("the spool quota is %d, expected 256 MiB", domain.DefaultSpoolQuotaBytes)
	}
}

func Test_every_call_of_defaults_gives_a_value_that_shares_nothing(t *testing.T) {
	first := config.Defaults()
	first.Masking.ClassRules["secret"] = "drop"
	first.Masking.ExtraPatterns = append(first.Masking.ExtraPatterns, "x")
	if second := config.Defaults(); len(second.Masking.ClassRules) != 0 || len(second.Masking.ExtraPatterns) != 0 {
		t.Errorf("the defaults share state: %+v", second.Masking)
	}
}

func Test_each_profile_has_its_delivery_mode_and_nothing_else_changes(t *testing.T) {
	cases := []struct {
		name    string
		profile func() config.Config
		mode    string
	}{
		{"microservice", config.Microservice, "best_effort"},
		{"cronjob", config.Cronjob, "retry"},
		{"worker", config.Worker, "retry"},
		{"robotics", config.Robotics, "durable"},
		{"offline_device", config.OfflineDevice, "durable"},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			got := test_case.profile()
			if got.Delivery.Mode != test_case.mode {
				t.Errorf("the mode is %q, expected %q", got.Delivery.Mode, test_case.mode)
			}
			expected := config.Defaults()
			expected.Delivery.Mode = test_case.mode
			if !reflect.DeepEqual(got, expected) {
				t.Errorf("a profile changed more than the delivery mode: %+v", got)
			}
		})
	}
}

func Test_the_precedence_is_default_then_profile_then_the_options_in_code(t *testing.T) {
	if config.Defaults().Delivery.Mode != "best_effort" || config.Worker().Delivery.Mode != "retry" {
		t.Fatal("the profile does not beat the default")
	}
	chosen := config.Worker()
	chosen.Delivery.Mode = "best_effort"
	chosen.Delivery.QueueSize = 50
	chosen.Delivery.BatchSize = 5
	if chosen.Delivery.Mode != "best_effort" || chosen.Delivery.QueueSize != 50 {
		t.Fatal("an option in code does not beat the profile")
	}
	if validation_error := config.Validate(withServiceName(chosen)); validation_error != nil {
		t.Errorf("the overridden profile is not valid: %v", validation_error)
	}
}

func withServiceName(configuration config.Config) config.Config {
	configuration.Resource.ServiceName = "sensor-hub"
	return configuration
}

func Test_the_zero_config_reports_several_problems_together_in_the_order_of_the_schema(t *testing.T) {
	validation_error := config.Validate(config.Config{})
	var validation *config.ValidationError
	if !errors.As(validation_error, &validation) {
		t.Fatalf("expected a *ValidationError, got %v", validation_error)
	}
	got_fields := []string{}
	for _, problem := range validation.Problems {
		got_fields = append(got_fields, problem.Field+" "+problem.Code)
	}
	expected_fields := []string{
		"resource.service_name config_field_invalid",
		"delivery.mode delivery_mode_unknown",
		"delivery.queue_size config_field_invalid",
		"delivery.batch_size config_field_invalid",
		"delivery.flush_interval_ms config_field_invalid",
		"delivery.timeout_ms config_field_invalid",
		"limits.max_attribute_keys config_field_invalid",
		"limits.max_attribute_key_chars config_field_invalid",
		"limits.max_attribute_value_chars config_field_invalid",
		"limits.max_event_bytes config_field_invalid",
		"limits.max_batch_bytes config_field_invalid",
		"spool.quota_bytes config_field_invalid",
	}
	if !reflect.DeepEqual(got_fields, expected_fields) {
		t.Errorf("got %v\nwant %v", got_fields, expected_fields)
	}
	if validation.Code() != "config_field_invalid" {
		t.Errorf("the code is %q", validation.Code())
	}
}

func Test_each_rule_of_the_validation_reports_its_field(t *testing.T) {
	cases := []struct {
		name   string
		change func(configuration *config.Config)
		field  string
		code   string
	}{
		{"blank service name", func(c *config.Config) { c.Resource.ServiceName = "   " }, "resource.service_name", "config_field_invalid"},
		{"control character in the service name", func(c *config.Config) { c.Resource.ServiceName = "a\tb" }, "resource.service_name", "config_field_invalid"},
		{"bad instance id", func(c *config.Config) { c.Resource.InstanceID = " " }, "resource.instance_id", "config_field_invalid"},
		{"bad environment", func(c *config.Config) { c.Resource.Environment = "\n" }, "resource.environment", "config_field_invalid"},
		{"bad service version", func(c *config.Config) { c.Resource.ServiceVersion = "\n" }, "resource.service_version", "config_field_invalid"},
		{"bad host", func(c *config.Config) { c.Resource.Host = "\n" }, "resource.host", "config_field_invalid"},
		{"bad time zone", func(c *config.Config) { c.Resource.TimeZone = "Europe Madrid" }, "resource.time_zone", "config_field_invalid"},
		{"unknown mode", func(c *config.Config) { c.Delivery.Mode = "fast" }, "delivery.mode", "delivery_mode_unknown"},
		{"zero queue", func(c *config.Config) { c.Delivery.QueueSize = 0 }, "delivery.queue_size", "config_field_invalid"},
		{"zero batch", func(c *config.Config) { c.Delivery.BatchSize = 0 }, "delivery.batch_size", "config_field_invalid"},
		{"batch over queue", func(c *config.Config) { c.Delivery.QueueSize = 5; c.Delivery.BatchSize = 6 }, "delivery.batch_size", "config_field_invalid"},
		{"zero flush", func(c *config.Config) { c.Delivery.FlushIntervalMS = 0 }, "delivery.flush_interval_ms", "config_field_invalid"},
		{"zero timeout", func(c *config.Config) { c.Delivery.TimeoutMS = -1 }, "delivery.timeout_ms", "config_field_invalid"},
		{"zero keys", func(c *config.Config) { c.Limits.MaxAttributeKeys = 0 }, "limits.max_attribute_keys", "config_field_invalid"},
		{"zero key chars", func(c *config.Config) { c.Limits.MaxAttributeKeyChars = 0 }, "limits.max_attribute_key_chars", "config_field_invalid"},
		{"zero value chars", func(c *config.Config) { c.Limits.MaxAttributeValueChars = 0 }, "limits.max_attribute_value_chars", "config_field_invalid"},
		{"zero event bytes", func(c *config.Config) { c.Limits.MaxEventBytes = 0 }, "limits.max_event_bytes", "config_field_invalid"},
		{"zero batch bytes", func(c *config.Config) { c.Limits.MaxBatchBytes = 0 }, "limits.max_batch_bytes", "config_field_invalid"},
		{"batch bytes under event bytes", func(c *config.Config) { c.Limits.MaxBatchBytes = 10 }, "limits.max_batch_bytes", "config_field_invalid"},
		{"durable without directory", func(c *config.Config) { c.Delivery.Mode = "durable" }, "spool.directory", "config_field_invalid"},
		{"zero quota", func(c *config.Config) { c.Spool.QuotaBytes = 0 }, "spool.quota_bytes", "config_field_invalid"},
		{"bad pattern", func(c *config.Config) { c.Masking.ExtraPatterns = []string{"("} }, "masking.extra_patterns", "config_field_invalid"},
		{"bad class rule", func(c *config.Config) { c.Masking.ClassRules = map[string]string{"pii": "hide"} }, "masking.class_rules", "config_field_invalid"},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			configuration := valid_config()
			test_case.change(&configuration)
			var validation *config.ValidationError
			if !errors.As(config.Validate(configuration), &validation) {
				t.Fatal("expected a *ValidationError")
			}
			if len(validation.Problems) != 1 || validation.Problems[0].Field != test_case.field || validation.Problems[0].Code != test_case.code {
				t.Errorf("got %+v, expected one problem on %s with %s", validation.Problems, test_case.field, test_case.code)
			}
		})
	}
}

func Test_a_valid_configuration_with_a_durable_directory_and_optional_fields_passes(t *testing.T) {
	configuration := config.Robotics()
	configuration.Resource = config.ResourceConfig{
		ServiceName: "arm", InstanceID: "arm-1", Environment: "prod", ServiceVersion: "1.0", Host: "h", TimeZone: "Europe/Madrid",
	}
	configuration.Spool.Directory = "spool"
	configuration.Masking.ExtraPatterns = []string{"ID-[0-9]{4}", ""}
	configuration.Masking.ClassRules = map[string]string{"a": "drop", "b": "redact", "c": "allow"}
	if validation_error := config.Validate(configuration); validation_error != nil {
		t.Errorf("unexpected: %v", validation_error)
	}
}

func Test_the_dependent_rules_are_skipped_when_the_field_they_depend_on_is_invalid(t *testing.T) {
	configuration := valid_config()
	configuration.Delivery.QueueSize = 0
	configuration.Delivery.BatchSize = 1000000
	configuration.Limits.MaxEventBytes = 0
	configuration.Limits.MaxBatchBytes = 1
	configuration.Delivery.Mode = "fast"
	var validation *config.ValidationError
	if !errors.As(config.Validate(configuration), &validation) {
		t.Fatal("expected a *ValidationError")
	}
	got_fields := []string{}
	for _, problem := range validation.Problems {
		got_fields = append(got_fields, problem.Field)
	}
	expected := []string{"delivery.mode", "delivery.queue_size", "limits.max_event_bytes"}
	if !reflect.DeepEqual(got_fields, expected) {
		t.Errorf("got %v, want %v", got_fields, expected)
	}
}

func Test_a_validation_error_is_found_by_errors_is_and_errors_as(t *testing.T) {
	configuration := config.Config{}
	validation_error := config.Validate(configuration)
	if !errors.Is(validation_error, domain.ErrConfigFieldInvalid) || !errors.Is(validation_error, config.ErrConfigFieldInvalid) {
		t.Error("ErrConfigFieldInvalid is not found")
	}
	if !errors.Is(validation_error, domain.ErrDeliveryModeUnknown) || !errors.Is(validation_error, config.ErrDeliveryModeUnknown) {
		t.Error("ErrDeliveryModeUnknown is not found")
	}
	var classified *domain.ClassifiedError
	if !errors.As(validation_error, &classified) || classified.Code != value_objects.ErrorCodeConfigFieldInvalid {
		t.Errorf("no classified error with the code of the first problem: %v", classified)
	}
	if !strings.Contains(validation_error.Error(), "delivery.mode: ") || !strings.Contains(validation_error.Error(), "(delivery_mode_unknown)") {
		t.Errorf("the message lacks the field, the reason or the code: %s", validation_error.Error())
	}
	if strings.Count(validation_error.Error(), "\n") != 11 {
		t.Errorf("expected one line per problem: %s", validation_error.Error())
	}

	only_mode := valid_config()
	only_mode.Delivery.Mode = "fast"
	only_mode_error := config.Validate(only_mode)
	if errors.Is(only_mode_error, domain.ErrConfigFieldInvalid) || !errors.Is(only_mode_error, domain.ErrDeliveryModeUnknown) {
		t.Errorf("the sentinels of the problems are wrong: %v", only_mode_error)
	}
	if only_mode_error.(*config.ValidationError).Code() != "delivery_mode_unknown" {
		t.Error("the code of the first problem is not delivery_mode_unknown")
	}
	if (&config.ValidationError{}).Code() != "" {
		t.Error("an empty validation error has a code")
	}
}

func Test_no_problem_contains_a_configured_value(t *testing.T) {
	configuration := config.Config{}
	configuration.Resource.ServiceName = "secret-looking-name\n"
	configuration.Resource.Host = "secret-host\n"
	configuration.Resource.TimeZone = "secret zone"
	configuration.Delivery.Mode = "secret-mode"
	configuration.Spool.Directory = "secret-directory"
	configuration.Masking.ExtraPatterns = []string{"(secret-pattern"}
	configuration.Masking.ClassRules = map[string]string{"secret-class": "secret-rule"}
	validation_error := config.Validate(configuration)
	if validation_error == nil {
		t.Fatal("expected problems")
	}
	for _, word := range []string{"secret-looking-name", "secret-host", "secret zone", "secret-mode", "secret-directory", "secret-pattern", "secret-class", "secret-rule"} {
		if strings.Contains(validation_error.Error(), word) {
			t.Errorf("the message contains %q: %s", word, validation_error.Error())
		}
	}
	var validation *config.ValidationError
	errors.As(validation_error, &validation)
	for _, unwrapped := range validation.Unwrap() {
		if strings.Contains(unwrapped.Error(), "secret-") {
			t.Errorf("an unwrapped error contains a value: %s", unwrapped.Error())
		}
	}
}

func Test_the_provider_generates_the_instance_id_when_it_is_empty_and_keeps_the_given_one(t *testing.T) {
	generated, generated_error := config.NewConfigurationProvider(context.Background(), valid_config(), fakes.NewFakeIDGenerator())
	if generated_error != nil {
		t.Fatal(generated_error)
	}
	generated_configuration, _ := generated.Configuration(context.Background())
	if generated_configuration.Resource.InstanceID() != "instance-1" {
		t.Errorf("the instance id is %q", generated_configuration.Resource.InstanceID())
	}

	given := valid_config()
	given.Resource.InstanceID = "hub-01"
	id_generator := fakes.NewFakeIDGenerator()
	kept, kept_error := config.NewConfigurationProvider(context.Background(), given, id_generator)
	if kept_error != nil {
		t.Fatal(kept_error)
	}
	kept_configuration, _ := kept.Configuration(context.Background())
	if kept_configuration.Resource.InstanceID() != "hub-01" {
		t.Errorf("the instance id is %q", kept_configuration.Resource.InstanceID())
	}
	if next_id, _ := id_generator.NewInstanceID(context.Background()); next_id != "instance-1" {
		t.Error("the generator was used although the instance id was given")
	}
}

func Test_the_provider_returns_the_failure_of_the_id_generator_with_its_sentinel(t *testing.T) {
	id_generator := fakes.NewFakeIDGenerator()
	generator_failure := errors.New("the generator is down")
	id_generator.FailWith(generator_failure)
	provider, provider_error := config.NewConfigurationProvider(context.Background(), valid_config(), id_generator)
	if provider != nil || !errors.Is(provider_error, application.ErrConfigurationInstanceIDFailed) || !errors.Is(provider_error, generator_failure) {
		t.Errorf("got %v, %v", provider, provider_error)
	}
}

func Test_the_provider_returns_the_validation_error_as_it_is(t *testing.T) {
	provider, provider_error := config.NewConfigurationProvider(context.Background(), config.Config{}, fakes.NewFakeIDGenerator())
	if provider != nil {
		t.Error("a provider was returned for an invalid configuration")
	}
	if _, is_validation := provider_error.(*config.ValidationError); !is_validation {
		t.Errorf("expected a *ValidationError, got %T", provider_error)
	}
}

func Test_the_provider_maps_every_section_of_the_configuration(t *testing.T) {
	configuration := config.Config{
		Resource: config.ResourceConfig{
			ServiceName: "sensor-hub", InstanceID: "hub-01", Environment: "staging", ServiceVersion: "1.2.3", Host: "host-a", TimeZone: "Europe/Madrid",
		},
		Delivery: config.DeliveryConfig{Mode: "durable", QueueSize: 2000, BatchSize: 50, FlushIntervalMS: 500, TimeoutMS: 1500},
		Limits: config.LimitsConfig{
			MaxAttributeKeys: 16, MaxAttributeKeyChars: 32, MaxAttributeValueChars: 128, MaxEventBytes: 32768, MaxBatchBytes: 524288,
		},
		Spool: config.SpoolConfig{Directory: "spool-dir", QuotaBytes: 1048576},
		Masking: config.MaskingConfig{
			ExtraPatterns: []string{"ID-[0-9]{4}"}, FieldNames: []string{"ssn"}, ExtraFieldNames: []string{"iban"},
			FieldClasses: map[string]string{"patient_name": "pii"}, ClassRules: map[string]string{"pii": "drop"},
		},
	}
	provider, provider_error := config.NewConfigurationProvider(context.Background(), configuration, fakes.NewFakeIDGenerator())
	if provider_error != nil {
		t.Fatal(provider_error)
	}
	got, _ := provider.Configuration(context.Background())

	if got.Resource.ServiceName() != "sensor-hub" || got.Resource.InstanceID() != "hub-01" {
		t.Errorf("resource: %+v", got.Resource)
	}
	if environment, present := got.Resource.Environment(); !present || environment != "staging" {
		t.Error("environment")
	}
	if service_version, present := got.Resource.ServiceVersion(); !present || service_version != "1.2.3" {
		t.Error("service version")
	}
	if host, present := got.Resource.Host(); !present || host != "host-a" {
		t.Error("host")
	}
	if time_zone, present := got.Resource.TimeZone(); !present || time_zone != "Europe/Madrid" {
		t.Error("time zone")
	}
	expected_delivery := dtos.DeliverySettings{Mode: value_objects.DeliveryModeDurable, QueueSize: 2000, BatchSize: 50, FlushIntervalMilliseconds: 500, TimeoutMilliseconds: 1500}
	if got.Delivery != expected_delivery {
		t.Errorf("delivery: %+v", got.Delivery)
	}
	expected_limits, _ := value_objects.NewLimits(16, 32, 128, 32768, 524288)
	if got.Limits != expected_limits {
		t.Errorf("limits: %+v", got.Limits)
	}
	if got.Spool != (dtos.SpoolSettings{Directory: "spool-dir", QuotaBytes: 1048576}) {
		t.Errorf("spool: %+v", got.Spool)
	}
	if !reflect.DeepEqual(got.Masking.ExtraPatterns, configuration.Masking.ExtraPatterns) ||
		!reflect.DeepEqual(got.Masking.FieldNames, configuration.Masking.FieldNames) ||
		!reflect.DeepEqual(got.Masking.ExtraFieldNames, configuration.Masking.ExtraFieldNames) ||
		!reflect.DeepEqual(got.Masking.FieldClasses, configuration.Masking.FieldClasses) ||
		!reflect.DeepEqual(got.Masking.ClassRules, configuration.Masking.ClassRules) {
		t.Errorf("masking: %+v", got.Masking)
	}
}

func Test_the_provider_omits_the_optional_fields_that_are_empty(t *testing.T) {
	provider, _ := config.NewConfigurationProvider(context.Background(), valid_config(), fakes.NewFakeIDGenerator())
	got, _ := provider.Configuration(context.Background())
	if _, present := got.Resource.Environment(); present {
		t.Error("environment is present")
	}
	if _, present := got.Resource.TimeZone(); present {
		t.Error("time zone is present")
	}
	if _, present := got.Resource.Host(); present {
		t.Error("host is present")
	}
	if _, present := got.Resource.ServiceVersion(); present {
		t.Error("service version is present")
	}
}

func Test_the_provider_keeps_field_names_nil_and_empty_apart(t *testing.T) {
	not_set, _ := config.NewConfigurationProvider(context.Background(), valid_config(), fakes.NewFakeIDGenerator())
	not_set_configuration, _ := not_set.Configuration(context.Background())
	if not_set_configuration.Masking.FieldNames != nil {
		t.Error("a field_names that is not set became non-nil")
	}

	replaced := valid_config()
	replaced.Masking.FieldNames = []string{}
	replaced_provider, _ := config.NewConfigurationProvider(context.Background(), replaced, fakes.NewFakeIDGenerator())
	replaced_configuration, _ := replaced_provider.Configuration(context.Background())
	if replaced_configuration.Masking.FieldNames == nil || len(replaced_configuration.Masking.FieldNames) != 0 {
		t.Error("an empty field_names that replaces the list became nil")
	}
}

func Test_the_configuration_is_independent_of_later_changes(t *testing.T) {
	configuration := valid_config()
	configuration.Masking.ExtraPatterns = []string{"ID-[0-9]{4}"}
	configuration.Masking.ClassRules = map[string]string{"pii": "drop"}
	provider, _ := config.NewConfigurationProvider(context.Background(), configuration, fakes.NewFakeIDGenerator())

	configuration.Masking.ExtraPatterns[0] = "changed"
	configuration.Masking.ClassRules["pii"] = "allow"
	configuration.Masking.ClassRules["other"] = "drop"
	first, _ := provider.Configuration(context.Background())
	if first.Masking.ExtraPatterns[0] != "ID-[0-9]{4}" || first.Masking.ClassRules["pii"] != "drop" || len(first.Masking.ClassRules) != 1 {
		t.Errorf("the provider shares the Config: %+v", first.Masking)
	}

	first.Masking.ClassRules["pii"] = "redact"
	first.Masking.ExtraPatterns[0] = "changed again"
	second, _ := provider.Configuration(context.Background())
	if second.Masking.ClassRules["pii"] != "drop" || second.Masking.ExtraPatterns[0] != "ID-[0-9]{4}" {
		t.Errorf("a returned copy changes the provider: %+v", second.Masking)
	}
}
