// Package event_validation checks the data of an event against the schema of its event type, at the source
// (REQ-CTR-002, REQ-CTR-003). The validators of the event types are generated from contracts/schemas by
// tools/contract_generator (event_validators_generated.go): plain code over the rules of rules.go, with no JSON Schema
// library, so the core keeps a minimal set of dependencies (REQ-ARC-005). The registry of event types is read at
// generation, never at run time.
//
// Validation never panics and never returns the value it rejected: a value may be a secret. A rejection is a
// classified error (domain.ClassifiedError) that names the event type and the field path (ADR-005, REQ-DLV-002).
package event_validation

import (
	"errors"
	"slices"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// root_path is the path of the data of an event.
const root_path = "data"

// ValidateEvent checks that the data of an event matches the schema of its event type. The data is the generic JSON
// value (a map[string]any). It returns nil when the data is valid. Otherwise it returns a *domain.ClassifiedError: the
// code event_type_unknown when the type is not registered and the code event_schema_invalid when the data does not
// match, with a message like "log.entry.recorded: data.message: required field is missing". The message names the
// field path and never the value. It never panics, whatever the input: a last-resort recover turns any panic into
// event_schema_invalid.
func ValidateEvent(event_type string, data any) error {
	return Safely(event_type, func() error { return validate_event(event_type, data) })
}

// Safely runs a validation and turns a panic into the classified error event_schema_invalid ("validation failed"), so
// the caller of a log call never sees one. ValidateEvent runs every validation through it.
func Safely(event_type string, validate func() error) (result error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = schema_error(event_type, "validation failed")
		}
	}()
	return validate()
}

// validate_event runs the validator generated for the event type.
func validate_event(event_type string, data any) error {
	validator, has_validator := event_validators[event_type]
	if !has_validator || !slices.Contains(value_objects.EventTypes(), event_type) {
		return domain.NewClassifiedError(
			value_objects.ErrorCodeEventTypeUnknown, domain.ErrEventTypeUnknown,
			errors.New("the event type is not registered: "+DisplayText(event_type)),
		)
	}
	if problem := validator(data, root_path); problem != nil {
		return schema_error(event_type, problem.Path+": "+problem.Reason)
	}
	return nil
}

// schema_error is the classified error of data that does not match its schema.
func schema_error(event_type string, detail string) error {
	return domain.NewClassifiedError(
		value_objects.ErrorCodeEventSchemaInvalid, domain.ErrEventSchemaInvalid,
		errors.New(DisplayText(event_type)+": "+detail),
	)
}
