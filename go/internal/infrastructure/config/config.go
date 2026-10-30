package config

// =============================================================================
// CONFIG
// =============================================================================

// Config is the configuration of the core as the consumer writes it: plain values, the same logical model as
// contracts/config/config.schema.json (the json tags are the names of the schema, the env tags are its x-env names).
// The library never reads the environment or a file (ADR-004): the consumer chooses a profile (Microservice, Cronjob,
// Worker, Robotics, OfflineDevice) or Defaults, loads the environment over it with its own tool, using the env tags,
// sets what it wants in code, and passes the result to NewConfigurationProvider, which validates it.
//
// The env tags carry no default on purpose: a default in a tag would override the profile when the consumer loads the
// environment after choosing the profile. The defaults live in one place, Defaults. An empty text means not set.
type Config struct {
	Resource ResourceConfig `json:"resource"`
	Delivery DeliveryConfig `json:"delivery"`
	Limits   LimitsConfig   `json:"limits"`
	Spool    SpoolConfig    `json:"spool"`
	Masking  MaskingConfig  `json:"masking"`
}

// ResourceConfig is the identity of the emitting process.
type ResourceConfig struct {
	// ServiceName is the logical name of the service. Required.
	ServiceName string `json:"service_name" env:"TELEMETRY_SERVICE_NAME"`
	// InstanceID is the instance id. When empty the core generates one at start.
	InstanceID string `json:"instance_id" env:"TELEMETRY_INSTANCE_ID"`
	// Environment is the environment (prod, staging, ...). Optional.
	Environment string `json:"environment" env:"TELEMETRY_ENVIRONMENT"`
	// ServiceVersion is the version of the service. Optional.
	ServiceVersion string `json:"service_version" env:"TELEMETRY_SERVICE_VERSION"`
	// Host is the host name. Optional.
	Host string `json:"host" env:"TELEMETRY_HOST"`
	// TimeZone is the IANA time zone of the source, for display only (ADR-012). Optional.
	TimeZone string `json:"time_zone" env:"TELEMETRY_TIME_ZONE"`
}

// DeliveryConfig is how events leave the process.
type DeliveryConfig struct {
	// Mode is the delivery mode: best_effort, retry or durable. The profile gives the default.
	Mode string `json:"mode" env:"TELEMETRY_DELIVERY_MODE"`
	// QueueSize is the number of events held in the bounded queue.
	QueueSize int `json:"queue_size" env:"TELEMETRY_QUEUE_SIZE"`
	// BatchSize is the number of events in one batch. Not greater than QueueSize.
	BatchSize int `json:"batch_size" env:"TELEMETRY_BATCH_SIZE"`
	// FlushIntervalMS is the milliseconds between flushes.
	FlushIntervalMS int `json:"flush_interval_ms" env:"TELEMETRY_FLUSH_INTERVAL_MS"`
	// TimeoutMS is the milliseconds before a network operation gives up.
	TimeoutMS int `json:"timeout_ms" env:"TELEMETRY_TIMEOUT_MS"`
}

// LimitsConfig is the size limits of an event and of a batch.
type LimitsConfig struct {
	// MaxAttributeKeys is the number of attribute keys per event.
	MaxAttributeKeys int `json:"max_attribute_keys" env:"TELEMETRY_MAX_ATTRIBUTE_KEYS"`
	// MaxAttributeKeyChars is the number of characters of an attribute key.
	MaxAttributeKeyChars int `json:"max_attribute_key_chars" env:"TELEMETRY_MAX_ATTRIBUTE_KEY_CHARS"`
	// MaxAttributeValueChars is the number of characters of a text attribute value.
	MaxAttributeValueChars int `json:"max_attribute_value_chars" env:"TELEMETRY_MAX_ATTRIBUTE_VALUE_CHARS"`
	// MaxEventBytes is the number of bytes of the canonical JSON of one event.
	MaxEventBytes int `json:"max_event_bytes" env:"TELEMETRY_MAX_EVENT_BYTES"`
	// MaxBatchBytes is the number of bytes of one HTTP batch. Not less than MaxEventBytes.
	MaxBatchBytes int `json:"max_batch_bytes" env:"TELEMETRY_MAX_BATCH_BYTES"`
}

// SpoolConfig is the local spool of the durable delivery mode.
type SpoolConfig struct {
	// Directory is the directory of the spool. Required when the delivery mode is durable; there is no default.
	Directory string `json:"directory" env:"TELEMETRY_SPOOL_DIRECTORY"`
	// QuotaBytes is the disk quota of the spool per destination.
	QuotaBytes int64 `json:"quota_bytes" env:"TELEMETRY_SPOOL_QUOTA_BYTES"`
}

// MaskingConfig is the options of the masking.
type MaskingConfig struct {
	// ExtraPatterns are extra regular expressions whose matches are hidden.
	ExtraPatterns []string `json:"extra_patterns" env:"TELEMETRY_MASK_EXTRA_PATTERNS"`
	// FieldNames, when not nil (even empty), REPLACES the default list of sensitive field names; nil means not set and
	// the default list is used.
	FieldNames []string `json:"field_names" env:"TELEMETRY_MASK_FIELD_NAMES"`
	// ExtraFieldNames are field names added to the active list.
	ExtraFieldNames []string `json:"extra_field_names" env:"TELEMETRY_MASK_EXTRA_FIELD_NAMES"`
	// FieldClasses maps a field name to a class name.
	FieldClasses map[string]string `json:"field_classes" env:"TELEMETRY_MASK_FIELD_CLASSES"`
	// ClassRules maps a class name to a rule: drop, redact or allow.
	ClassRules map[string]string `json:"class_rules" env:"TELEMETRY_MASK_CLASS_RULES"`
}
