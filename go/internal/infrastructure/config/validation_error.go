package config

import (
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// VALIDATION ERROR
// =============================================================================

// ErrConfigFieldInvalid is domain.ErrConfigFieldInvalid.
var ErrConfigFieldInvalid = domain.ErrConfigFieldInvalid

// ErrDeliveryModeUnknown is domain.ErrDeliveryModeUnknown.
var ErrDeliveryModeUnknown = domain.ErrDeliveryModeUnknown

// Problem is one thing that is wrong in a configuration: the path of the field (as in the schema, for example
// delivery.queue_size), the code of the catalog and a fixed reason. The reason never contains a configured value.
type Problem struct {
	Field  string
	Code   string
	Reason string
}

// ValidationError holds every problem of a configuration, in the order of the properties of the schema.
type ValidationError struct {
	Problems []Problem
}

// Error returns one line per problem: "field: reason (code)".
func (validation_error *ValidationError) Error() string {
	lines := make([]string, 0, len(validation_error.Problems))
	for _, problem := range validation_error.Problems {
		lines = append(lines, problem.Field+": "+problem.Reason+" ("+problem.Code+")")
	}
	return strings.Join(lines, "\n")
}

// Unwrap returns one *domain.ClassifiedError per problem, so errors.Is finds ErrConfigFieldInvalid and
// ErrDeliveryModeUnknown and errors.As finds the classified error with its code.
func (validation_error *ValidationError) Unwrap() []error {
	classified_errors := make([]error, 0, len(validation_error.Problems))
	for _, problem := range validation_error.Problems {
		sentinel := ErrConfigFieldInvalid
		if problem.Code == value_objects.ErrorCodeDeliveryModeUnknown {
			sentinel = ErrDeliveryModeUnknown
		}
		classified_errors = append(classified_errors, domain.NewClassifiedError(problem.Code, sentinel, problem_detail(problem)))
	}
	return classified_errors
}

// Code returns the code of the first problem, or an empty text when there is none.
func (validation_error *ValidationError) Code() string {
	if len(validation_error.Problems) == 0 {
		return ""
	}
	return validation_error.Problems[0].Code
}

// ----- HELPERS --------------------------------------------------------------

// problem_detail returns the detail of a problem: its field and reason, as an error value.
func problem_detail(problem Problem) error {
	return &problem_text{text: problem.Field + ": " + problem.Reason}
}

// problem_text is an error whose message is a fixed text.
type problem_text struct {
	text string
}

// Error returns the text.
func (problem *problem_text) Error() string {
	return problem.text
}
