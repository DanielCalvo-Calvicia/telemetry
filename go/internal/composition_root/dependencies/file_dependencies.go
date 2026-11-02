package dependencies

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file"
)

// NewFileDestination builds the file destination: it validates the settings, creates the directory and checks that it can
// be written. An invalid setting or a directory that cannot be written returns the *config.ValidationError with its
// problems. The concrete type is returned so that its Counters are reachable.
func NewFileDestination(settings file.Settings) (*file.FileDestination, error) {
	return file.NewFileDestination(settings)
}
