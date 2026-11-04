// Package containers builds the objects of the core with constructor injection.
package containers

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/inbound"
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
	delivery *services.DeliveryService
	logger   inbound.LoggerPort
}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewCore validates the configuration and wires the core with the adapters of the options; the ones that are not given
// are the system clock and the UUIDv7 generator. Nothing is read from the environment (ADR-004).
//
// NewCore also starts the delivery: one worker goroutine per destination, each with a bounded queue (T-05.1). Nothing
// starts before NewCore is called, and an invalid configuration starts nothing. Only Shutdown stops these workers: a
// program must call it before it exits, and a core that is no longer used without it leaves its goroutines running.
//
// An invalid configuration returns a *infra_config.ValidationError with every problem together. The delivery mode
// durable is refused while the spool does not exist (T-06.5), with or without a spool adapter: it is one more problem of
// the same error (field delivery.mode, code config_field_invalid), after the ones of the configuration, so a project
// sees everything it must fix at once. The configuration itself (infra_config.Validate) still accepts durable, because
// the profiles Robotics and OfflineDevice give it.
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

	if validation_error := validate_start(cfg); validation_error != nil {
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
	delivery := services.NewDeliveryService(
		applied_options.Destinations,
		services.DeliverySettings{
			QueueSize:        configuration.Delivery.QueueSize,
			BatchSize:        configuration.Delivery.BatchSize,
			FlushIntervalMS:  configuration.Delivery.FlushIntervalMilliseconds,
			TimeoutMS:        configuration.Delivery.TimeoutMilliseconds,
			QueuePolicy:      configuration.Delivery.QueuePolicy,
			BlockTimeoutMS:   configuration.Delivery.BlockTimeoutMilliseconds,
			Mode:             configuration.Delivery.Mode,
			EventClasses:     configuration.Delivery.EventClasses,
			RetryMaxAttempts: configuration.Delivery.RetryMaxAttempts,
			RetryBaseDelayMS: configuration.Delivery.RetryBaseDelayMS,
			RetryMaxDelayMS:  configuration.Delivery.RetryMaxDelayMS,
		},
		applied_options.Clock,
		dependencies.NewSystemRandom(),
		dependencies.NewMemoryQueue,
	)
	pipeline := services.NewEventPipelineService(
		builder,
		services.NewMaskingService(masker),
		services.NewLimitsService(configuration.Limits),
		delivery,
	)
	delivery.Start()
	return &Core{pipeline: pipeline, delivery: delivery, logger: services.NewLoggerService(pipeline)}, nil
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

// Logger returns the logger of the core: one method per severity, each call one log.entry.recorded event through the
// pipeline. It never returns nil: a nil *Core, or one that was not built by NewCore, returns a logger that does nothing.
func (core *Core) Logger() inbound.LoggerPort {
	if core == nil || core.logger == nil {
		return (*services.LoggerService)(nil)
	}
	return core.logger
}

// Flush sends the events that are queued and keeps the core running. Because there is no synchronous send, a program MUST
// call Shutdown before it exits, or the events that are still queued are lost; Flush alone does not protect them.
//
// The time is the deadline of ctx; without a deadline it is 5 seconds (domain.DefaultShutdownTimeoutMilliseconds), so no
// call waits for ever. The result says how many events were sent and how many were dropped (the events that a destination
// failed, dropped or did not finish in time; with the retry mode, after the attempts of the batch); Spooled is zero
// until the spool exists. It never panics and never returns an error, a nil *Core included (the zero result), and a Flush after Shutdown is the zero result.
func (core *Core) Flush(ctx context.Context) dtos.ShutdownResult {
	if core == nil || core.delivery == nil {
		return dtos.ShutdownResult{}
	}
	return core.delivery.Flush(ctx)
}

// Shutdown flushes the queues and closes the core. Because there is no synchronous send, a program MUST call Shutdown
// before it exits, or the events that are still queued are lost.
//
// The time is the deadline of ctx; without a deadline it is 5 seconds (domain.DefaultShutdownTimeoutMilliseconds), so the
// call never waits for ever: a destination that hangs is abandoned at that time (its goroutine leaks until its Send
// returns) and its events are counted as dropped. After Shutdown every log call and Emit is Dropped 1 with the reason
// "shutdown", counted in the stats of the destinations. A second call returns the first result. It never panics and never
// returns an error, a nil *Core included (the zero result).
func (core *Core) Shutdown(ctx context.Context) dtos.ShutdownResult {
	if core == nil || core.delivery == nil {
		return dtos.ShutdownResult{}
	}
	return core.delivery.Shutdown(ctx)
}

// ----- HELPERS --------------------------------------------------------------

// validate_start returns the *ValidationError of the configuration and of the start together, or nil when both are
// valid. The configuration problems come first.
func validate_start(cfg infra_config.Config) error {
	var problems []infra_config.Problem
	if validation_error, is_validation := infra_config.Validate(cfg).(*infra_config.ValidationError); is_validation {
		problems = append(problems, validation_error.Problems...)
	}
	if cfg.Delivery.Mode == string(value_objects.DeliveryModeDurable) {
		problems = append(problems, infra_config.Problem{
			Field:  "delivery.mode",
			Code:   value_objects.ErrorCodeConfigFieldInvalid,
			Reason: "the durable mode is not available yet: it needs the spool of T-06.5",
		})
	}
	if len(problems) == 0 {
		return nil
	}
	return infra_config.NewValidationError(problems)
}
