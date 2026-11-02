// Package helpers holds the pure functions of the console destination: the two encoders of an event to a line. They do
// no I/O.
package helpers

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// ENCODE JSON LINE
// =============================================================================

// EncodeJSONLine returns the line of the json format: the canonical JSON of the whole envelope and a line feed. The
// bytes are the same in Go and in Python for the same event.
func EncodeJSONLine(event value_objects.Envelope) ([]byte, error) {
	canonical_bytes, encode_error := canonical_json.CanonicalJSON(event)
	if encode_error != nil {
		return nil, encode_error
	}
	return append(canonical_bytes, '\n'), nil
}
