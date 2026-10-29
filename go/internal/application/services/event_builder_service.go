// Package services holds the application services of the core.
package services

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/mappers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// EVENT BUILDER SERVICE
// =============================================================================

// EventDataValidator checks the data of an event against the schema of its event type. It returns nil when the data
// is valid and otherwise a *domain.ClassifiedError (event_type_unknown or event_schema_invalid). The composition root
// passes event_validation.ValidateEvent; a test may pass a permissive one.
type EventDataValidator func(event_type string, data any) error

// EventBuilderService turns what a capability gives the core into a complete, valid envelope.
type EventBuilderService struct {
	clock         outbound.ClockPort
	id_generator  outbound.IDGeneratorPort
	configuration outbound.ConfigurationPort
	validate_data EventDataValidator
}

// ----- CONSTANTS ------------------------------------------------------------

const (
	observed_at_key  = "observed_at"
	clock_source_key = "clock_source"
	clock_synced_key = "clock_synced"
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewEventBuilderService returns an EventBuilderService that reads the time and the clock information from the clock,
// the event ids from the id generator, the resource from the configuration and checks the data with validate_data.
func NewEventBuilderService(
	clock outbound.ClockPort,
	id_generator outbound.IDGeneratorPort,
	configuration outbound.ConfigurationPort,
	validate_data EventDataValidator,
) *EventBuilderService {
	return &EventBuilderService{
		clock:         clock,
		id_generator:  id_generator,
		configuration: configuration,
		validate_data: validate_data,
	}
}

// ----- METHODS --------------------------------------------------------------

// Build makes one envelope from the input. It never panics (a panic, including one of a port, becomes an error that
// wraps application.ErrEventBuildFailed) and it never returns a half-built envelope together with a nil error. The
// steps run in this order:
//
//  1. Read the clock once. recorded_at is that time, always: only the clock port sets it and the input has no field
//     for it. occurred_at is input.OccurredAt when given, otherwise that time. When data has observed_at, it must be a
//     string in the format YYYY-MM-DDTHH:MM:SS.ffffffZ and occurred_at is then equal to it, even over a given
//     OccurredAt (REQ-CTR-008). When it is present and not in that format the result is event_schema_invalid, with a
//     message that names data.observed_at and never its value.
//  2. Get the event_id from the id generator.
//  3. Get the resource from the configuration.
//  4. Read the clock information and merge it into a copy of the attributes of the caller: clock_source when the
//     source is known and clock_synced when the flag is known. An unknown value is absent, never false. When the
//     caller already has one of those keys, the value of the clock wins.
//  5. Assemble the envelope: schema_version is domain.SchemaVersion; actor and context are only there when given;
//     attributes only when not empty; data is a copy of the data of the caller and never null (see the mappers).
//  6. Validate data against the schema of the event type, last, on the data of the built envelope. A failure is
//     returned as it is: a *domain.ClassifiedError.
//
// A failure of a port or of the values the core made is application.ErrEventBuildFailed joined with its cause.
//
// The envelope is NOT masked and NOT limited yet (T-03.4, T-03.5). Nothing may send it to a sink before masking
// (ADR-006); T-03.9 wires the chain.
func (service *EventBuilderService) Build(ctx context.Context, input dtos.NewEventInput) (built_envelope value_objects.Envelope, build_error error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			built_envelope = value_objects.Envelope{}
			build_error = application.JoinErrors(application.ErrEventBuildFailed, application.ErrEventBuildPanicked)
		}
	}()

	now := service.clock.Now(ctx)
	occurred_at, time_error := occurrence_time(now, input)
	if time_error != nil {
		return value_objects.Envelope{}, time_error
	}

	event_id, id_error := service.id_generator.NewEventID(ctx)
	if id_error != nil {
		return value_objects.Envelope{}, application.JoinErrors(application.ErrEventBuildFailed, id_error)
	}

	configuration, configuration_error := service.configuration.Configuration(ctx)
	if configuration_error != nil {
		return value_objects.Envelope{}, application.JoinErrors(application.ErrEventBuildFailed, configuration_error)
	}

	attributes, attributes_error := merge_attributes(input.Attributes, service.clock.ClockInfo(ctx))
	if attributes_error != nil {
		return value_objects.Envelope{}, application.JoinErrors(application.ErrEventBuildFailed, attributes_error)
	}

	envelope := value_objects.Envelope{
		SchemaVersion: domain.SchemaVersion,
		EventId:       event_id.String(),
		EventType:     input.EventType,
		OccurredAt:    occurred_at.String(),
		RecordedAt:    now.String(),
		Severity:      input.Severity,
		Resource:      mappers.MapResource(configuration.Resource),
		Actor:         mappers.MapActor(input.Actor),
		Context:       mappers.MapContext(input.Context),
		Attributes:    mappers.MapAttributes(attributes),
		Data:          mappers.CopyData(input.Data),
	}

	if validation_error := service.validate_data(input.EventType, envelope.Data); validation_error != nil {
		return value_objects.Envelope{}, validation_error
	}
	return envelope, nil
}

// ----- HELPERS --------------------------------------------------------------

// occurrence_time returns occurred_at: data.observed_at when present, otherwise the given instant, otherwise now.
func occurrence_time(now value_objects.UTCInstant, input dtos.NewEventInput) (value_objects.UTCInstant, error) {
	if observed_value, has_observed_at := input.Data[observed_at_key]; has_observed_at {
		observed_text, is_text := observed_value.(string)
		if !is_text {
			return value_objects.UTCInstant{}, observed_at_error()
		}
		observed_at, parse_error := value_objects.ParseUTCInstant(observed_text)
		if parse_error != nil {
			return value_objects.UTCInstant{}, observed_at_error()
		}
		return observed_at, nil
	}
	if input.OccurredAt != nil {
		return *input.OccurredAt, nil
	}
	return now, nil
}

// observed_at_error is the classified error of a data.observed_at that is not in the format of the contract. It names
// the field and never the value.
func observed_at_error() error {
	return domain.NewClassifiedError(value_objects.ErrorCodeEventSchemaInvalid, domain.ErrEventSchemaInvalid, application.ErrObservedAtInvalid)
}

// merge_attributes returns the attributes of the caller plus the clock keys that the clock knows. The clock wins.
func merge_attributes(caller_attributes *value_objects.EventAttributes, clock_info dtos.ClockInfo) (value_objects.EventAttributes, error) {
	merged := map[string]any{}
	if caller_attributes != nil {
		for key, value := range caller_attributes.Entries() {
			merged[key] = value
		}
	}
	if clock_info.SourceKnown {
		merged[clock_source_key] = clock_info.Source
	}
	if clock_info.SyncedKnown {
		merged[clock_synced_key] = clock_info.Synced
	}
	return value_objects.NewEventAttributes(merged)
}
