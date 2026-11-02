// Package helpers holds the pure pieces of the HTTP destination: the mapping from the outcome of the shared client to
// the result of the port. Nothing here touches the network.
package helpers

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
	client_helpers "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/helpers"
)

// =============================================================================
// RESULT FROM OUTCOME
// =============================================================================

// ResultFromOutcome maps what the shared client gave for one batch to the result of the port. The filtered events were
// not sent and count as accepted. For the sent events: a success accepts them; a transient outcome fails them (the
// worker may retry them) with the reason of the outcome; a permanent outcome drops them (they are not retried) with the
// reason of the outcome. The reason is empty when every event was accepted.
func ResultFromOutcome(outcome http_client.Outcome, sent int, filtered int) dtos.SendResult {
	result := dtos.SendResult{Accepted: filtered}
	switch outcome.Class {
	case client_helpers.ClassSuccess:
		result.Accepted += sent
	case client_helpers.ClassPermanent:
		result.Dropped = sent
		result.Reason = outcome.Reason
	default:
		result.Failed = sent
		result.Reason = outcome.Reason
	}
	if sent == 0 {
		result.Reason = ""
	}
	return result
}
