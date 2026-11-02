// Package helpers holds the pure pieces of the shared HTTP client: the batch body, the compression, the classification
// of a response, Retry-After, the backoff and the authentication header. Nothing here touches the network.
package helpers

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// BUILD BATCH BODY
// =============================================================================

// batch_body is the object that an HTTP batch carries (REQ-OUT-010).
type batch_body struct {
	SchemaVersion string                   `json:"schema_version"`
	Events        []value_objects.Envelope `json:"events"`
}

// BuildBatchBody returns the body of an HTTP batch: the canonical JSON of the object {schema_version, events}, so both
// languages give the same bytes. An empty batch has events as an empty list.
func BuildBatchBody(events []value_objects.Envelope) ([]byte, error) {
	if events == nil {
		events = []value_objects.Envelope{}
	}
	return canonical_json.CanonicalJSON(batch_body{SchemaVersion: domain.SchemaVersion, Events: events})
}
