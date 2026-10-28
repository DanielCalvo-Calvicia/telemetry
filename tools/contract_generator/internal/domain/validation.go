package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	enum_value_pattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(_[A-Za-z0-9]+)*$`)
	event_type_pattern = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)
	error_code_pattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// SchemaPathOf returns the path of the data schema of an event type, relative to the contracts folder:
// schemas/<domain>/<entity>_<action>.schema.json. The event type must have three segments.
func SchemaPathOf(event_type string) string {
	segments := strings.Split(event_type, ".")
	return fmt.Sprintf("schemas/%s/%s_%s.schema.json", segments[0], segments[1], segments[2])
}

// Validate checks the rules of the contract: the enum values and the error codes are unique and well formed, every
// error code has a known category and a message, the event types are unique and well formed, every registered type
// has its schema and every event data schema is registered. It returns every breach, joined, each one naming its
// file; nil means the contract is valid.
func (contract Contract) Validate() error {
	var breaches []error
	breaches = append(breaches, contract.validateEnums()...)
	breaches = append(breaches, contract.validateEventTypes()...)
	breaches = append(breaches, contract.validateErrorCodes()...)
	return errors.Join(breaches...)
}

func (contract Contract) validateEnums() []error {
	var breaches []error
	for _, enum := range contract.Enums {
		seen := map[string]bool{}
		for _, value := range enum.Values {
			if !enum_value_pattern.MatchString(value) {
				breaches = append(breaches, fmt.Errorf("%w: %s: %q", ErrInvalidEnumValue, enum.Source, value))
				continue
			}
			if seen[strings.ToUpper(value)] {
				breaches = append(breaches, fmt.Errorf("%w: %s: %q", ErrDuplicateEnumValue, enum.Source, value))
			}
			seen[strings.ToUpper(value)] = true
		}
	}
	return breaches
}

func (contract Contract) validateEventTypes() []error {
	var breaches []error
	registered := map[string]bool{}
	seen := map[string]bool{}
	available := map[string]bool{}
	for _, schema_path := range contract.DataSchemas {
		available[schema_path] = true
	}
	for _, event_type := range contract.EventTypes {
		if !event_type_pattern.MatchString(event_type.Name) {
			breaches = append(breaches, fmt.Errorf("%w: %s: %q", ErrInvalidEventType, event_type.Source, event_type.Name))
			continue
		}
		if seen[event_type.Name] {
			breaches = append(breaches, fmt.Errorf("%w: %s: %q", ErrDuplicateEventType, event_type.Source, event_type.Name))
		}
		seen[event_type.Name] = true
		expected_path := SchemaPathOf(event_type.Name)
		registered[event_type.Schema] = true
		registered[expected_path] = true
		if event_type.Schema != expected_path {
			breaches = append(breaches, fmt.Errorf("%w: %s: %q points to %q, expected %q",
				ErrSchemaPathMismatch, event_type.Source, event_type.Name, event_type.Schema, expected_path))
		}
		if !available[expected_path] {
			breaches = append(breaches, fmt.Errorf("%w: %s: %q has no %s%s",
				ErrSchemaMissing, event_type.Source, event_type.Name, SourcePrefix, expected_path))
		}
	}
	for _, schema_path := range contract.DataSchemas {
		if !registered[schema_path] {
			breaches = append(breaches, fmt.Errorf("%w: %s%s", ErrSchemaNotRegistered, SourcePrefix, schema_path))
		}
	}
	return breaches
}

func (contract Contract) validateErrorCodes() []error {
	var breaches []error
	categories := map[string]bool{}
	for _, enum := range contract.Enums {
		if enum.Name == FailureCategoryEnum {
			for _, value := range enum.Values {
				categories[value] = true
			}
		}
	}
	seen := map[string]bool{}
	for _, error_code := range contract.ErrorCodes {
		if !error_code_pattern.MatchString(error_code.Code) {
			breaches = append(breaches, fmt.Errorf("%w: %s: %q", ErrInvalidErrorCode, error_code.Source, error_code.Code))
			continue
		}
		if seen[error_code.Code] {
			breaches = append(breaches, fmt.Errorf("%w: %s: %q", ErrDuplicateErrorCode, error_code.Source, error_code.Code))
		}
		seen[error_code.Code] = true
		if !categories[error_code.Category] {
			breaches = append(breaches, fmt.Errorf("%w: %s: %q has the category %q",
				ErrUnknownErrorCategory, error_code.Source, error_code.Code, error_code.Category))
		}
		if strings.TrimSpace(error_code.DefaultMessage) == "" {
			breaches = append(breaches, fmt.Errorf("%w: %s: %q", ErrMissingDefaultMessage, error_code.Source, error_code.Code))
		}
	}
	return breaches
}
