package helpers

import (
	"encoding/json"

	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
)

// EncodeLogEntry encodes an entry as JSON with the flat field names of the log payload.
func EncodeLogEntry(log_entry dtos.LogEntryOutboundDTO) ([]byte, error) {
	return json.Marshal(log_entry)
}
