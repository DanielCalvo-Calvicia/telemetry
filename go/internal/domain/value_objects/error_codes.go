package value_objects

import (
	"errors"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
)

// =============================================================================
// THE CODE OF AN ERROR
// =============================================================================

// sentinel_code maps one sentinel of the domain to the code of the catalog that it stands for.
type sentinel_code struct {
	sentinel error
	code     string
}

// sentinel_codes holds every sentinel of domain/errors.go and of base_value_objects/errors.go with its code. The
// sentinels of the application layer are not here: the application layer cannot be imported by the domain, and its
// services always return them inside a *domain.ClassifiedError, which carries the code itself.
var sentinel_codes = []sentinel_code{
	{base_value_objects.ErrIdentifierBlank, ErrorCodeEventSchemaInvalid},
	{base_value_objects.ErrIdentifierHasControlCharacters, ErrorCodeEventSchemaInvalid},
	{base_value_objects.ErrIdentifierNotUTF8, ErrorCodeEventSchemaInvalid},
	{base_value_objects.ErrHexDigitsInvalid, ErrorCodeEventSchemaInvalid},
	{base_value_objects.ErrHexDigitsWrongWidth, ErrorCodeEventSchemaInvalid},
	{domain.ErrNumberOutOfRange, ErrorCodeEventSchemaInvalid},
	{domain.ErrUnsupportedValue, ErrorCodeEventSchemaInvalid},
	{domain.ErrNonUTCTime, ErrorCodeEventSchemaInvalid},
	{domain.ErrEventSchemaInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrUTCInstantTextInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrSeverityInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrActorKindInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrEventIDInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrResourceServiceNameInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrResourceInstanceIDInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrResourceEnvironmentInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrResourceServiceVersionInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrResourceHostInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrTimeZoneInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrActorIDInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrActorTypeInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrActorVersionInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrContextTraceIDInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrContextSpanIDInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrContextExecutionIDInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrContextCorrelationIDInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrAttributeKeyInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrAttributeValueInvalid, ErrorCodeEventSchemaInvalid},
	{domain.ErrEventTypeUnknown, ErrorCodeEventTypeUnknown},
	{domain.ErrEventTooLarge, ErrorCodeLimitExceeded},
	{domain.ErrLimitInvalid, ErrorCodeConfigFieldInvalid},
	{domain.ErrBatchLimitBelowEventLimit, ErrorCodeConfigFieldInvalid},
	{domain.ErrMaskPatternInvalid, ErrorCodeConfigFieldInvalid},
	{domain.ErrMaskRuleInvalid, ErrorCodeConfigFieldInvalid},
	{domain.ErrConfigFieldInvalid, ErrorCodeConfigFieldInvalid},
	{domain.ErrDeliveryModeUnknown, ErrorCodeDeliveryModeUnknown},
}

// CodeOf returns the code of the catalog of an error of the core, or an empty text when it has none. The code of the
// first *domain.ClassifiedError in the chain (errors.As) always wins, because it is the explicit one; otherwise it is
// the code of the first sentinel of the domain that errors.Is finds in the chain. The result is a code, never a value
// that a caller passed. A nil error has no code. It never panics.
//
// It lives here and not in the domain root because the codes are the constants generated in this package, and this
// package imports the domain root and base_value_objects, not the other way round.
func CodeOf(err error) string {
	if err == nil {
		return ""
	}
	var classified *domain.ClassifiedError
	if errors.As(err, &classified) {
		return classified.Code
	}
	for _, entry := range sentinel_codes {
		if errors.Is(err, entry.sentinel) {
			return entry.code
		}
	}
	return ""
}
