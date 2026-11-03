package error_test

import (
	"errors"
	"testing"

	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http/error"
)

func Test_the_destination_error_joins_the_sentinel_and_the_cause(t *testing.T) {
	cause := errors.New("the cause")
	mapped := adapter_error.MapDestinationError(cause)
	if !errors.Is(mapped, adapter_error.ErrDestinationUnusable) || !errors.Is(mapped, cause) {
		t.Fatalf("%v", mapped)
	}
	if !errors.Is(adapter_error.MapDestinationError(nil), adapter_error.ErrDestinationUnusable) {
		t.Fatal("a nil cause keeps the sentinel")
	}
}

func Test_the_translation_error_joins_the_sentinel_and_the_cause(t *testing.T) {
	cause := errors.New("the cause")
	mapped := adapter_error.MapTranslationError(cause)
	if !errors.Is(mapped, adapter_error.ErrEventNotTranslatable) || !errors.Is(mapped, cause) {
		t.Fatalf("%v", mapped)
	}
	if !errors.Is(adapter_error.MapTranslationError(nil), adapter_error.ErrEventNotTranslatable) {
		t.Fatal("a nil cause keeps the sentinel")
	}
}

func Test_the_reasons_are_fixed_words(t *testing.T) {
	if adapter_error.ReasonDestinationFailed != "destination_failed" || adapter_error.ReasonTranslationFailed != "serialization_failed" {
		t.Fatal("the reasons are part of the contract of the results")
	}
}
