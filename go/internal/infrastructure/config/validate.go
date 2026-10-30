package config

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// VALIDATE
// =============================================================================

// Validate checks a configuration. It returns nil when the configuration is valid, or a *ValidationError with every
// problem together, in the order of the properties of the schema. Each problem names the field and gives a fixed
// reason; no reason contains a configured value, because a value may be sensitive. This is the reference of the shared
// cases of integration-tests/fixtures/config.
func Validate(config Config) error {
	var problems []Problem
	add := func(field string, code string, reason string) {
		problems = append(problems, Problem{Field: field, Code: code, Reason: reason})
	}
	invalid := func(field string, reason string) {
		add(field, value_objects.ErrorCodeConfigFieldInvalid, reason)
	}

	// ----- resource -----
	if identifier_error := base_value_objects.ValidateIdentifier(config.Resource.ServiceName); identifier_error != nil {
		invalid("resource.service_name", "the service name is required and must be a text without control characters")
	}
	optional_identifiers := []struct{ field, text string }{
		{"resource.instance_id", config.Resource.InstanceID},
		{"resource.environment", config.Resource.Environment},
		{"resource.service_version", config.Resource.ServiceVersion},
		{"resource.host", config.Resource.Host},
	}
	for _, optional_identifier := range optional_identifiers {
		if optional_identifier.text != "" && base_value_objects.ValidateIdentifier(optional_identifier.text) != nil {
			invalid(optional_identifier.field, "when set it must be a text without control characters")
		}
	}
	if config.Resource.TimeZone != "" && value_objects.ValidateTimeZone(config.Resource.TimeZone) != nil {
		invalid("resource.time_zone", "when set it must have the shape of an IANA time zone name")
	}

	// ----- delivery -----
	delivery_mode_valid := value_objects.ValidateDeliveryMode(config.Delivery.Mode) == nil
	if !delivery_mode_valid {
		add("delivery.mode", value_objects.ErrorCodeDeliveryModeUnknown, "the delivery mode must be best_effort, retry or durable")
	}
	queue_size_valid := config.Delivery.QueueSize > 0
	if !queue_size_valid {
		invalid("delivery.queue_size", "it must be greater than zero")
	}
	if config.Delivery.BatchSize <= 0 {
		invalid("delivery.batch_size", "it must be greater than zero")
	} else if queue_size_valid && config.Delivery.BatchSize > config.Delivery.QueueSize {
		invalid("delivery.batch_size", "it must not be greater than the queue size")
	}
	if config.Delivery.FlushIntervalMS <= 0 {
		invalid("delivery.flush_interval_ms", "it must be greater than zero")
	}
	if config.Delivery.TimeoutMS <= 0 {
		invalid("delivery.timeout_ms", "it must be greater than zero")
	}

	// ----- limits -----
	limit_numbers := []struct {
		field  string
		number int
	}{
		{"limits.max_attribute_keys", config.Limits.MaxAttributeKeys},
		{"limits.max_attribute_key_chars", config.Limits.MaxAttributeKeyChars},
		{"limits.max_attribute_value_chars", config.Limits.MaxAttributeValueChars},
		{"limits.max_event_bytes", config.Limits.MaxEventBytes},
		{"limits.max_batch_bytes", config.Limits.MaxBatchBytes},
	}
	for _, limit_number := range limit_numbers {
		if limit_number.number <= 0 {
			invalid(limit_number.field, "it must be greater than zero")
		}
	}
	if config.Limits.MaxEventBytes > 0 && config.Limits.MaxBatchBytes > 0 && config.Limits.MaxBatchBytes < config.Limits.MaxEventBytes {
		invalid("limits.max_batch_bytes", "it must not be less than the event limit")
	}

	// ----- spool -----
	if delivery_mode_valid && config.Delivery.Mode == string(value_objects.DeliveryModeDurable) && config.Spool.Directory == "" {
		invalid("spool.directory", "it is required when the delivery mode is durable")
	}
	if config.Spool.QuotaBytes <= 0 {
		invalid("spool.quota_bytes", "it must be greater than zero")
	}

	// ----- masking -----
	if _, masker_error := masking.NewSensitiveDataMasker(masking.MaskerOptions{ExtraPatterns: config.Masking.ExtraPatterns}); masker_error != nil {
		invalid("masking.extra_patterns", "an extra pattern is not a valid regular expression")
	}
	for _, rule := range config.Masking.ClassRules {
		if rule != domain.MaskRuleDrop && rule != domain.MaskRuleRedact && rule != domain.MaskRuleAllow {
			invalid("masking.class_rules", "every rule must be drop, redact or allow")
			break
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return &ValidationError{Problems: problems}
}
