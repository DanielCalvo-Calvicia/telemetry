// Package error translates the failures of the UUID generator to the errors of the application.
package error

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// MapGenerateError translates a failure of the random source into a classified error event_build_failed that wraps
// application.ErrIDGenerationFailed and keeps the cause, so errors.Is finds both.
func MapGenerateError(cause error) error {
	return domain.NewClassifiedError(value_objects.ErrorCodeEventBuildFailed, application.ErrIDGenerationFailed, cause)
}
