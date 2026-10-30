package config

import (
	"context"
	"maps"
	"slices"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// CONFIGURATION PROVIDER
// =============================================================================

// ConfigurationProvider is the outbound.ConfigurationPort of a consumer that passes its values in code, or loaded from
// its environment by its own tool. It validates the Config once, at start, and keeps the validated dtos.Configuration;
// it holds no state that changes and it reads no file and no environment (ADR-004).
type ConfigurationProvider struct {
	configuration dtos.Configuration
}

var _ outbound.ConfigurationPort = (*ConfigurationProvider)(nil)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewConfigurationProvider validates the config and returns the provider of it. An invalid config returns the
// *ValidationError as it is, with every problem. When resource.instance_id is empty it asks the identifier generator
// for one; a failure is returned as a classified error (instance_id_unavailable) with application.ErrConfigurationInstanceIDFailed joined with the cause.
func NewConfigurationProvider(ctx context.Context, config Config, id_generator outbound.IDGeneratorPort) (*ConfigurationProvider, error) {
	if validation_error := Validate(config); validation_error != nil {
		return nil, validation_error
	}

	instance_id := config.Resource.InstanceID
	if instance_id == "" {
		generated_id, generation_error := id_generator.NewInstanceID(ctx)
		if generation_error != nil {
			return nil, domain.NewClassifiedError(value_objects.ErrorCodeInstanceIdUnavailable, application.ErrConfigurationInstanceIDFailed, generation_error)
		}
		instance_id = generated_id.String()
	}

	resource_options := []value_objects.ResourceOption{}
	if config.Resource.Environment != "" {
		resource_options = append(resource_options, value_objects.WithEnvironment(config.Resource.Environment))
	}
	if config.Resource.ServiceVersion != "" {
		resource_options = append(resource_options, value_objects.WithServiceVersion(config.Resource.ServiceVersion))
	}
	if config.Resource.Host != "" {
		resource_options = append(resource_options, value_objects.WithHost(config.Resource.Host))
	}
	if config.Resource.TimeZone != "" {
		resource_options = append(resource_options, value_objects.WithTimeZone(config.Resource.TimeZone))
	}
	resource, resource_error := value_objects.NewResourceIdentity(config.Resource.ServiceName, instance_id, resource_options...)
	if resource_error != nil {
		return nil, resource_error
	}

	delivery_mode, delivery_mode_error := value_objects.NewDeliveryMode(config.Delivery.Mode)
	if delivery_mode_error != nil {
		return nil, domain.NewClassifiedError(value_objects.ErrorCodeDeliveryModeUnknown, delivery_mode_error, nil)
	}

	limits, limits_error := value_objects.NewLimits(
		config.Limits.MaxAttributeKeys, config.Limits.MaxAttributeKeyChars, config.Limits.MaxAttributeValueChars,
		config.Limits.MaxEventBytes, config.Limits.MaxBatchBytes)
	if limits_error != nil {
		return nil, limits_error
	}

	return &ConfigurationProvider{configuration: dtos.Configuration{
		Resource: resource,
		Delivery: dtos.DeliverySettings{
			Mode:                      delivery_mode,
			QueueSize:                 config.Delivery.QueueSize,
			BatchSize:                 config.Delivery.BatchSize,
			FlushIntervalMilliseconds: config.Delivery.FlushIntervalMS,
			TimeoutMilliseconds:       config.Delivery.TimeoutMS,
		},
		Limits: limits,
		Spool:  dtos.SpoolSettings{Directory: config.Spool.Directory, QuotaBytes: config.Spool.QuotaBytes},
		Masking: clone_masker_options(masking.MaskerOptions{
			ExtraPatterns:   config.Masking.ExtraPatterns,
			FieldNames:      config.Masking.FieldNames,
			ExtraFieldNames: config.Masking.ExtraFieldNames,
			FieldClasses:    config.Masking.FieldClasses,
			ClassRules:      config.Masking.ClassRules,
		}),
	}}, nil
}

// ----- METHODS --------------------------------------------------------------

// Configuration returns a copy of the validated configuration: the lists and maps of the masking options are not shared
// with the provider, so a caller that changes them changes nothing else.
func (provider *ConfigurationProvider) Configuration(_ context.Context) (dtos.Configuration, error) {
	configuration := provider.configuration
	configuration.Masking = clone_masker_options(configuration.Masking)
	return configuration, nil
}

// ----- HELPERS --------------------------------------------------------------

// clone_masker_options copies the lists and maps of the options. A nil list stays nil, because for FieldNames nil and
// empty mean different things.
func clone_masker_options(options masking.MaskerOptions) masking.MaskerOptions {
	return masking.MaskerOptions{
		ExtraPatterns:   slices.Clone(options.ExtraPatterns),
		FieldNames:      slices.Clone(options.FieldNames),
		ExtraFieldNames: slices.Clone(options.ExtraFieldNames),
		FieldClasses:    maps.Clone(options.FieldClasses),
		ClassRules:      maps.Clone(options.ClassRules),
	}
}
