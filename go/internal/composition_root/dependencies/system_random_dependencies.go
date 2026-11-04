package dependencies

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/system_random"
)

// NewSystemRandom builds the default random source of the core: the one of the machine.
func NewSystemRandom() outbound.RandomPort {
	return system_random.NewSystemRandom()
}
