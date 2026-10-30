package value_objects_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

func Test_the_default_limits_are_exactly_the_ones_of_the_contract(t *testing.T) {
	defaults := value_objects.DefaultLimits()
	if defaults.MaxAttributeKeys() != 32 || defaults.MaxAttributeKeyChars() != 64 || defaults.MaxAttributeValueChars() != 256 ||
		defaults.MaxEventBytes() != 65536 || defaults.MaxBatchBytes() != 1048576 {
		t.Errorf("defaults = %d/%d/%d/%d/%d", defaults.MaxAttributeKeys(), defaults.MaxAttributeKeyChars(),
			defaults.MaxAttributeValueChars(), defaults.MaxEventBytes(), defaults.MaxBatchBytes())
	}
}

func Test_new_limits_keeps_the_numbers_and_accepts_a_batch_equal_to_the_event(t *testing.T) {
	limits, limits_error := value_objects.NewLimits(1, 2, 3, 4, 4)
	if limits_error != nil {
		t.Fatal(limits_error)
	}
	if limits.MaxAttributeKeys() != 1 || limits.MaxAttributeKeyChars() != 2 || limits.MaxAttributeValueChars() != 3 ||
		limits.MaxEventBytes() != 4 || limits.MaxBatchBytes() != 4 {
		t.Errorf("limits = %+v", limits)
	}
}

func Test_new_limits_refuses_a_limit_that_is_not_greater_than_zero_and_a_batch_below_the_event(t *testing.T) {
	cases := []struct {
		name     string
		numbers  [5]int
		sentinel error
	}{
		{"zero keys", [5]int{0, 64, 256, 100, 200}, domain.ErrLimitInvalid},
		{"zero key chars", [5]int{1, 0, 256, 100, 200}, domain.ErrLimitInvalid},
		{"negative value chars", [5]int{1, 1, -1, 100, 200}, domain.ErrLimitInvalid},
		{"zero event bytes", [5]int{1, 1, 1, 0, 200}, domain.ErrLimitInvalid},
		{"negative batch bytes", [5]int{1, 1, 1, 100, -5}, domain.ErrLimitInvalid},
		{"batch below event", [5]int{1, 1, 1, 100, 99}, domain.ErrBatchLimitBelowEventLimit},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			limits, limits_error := value_objects.NewLimits(current.numbers[0], current.numbers[1], current.numbers[2], current.numbers[3], current.numbers[4])
			if !errors.Is(limits_error, current.sentinel) {
				t.Fatalf("error = %v, want %v", limits_error, current.sentinel)
			}
			var classified *domain.ClassifiedError
			if !errors.As(limits_error, &classified) || classified.Code != value_objects.ErrorCodeConfigFieldInvalid {
				t.Errorf("error = %v, want the code config_field_invalid", limits_error)
			}
			if limits != (value_objects.Limits{}) {
				t.Errorf("the limits of a refused call must be the zero value, got %+v", limits)
			}
		})
	}
}

func Test_the_zero_value_of_limits_is_not_the_defaults(t *testing.T) {
	if (value_objects.Limits{}) == value_objects.DefaultLimits() {
		t.Error("the zero value must not be the defaults")
	}
	if value_objects.ErrLimitInvalid != domain.ErrLimitInvalid || value_objects.ErrBatchLimitBelowEventLimit != domain.ErrBatchLimitBelowEventLimit {
		t.Error("the sentinels of value_objects must be the ones of the domain")
	}
}
