package domain

import "errors"

// #### OPERATION ERRORS

// The sentinels of the domain operations. Each one is wrapped in a ClassifiedError, which carries the code of the
// catalog (contracts/errors/catalog.yaml). The full error model arrives with T-03.7; these are small on purpose and
// easy to move.
var (
	// ErrNumberOutOfRange is returned for a number that has no exact canonical form: NaN, an infinity or an integer
	// beyond the range of exact integers (2^53 - 1).
	ErrNumberOutOfRange = errors.New("the number is outside the range of the canonical form")
	// ErrUnsupportedValue is returned for a value that has no canonical JSON form, such as a channel or a string that
	// is not valid UTF-8.
	ErrUnsupportedValue = errors.New("the value has no canonical JSON form")
	// ErrNonUTCTime is returned for an instant that is not in UTC or is outside the years 0001 to 9999.
	ErrNonUTCTime = errors.New("the instant is not a UTC time of the format of the contract")
	// ErrEventSchemaInvalid is returned for the data of an event that does not match the schema of its event type.
	ErrEventSchemaInvalid = errors.New("the data of the event does not match the schema of its type")
	// ErrEventTypeUnknown is returned for an event type that is not in the registry.
	ErrEventTypeUnknown = errors.New("the event type is not in the registry")
)

// #### VALUE OBJECT ERRORS

// The sentinels of the value objects. Each one states the rule that the input broke. They are provisional: the
// mapping to the codes of the catalog (ClassifiedError) is T-03.7. A constructor returns the sentinel itself, so
// errors.Is finds it; a composite object joins the sentinel of the field with the cause of the base value object.

// === UTCInstant ===

var (
	// ErrUTCInstantTextInvalid is returned for a text that is not an instant in the exact format of the contract:
	// YYYY-MM-DDTHH:MM:SS.ffffffZ, with six decimals and the letter Z.
	ErrUTCInstantTextInvalid = errors.New("the text is not an instant in the format YYYY-MM-DDTHH:MM:SS.ffffffZ")
)

// === Severity ===

var (
	// ErrSeverityInvalid is returned for a text that is not one of the values of the Severity enum of the contract.
	ErrSeverityInvalid = errors.New("the severity is not one of the values of the contract")
)

// === ActorKind ===

var (
	// ErrActorKindInvalid is returned for a text that is not one of the values of the ActorKind enum of the contract.
	ErrActorKindInvalid = errors.New("the actor kind is not one of the values of the contract")
)

// === EventID ===

var (
	// ErrEventIDInvalid is returned for an event id that is not a UUID of version 7, in lowercase, hyphenated 8-4-4-4-12
	// and with the RFC 4122 variant.
	ErrEventIDInvalid = errors.New("the event id is not a lowercase hyphenated UUID of version 7")
)

// === ResourceIdentity ===

var (
	// ErrResourceServiceNameInvalid is returned when service_name is missing or is not a valid identifier.
	ErrResourceServiceNameInvalid = errors.New("the resource needs a service_name that is a valid identifier")
	// ErrResourceInstanceIDInvalid is returned when instance_id is missing or is not a valid identifier.
	ErrResourceInstanceIDInvalid = errors.New("the resource needs an instance_id that is a valid identifier")
	// ErrResourceEnvironmentInvalid is returned when environment is present but is not a valid identifier.
	ErrResourceEnvironmentInvalid = errors.New("the environment of the resource, when present, must be a valid identifier")
	// ErrResourceServiceVersionInvalid is returned when service_version is present but is not a valid identifier.
	ErrResourceServiceVersionInvalid = errors.New("the service_version of the resource, when present, must be a valid identifier")
	// ErrResourceHostInvalid is returned when host is present but is not a valid identifier.
	ErrResourceHostInvalid = errors.New("the host of the resource, when present, must be a valid identifier")
	// ErrTimeZoneInvalid is returned when time_zone is present but does not have the shape of an IANA name. The time
	// zone is for display only; it is never used to compute a time (ADR-012).
	ErrTimeZoneInvalid = errors.New("the time_zone, when present, must have the shape of an IANA time zone name")
)

// === ActorIdentity ===

var (
	// ErrActorIDInvalid is returned when the id of the actor is missing or is not a valid identifier.
	ErrActorIDInvalid = errors.New("the actor needs an id that is a valid identifier")
	// ErrActorTypeInvalid is returned when the type of the actor is present but is not a valid identifier.
	ErrActorTypeInvalid = errors.New("the type of the actor, when present, must be a valid identifier")
	// ErrActorVersionInvalid is returned when the version of the actor is present but is not a valid identifier.
	ErrActorVersionInvalid = errors.New("the version of the actor, when present, must be a valid identifier")
)

// === CorrelationContext ===

var (
	// ErrContextTraceIDInvalid is returned when trace_id is present but is not 32 lowercase hexadecimal digits.
	ErrContextTraceIDInvalid = errors.New("the trace_id, when present, must be 32 lowercase hexadecimal digits")
	// ErrContextSpanIDInvalid is returned when span_id is present but is not 16 lowercase hexadecimal digits.
	ErrContextSpanIDInvalid = errors.New("the span_id, when present, must be 16 lowercase hexadecimal digits")
	// ErrContextExecutionIDInvalid is returned when execution_id is present but is not a valid identifier.
	ErrContextExecutionIDInvalid = errors.New("the execution_id, when present, must be a valid identifier")
	// ErrContextCorrelationIDInvalid is returned when correlation_id is present but is not a valid identifier.
	ErrContextCorrelationIDInvalid = errors.New("the correlation_id, when present, must be a valid identifier")
)

// === EventAttributes ===

var (
	// ErrAttributeKeyInvalid is returned for an attribute whose key is empty.
	ErrAttributeKeyInvalid = errors.New("the key of an attribute must not be empty")
	// ErrAttributeValueInvalid is returned for an attribute whose value is not a string, a boolean, a number or a flat
	// list of those (REQ-CTR-010): a null, a nested object and a nested list are rejected.
	ErrAttributeValueInvalid = errors.New("the value of an attribute must be a scalar or a flat list of scalars")
)

// #### CLASSIFIED ERROR

// ClassifiedError is an error with the code of the catalog of error codes. It keeps the cause, so errors.Is finds the
// sentinel and errors.As finds the code.
type ClassifiedError struct {
	// Code is a code of contracts/errors/catalog.yaml (see the ErrorCode constants of value_objects).
	Code string
	// Err is the cause: a sentinel joined with the detail.
	Err error
}

// Error returns the code and the message of the cause.
func (classified *ClassifiedError) Error() string {
	return classified.Code + ": " + classified.Err.Error()
}

// Unwrap returns the cause, so errors.Is and errors.As reach the sentinel.
func (classified *ClassifiedError) Unwrap() error {
	return classified.Err
}

// NewClassifiedError builds a ClassifiedError from a code, a sentinel and an optional detail. The sentinel and the
// detail are joined, so both stay reachable.
func NewClassifiedError(code string, sentinel error, detail error) error {
	if detail == nil {
		return &ClassifiedError{Code: code, Err: sentinel}
	}
	return &ClassifiedError{Code: code, Err: errors.Join(sentinel, detail)}
}

// #### ERROR COMPOSITION

// JoinErrors joins the non-nil errors into one, so errors.Is finds each of them. It returns nil when there is none.
func JoinErrors(errs ...error) error {
	return errors.Join(errs...)
}
