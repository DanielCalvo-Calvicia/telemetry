// Package http_destination is the adapter that sends batches of events to the URL the consumer configures, with a
// minimum severity (REQ-OUT-002). It is built on the shared HTTP client of T-04.4 and adds only the semantics of the
// payload: which events are sent and how the outcome of the client counts them. It does not queue and does not retry on
// its own (the queue is T-05.1 and the retry T-05.3); a failure is a value, never a panic or an error to the caller
// (ADR-005). It has no external/ folder because it does no I/O itself: all of it is in the shared client. The folder is
// called http and the package http_destination so that it does not shadow net/http.
package http_destination

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/http/error"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/http/helpers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
)

// =============================================================================
// HTTP DESTINATION
// =============================================================================

// HTTPDestination implements outbound.LogSinkPort and outbound.EventSinkPort: it sends the events of a call as one batch
// to the URL of its settings. It holds no queue and no goroutine, and it is safe for concurrent use. The token and the
// key live only in its client and never appear in a result or an error.
type HTTPDestination struct {
	client           *http_client.Client
	minimum_severity value_objects.Severity
}

var (
	_ outbound.LogSinkPort   = (*HTTPDestination)(nil)
	_ outbound.EventSinkPort = (*HTTPDestination)(nil)
)

// Delivery is what Deliver answers: the result of the port, the outcome of the request that the shared client made (the
// zero value when no request was made), the number of events that were sent and the number that the minimum severity
// filtered. A worker that retries uses the Outcome and NextDelay.
type Delivery struct {
	Result   dtos.SendResult
	Outcome  http_client.Outcome
	Sent     int
	Filtered int
}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewHTTPDestination validates the settings and returns a destination. An invalid setting returns the
// *config.ValidationError of ValidateSettings as it is. The limits give the size of a batch that is never exceeded.
func NewHTTPDestination(settings Settings, limits value_objects.Limits) (*HTTPDestination, error) {
	if validation_error := ValidateSettings(settings); validation_error != nil {
		return nil, validation_error
	}
	client, client_error := http_client.NewClient(settings.ClientSettings(), limits)
	if client_error != nil {
		return nil, client_error
	}
	return &HTTPDestination{client: client, minimum_severity: value_objects.Severity(settings.MinimumSeverity)}, nil
}

// ----- SEND -----------------------------------------------------------------

// Send sends the events as one batch and counts each event once; it is Deliver without the details. It never panics and
// never returns an error: see Deliver.
func (destination *HTTPDestination) Send(ctx context.Context, events []value_objects.Envelope) dtos.SendResult {
	return destination.Deliver(ctx, events).Result
}

// Deliver sends the events that reach the minimum severity as ONE request and says how it went. An event below the
// minimum severity is not sent and counts as accepted: the destination took it and its policy decided not to send it.
// When no event is left (all filtered, or none given) no request is made. A 2xx accepts the sent events; a transient
// outcome (408, 429, 5xx, a timeout, a network error) fails them with the reason of the outcome; a permanent one (any
// other status, a batch over the limit) drops them with that reason. It makes no retry. A panic inside the destination
// (a value that panics while it is serialized, for example) is recovered: the events not counted yet are failed with the
// reason destination_failed. A nil destination counts every event as failed with that reason.
func (destination *HTTPDestination) Deliver(ctx context.Context, events []value_objects.Envelope) (delivery Delivery) {
	if destination == nil {
		return Delivery{Result: dtos.SendResult{Failed: len(events), Reason: adapter_error.ReasonDestinationFailed}}
	}
	defer func() {
		if recover() != nil {
			not_counted := len(events) - delivery.Result.Accepted - delivery.Result.Dropped - delivery.Result.Failed
			delivery.Result.Failed += not_counted
			delivery.Result.Reason = adapter_error.ReasonDestinationFailed
		}
	}()

	events_to_send := make([]value_objects.Envelope, 0, len(events))
	for _, event := range events {
		if value_objects.SeverityAtLeast(event.Severity, destination.minimum_severity) {
			events_to_send = append(events_to_send, event)
		}
	}
	delivery.Sent = len(events_to_send)
	delivery.Filtered = len(events) - delivery.Sent
	if delivery.Sent == 0 {
		delivery.Result = dtos.SendResult{Accepted: delivery.Filtered}
		return delivery
	}
	delivery.Outcome = destination.client.SendBatch(ctx, events_to_send)
	delivery.Result = helpers.ResultFromOutcome(delivery.Outcome, delivery.Sent, delivery.Filtered)
	return delivery
}

// NextDelay says what to do after the attempt number attempt (0 for the first one) that gave outcome: retry is true only
// for a transient outcome while there are attempts left, and delay_ms is the larger of the backoff with jitter and the
// Retry-After of the outcome. random is in [0, 1]. It delegates to the shared client; a nil destination never retries.
func (destination *HTTPDestination) NextDelay(attempt int, outcome http_client.Outcome, random float64) (delay_ms int64, retry bool) {
	if destination == nil {
		return 0, false
	}
	return destination.client.NextDelay(attempt, outcome, random)
}
