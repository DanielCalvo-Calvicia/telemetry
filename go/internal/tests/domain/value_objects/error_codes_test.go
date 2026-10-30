package value_objects_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

func Test_a_nil_error_has_no_code(t *testing.T) {
	if code := value_objects.CodeOf(nil); code != "" {
		t.Errorf("CodeOf(nil) = %q, want an empty text", code)
	}
}

func Test_an_error_that_is_not_of_the_core_has_no_code(t *testing.T) {
	if code := value_objects.CodeOf(errors.New("something else")); code != "" {
		t.Errorf("CodeOf = %q, want an empty text", code)
	}
}

func Test_a_sentinel_of_the_domain_or_of_the_base_has_its_code(t *testing.T) {
	cases := map[error]string{
		domain.ErrEventTypeUnknown:                value_objects.ErrorCodeEventTypeUnknown,
		domain.ErrEventTooLarge:                   value_objects.ErrorCodeLimitExceeded,
		domain.ErrDeliveryModeUnknown:             value_objects.ErrorCodeDeliveryModeUnknown,
		domain.ErrLimitInvalid:                    value_objects.ErrorCodeConfigFieldInvalid,
		domain.ErrMaskRuleInvalid:                 value_objects.ErrorCodeConfigFieldInvalid,
		domain.ErrSeverityInvalid:                 value_objects.ErrorCodeEventSchemaInvalid,
		base_value_objects.ErrIdentifierBlank:     value_objects.ErrorCodeEventSchemaInvalid,
		base_value_objects.ErrHexDigitsWrongWidth: value_objects.ErrorCodeEventSchemaInvalid,
	}
	for sentinel, expected_code := range cases {
		if code := value_objects.CodeOf(sentinel); code != expected_code {
			t.Errorf("CodeOf(%v) = %q, want %q", sentinel, code, expected_code)
		}
	}
}

func Test_a_classified_error_wins_over_the_sentinel_it_wraps(t *testing.T) {
	classified := domain.NewClassifiedError(value_objects.ErrorCodeLimitsFailed, domain.ErrSeverityInvalid, nil)
	if code := value_objects.CodeOf(classified); code != value_objects.ErrorCodeLimitsFailed {
		t.Errorf("CodeOf = %q, want the explicit code limits_failed", code)
	}
}

func Test_a_joined_error_gives_the_code_of_the_cause_it_holds(t *testing.T) {
	joined := errors.Join(errors.New("first"), domain.ErrBatchLimitBelowEventLimit)
	if code := value_objects.CodeOf(joined); code != value_objects.ErrorCodeConfigFieldInvalid {
		t.Errorf("CodeOf = %q, want config_field_invalid", code)
	}
	joined_classified := errors.Join(errors.New("first"), domain.NewClassifiedError(value_objects.ErrorCodeMaskingFailed, domain.ErrEventTooLarge, nil))
	if code := value_objects.CodeOf(joined_classified); code != value_objects.ErrorCodeMaskingFailed {
		t.Errorf("CodeOf = %q, want masking_failed", code)
	}
}

func Test_every_sentinel_of_the_domain_has_a_code(t *testing.T) {
	sentinels := []error{
		domain.ErrNumberOutOfRange, domain.ErrUnsupportedValue, domain.ErrNonUTCTime, domain.ErrEventSchemaInvalid,
		domain.ErrEventTypeUnknown, domain.ErrEventTooLarge, domain.ErrMaskPatternInvalid, domain.ErrMaskRuleInvalid,
		domain.ErrLimitInvalid, domain.ErrBatchLimitBelowEventLimit, domain.ErrConfigFieldInvalid,
		domain.ErrDeliveryModeUnknown, domain.ErrUTCInstantTextInvalid, domain.ErrSeverityInvalid,
		domain.ErrActorKindInvalid, domain.ErrEventIDInvalid, domain.ErrResourceServiceNameInvalid,
		domain.ErrResourceInstanceIDInvalid, domain.ErrResourceEnvironmentInvalid,
		domain.ErrResourceServiceVersionInvalid, domain.ErrResourceHostInvalid, domain.ErrTimeZoneInvalid,
		domain.ErrActorIDInvalid, domain.ErrActorTypeInvalid, domain.ErrActorVersionInvalid,
		domain.ErrContextTraceIDInvalid, domain.ErrContextSpanIDInvalid, domain.ErrContextExecutionIDInvalid,
		domain.ErrContextCorrelationIDInvalid, domain.ErrAttributeKeyInvalid, domain.ErrAttributeValueInvalid,
		base_value_objects.ErrIdentifierBlank, base_value_objects.ErrIdentifierHasControlCharacters,
		base_value_objects.ErrIdentifierNotUTF8, base_value_objects.ErrHexDigitsInvalid,
		base_value_objects.ErrHexDigitsWrongWidth,
	}
	known_codes := map[string]bool{}
	for _, info := range value_objects.ErrorCodes() {
		known_codes[info.Code] = true
	}
	for _, sentinel := range sentinels {
		if code := value_objects.CodeOf(sentinel); !known_codes[code] {
			t.Errorf("the sentinel %v has the code %q, which is not in the catalog", sentinel, code)
		}
	}
}
