package domain

import "errors"

// The errors of a contract that breaks a rule. Each one is wrapped with the path of the file that breaks it.
var (
	// ErrInvalidEnumValue is returned for an enum value that cannot be turned into an identifier in both languages.
	ErrInvalidEnumValue = errors.New("invalid enum value")
	// ErrDuplicateEnumValue is returned for an enum that repeats a value.
	ErrDuplicateEnumValue = errors.New("duplicated enum value")
	// ErrInvalidEventType is returned for an event type that is not three lowercase segments.
	ErrInvalidEventType = errors.New("invalid event type")
	// ErrDuplicateEventType is returned for an event type registered twice.
	ErrDuplicateEventType = errors.New("duplicated event type")
	// ErrSchemaPathMismatch is returned for a registered schema that is not the one of the event type.
	ErrSchemaPathMismatch = errors.New("the schema of the event type is not schemas/<domain>/<entity>_<action>.schema.json")
	// ErrSchemaMissing is returned for a registered event type whose schema does not exist.
	ErrSchemaMissing = errors.New("registered event type without a schema")
	// ErrSchemaNotRegistered is returned for an event data schema that no registered event type points to.
	ErrSchemaNotRegistered = errors.New("schema not registered in the event registry")
	// ErrInvalidErrorCode is returned for an error code that is not lowercase snake_case.
	ErrInvalidErrorCode = errors.New("invalid error code")
	// ErrDuplicateErrorCode is returned for an error code that appears twice.
	ErrDuplicateErrorCode = errors.New("duplicated error code")
	// ErrUnknownErrorCategory is returned for an error code whose category is not in enums/failure_category.json.
	ErrUnknownErrorCategory = errors.New("unknown error category")
	// ErrMissingDefaultMessage is returned for an error code without a default message.
	ErrMissingDefaultMessage = errors.New("error code without a default message")
)
