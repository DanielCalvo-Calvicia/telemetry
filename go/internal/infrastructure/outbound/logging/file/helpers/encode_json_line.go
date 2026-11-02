// Package helpers holds the pure functions of the file destination: the line encoder, the plan of a rotation and the
// plan of a quota. They do no I/O: the destination gives them names and sizes and runs the steps they return.
package helpers

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// ENCODE JSON LINE
// =============================================================================

// EncodeJSONLine returns the line of an event: the canonical JSON of the whole envelope and a line feed. The bytes are
// the same in Go and in Python for the same event, and the same as the json format of the console.
func EncodeJSONLine(event value_objects.Envelope) ([]byte, error) {
	canonical_bytes, encode_error := canonical_json.CanonicalJSON(event)
	if encode_error != nil {
		return nil, encode_error
	}
	return append(canonical_bytes, '\n'), nil
}
