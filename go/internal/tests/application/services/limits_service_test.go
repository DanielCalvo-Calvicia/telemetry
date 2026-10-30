package services_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/limits"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

func limits_service_envelope(attributes value_objects.Attributes) value_objects.Envelope {
	return value_objects.Envelope{
		SchemaVersion: "0.1",
		EventId:       "01900000-0000-7000-8000-000000000001",
		EventType:     "log.entry.recorded",
		OccurredAt:    "2026-10-28T10:00:00.123456Z",
		RecordedAt:    "2026-10-28T10:00:00.123456Z",
		Severity:      "INFO",
		Resource:      value_objects.Resource{ServiceName: "sensor-hub", InstanceId: "hub-01"},
		Data:          map[string]any{"category": "c", "message": "m"},
		Attributes:    attributes,
	}
}

func Test_the_limits_service_cuts_and_reports(t *testing.T) {
	service := services.NewLimitsService(value_objects.DefaultLimits())
	envelope := limits_service_envelope(value_objects.Attributes{"k": strings.Repeat("v", 257)})
	limited, report, apply_error := service.Apply(context.Background(), envelope)
	if apply_error != nil || limited.Attributes["k"] != strings.Repeat("v", 256) || report.AttributeValuesTruncated != 1 {
		t.Errorf("got %v, %+v, %v", limited.Attributes, report, apply_error)
	}
	if envelope.Attributes["k"] != strings.Repeat("v", 257) {
		t.Error("the envelope that was passed in was modified")
	}
}

func Test_the_limits_service_returns_the_rejection_of_a_large_event(t *testing.T) {
	small_limits, limits_error := value_objects.NewLimits(32, 64, 256, 10, 10)
	if limits_error != nil {
		t.Fatal(limits_error)
	}
	limited, report, apply_error := services.NewLimitsService(small_limits).Apply(context.Background(), limits_service_envelope(nil))
	var classified *domain.ClassifiedError
	if !errors.As(apply_error, &classified) || classified.Code != value_objects.ErrorCodeLimitExceeded {
		t.Fatalf("error = %v", apply_error)
	}
	if !reflect.DeepEqual(limited, value_objects.Envelope{}) || report.EventsRejected != 1 {
		t.Errorf("got %+v, %+v", limited, report)
	}
}

func Test_the_limits_service_never_panics_and_does_not_keep_the_panic_text(t *testing.T) {
	var service *services.LimitsService // a nil service panics inside Apply
	limited, report, apply_error := service.Apply(context.Background(), limits_service_envelope(nil))
	if !errors.Is(apply_error, application.ErrEnvelopeLimitsFailed) {
		t.Fatalf("error = %v, want ErrEnvelopeLimitsFailed", apply_error)
	}
	if text := apply_error.Error(); strings.Contains(text, "nil pointer") || strings.Contains(text, "runtime") {
		t.Errorf("the error must not hold the panic value: %s", text)
	}
	if !reflect.DeepEqual(limited, value_objects.Envelope{}) || report != (limits.Report{}) {
		t.Errorf("got %+v, %+v", limited, report)
	}
}
