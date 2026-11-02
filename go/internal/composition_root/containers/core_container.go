// Package containers builds the objects of the core with constructor injection.
package containers

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/dependencies"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/event_validation"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	infra_config "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
)

// =============================================================================
// CORE CONTAINER
// =============================================================================

// Core is the telemetry core of one project: configured, wired and ready to take events. It holds no global state, so
// two cores in one process share nothing, and it is safe for concurrent use.
type Core struct {
	pipeline *services.EventPipelineService
}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewCore validates the configuration and wires the core with the adapters of the options; the ones that are not given
// are the system clock and the UUIDv7 generator. Nothing is read from the environment (ADR-004).
//
// An invalid configuration returns a *infra_config.ValidationError with every problem together. A delivery mode
// durable with no spool is one more problem of the same error (field delivery.mode, code config_field_invalid), after
// the ones of the configuration, so a project sees everything it must fix at once.
func NewCore(ctx context.Context, cfg infra_config.Config, options ...config.Option) (*Core, error) {
	applied_options := config.Options{}
	for _, option := range options {
		if option != nil {
			option(&applied_options)
		}
	}
	if applied_options.Clock == nil {
		applied_options.Clock = dependencies.NewSystemClock()
	}
	if applied_options.IDGenerator == nil {
		applied_options.IDGenerator = dependencies.NewUUIDGenerator()
	}

	if validation_error := validate_start(cfg, applied_options); validation_error != nil {
		return nil, validation_error
	}

	provider, provider_error := infra_config.NewConfigurationProvider(ctx, cfg, applied_options.IDGenerator)
	if provider_error != nil {
		return nil, provider_error
	}
	configuration, configuration_error := provider.Configuration(ctx)
	if configuration_error != nil {
		return nil, configuration_error
	}

	masker, masker_error := masking.NewSensitiveDataMasker(configuration.Masking)
	if masker_error != nil {
		return nil, masker_error
	}

	builder := services.NewEventBuilderService(applied_options.Clock, applied_options.IDGenerator, provider, event_validation.ValidateEvent)
	pipeline := services.NewEventPipelineService(
		builder,
		services.NewMaskingService(masker),
		services.NewLimitsService(configuration.Limits),
		applied_options.Destinations,
	)
	return &Core{pipeline: pipeline}, nil
}

// ----- METHODS --------------------------------------------------------------

// Emit turns one input into one masked, limited and valid event and delivers it to the sinks of its kind. It never
// panics and never returns an error: what happened is in the result (ADR-005). A nil *Core drops the event.
func (core *Core) Emit(ctx context.Context, input dtos.NewEventInput) dtos.EmitResult {
	if core == nil || core.pipeline == nil {
		return dtos.EmitResult{Dropped: 1, Reason: value_objects.ErrorCodeEventBuildFailed}
	}
	return core.pipeline.Emit(ctx, input)
}

// Shutdown closes the core. It is a stub: it returns the zero result and does nothing, because the core has no queue to
// flush yet; T-05.4 gives it the flush and the shutdown (REQ-DLV-010). It never panics, a nil *Core included.
func (core *Core) Shutdown(_ context.Context) dtos.ShutdownResult {
	return dtos.ShutdownResult{}
}

// ----- HELPERS --------------------------------------------------------------

// validate_start returns the *ValidationError of the configuration and of the options together, or nil when both are
// valid. The configuration problems come first.
func validate_start(cfg infra_config.Config, options config.Options) error {
	var problems []infra_config.Problem
	if validation_error, is_validation := infra_config.Validate(cfg).(*infra_config.ValidationError); is_validation {
		problems = append(problems, validation_error.Problems...)
	}
	if cfg.Delivery.Mode == string(value_objects.DeliveryModeDurable) && options.Spool == nil {
		problems = append(problems, infra_config.Problem{
			Field:  "delivery.mode",
			Code:   value_objects.ErrorCodeConfigFieldInvalid,
			Reason: "the durable mode needs a spool adapter",
		})
	}
	if len(problems) == 0 {
		return nil
	}
	return infra_config.NewValidationError(problems)
}
