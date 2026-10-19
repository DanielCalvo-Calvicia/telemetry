package dependencies

import (
	systemclock "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/systemclock"
)

// NewSystemClockDependency builds the clock of the logger: the time of the system.
func NewSystemClockDependency() *systemclock.SystemClock {
	return systemclock.NewSystemClock()
}
