package dependencies

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	http_destination "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/http"
)

// NewHTTPDestination builds the HTTP destination: the shared client with the settings, and the limits that cap the size
// of a batch. An invalid setting returns the *config.ValidationError with every problem.
func NewHTTPDestination(settings http_destination.Settings, limits value_objects.Limits) (*http_destination.HTTPDestination, error) {
	destination, build_error := http_destination.NewHTTPDestination(settings, limits)
	if build_error != nil {
		return nil, build_error
	}
	return destination, nil
}
