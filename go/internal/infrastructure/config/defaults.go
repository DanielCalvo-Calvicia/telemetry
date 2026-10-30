package config

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// DEFAULTS
// =============================================================================

// Defaults returns the configuration with the documented defaults: delivery mode best_effort, a queue of 10000 events,
// a batch of 100, a flush every 2000 ms, a timeout of 3000 ms, the default size limits and a spool quota of 256 MiB.
// Every text is empty (not set), the lists and maps of the masking are empty and not nil, and FieldNames is nil so the
// default list of sensitive field names applies. It is a new value on every call, so the caller may change it.
func Defaults() Config {
	return Config{
		Delivery: DeliveryConfig{
			Mode:            string(value_objects.DeliveryModeBestEffort),
			QueueSize:       domain.DefaultQueueSize,
			BatchSize:       domain.DefaultBatchSize,
			FlushIntervalMS: domain.DefaultFlushIntervalMilliseconds,
			TimeoutMS:       domain.DefaultTimeoutMilliseconds,
		},
		Limits: LimitsConfig{
			MaxAttributeKeys:       domain.DefaultMaxAttributeKeys,
			MaxAttributeKeyChars:   domain.DefaultMaxAttributeKeyChars,
			MaxAttributeValueChars: domain.DefaultMaxAttributeValueChars,
			MaxEventBytes:          domain.DefaultMaxEventBytes,
			MaxBatchBytes:          domain.DefaultMaxBatchBytes,
		},
		Spool: SpoolConfig{QuotaBytes: domain.DefaultSpoolQuotaBytes},
		Masking: MaskingConfig{
			ExtraPatterns:   []string{},
			FieldNames:      nil,
			ExtraFieldNames: []string{},
			FieldClasses:    map[string]string{},
			ClassRules:      map[string]string{},
		},
	}
}
