package value_objects_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

func Test_a_delivery_mode_accepts_exactly_the_values_of_the_contract(t *testing.T) {
	for _, text := range []string{"best_effort", "retry", "durable"} {
		delivery_mode, mode_error := value_objects.NewDeliveryMode(text)
		if mode_error != nil || string(delivery_mode) != text {
			t.Errorf("%q: got %q, %v", text, delivery_mode, mode_error)
		}
		if validation_error := value_objects.ValidateDeliveryMode(text); validation_error != nil {
			t.Errorf("%q: %v", text, validation_error)
		}
	}
	if len(value_objects.DeliveryModeValues()) != 3 {
		t.Errorf("the contract has %d values, expected 3", len(value_objects.DeliveryModeValues()))
	}
}

func Test_a_delivery_mode_rejects_every_other_text_with_the_sentinel_unwrapped(t *testing.T) {
	for _, text := range []string{"", "x", "Durable", "best-effort", " retry", "retry ", "durable\n"} {
		delivery_mode, mode_error := value_objects.NewDeliveryMode(text)
		if mode_error != domain.ErrDeliveryModeUnknown || delivery_mode != "" {
			t.Errorf("%q: got %q, %v", text, delivery_mode, mode_error)
		}
		if !errors.Is(value_objects.ValidateDeliveryMode(text), value_objects.ErrDeliveryModeUnknown) {
			t.Errorf("%q: the sentinel is not found", text)
		}
	}
}
