// Package aws_http is the adapter that sends the log events of the core to the add_log endpoint of aws_microservice
// (ADR-008). It lives in a module of its own (ADR-013) and is built on the shared HTTP client of the core: it adds only the
// semantics of the payload, which events are translated and how the outcome of each request counts them. A log.* event
// becomes one add_log request (the entry of the old contract and a timestamp in milliseconds); an event of any other
// type is skipped, because the endpoint has no place for it. The adapter does not queue and does not retry on its own
// (the queue is T-05.1 and the retry T-05.3); a failure is a value, never a panic or an error to the caller (ADR-005).
// It has no external/ folder because it does no I/O itself: all of it is in the shared client. The project passes the
// destination to the core in code; the adapter does not register itself. The package reads no environment variable and
// no file: the project passes the Settings in.
package aws_http

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
	client_helpers "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/helpers"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http/error"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http/helpers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

// =============================================================================
// AWS HTTP DESTINATION
// =============================================================================

// Destination implements outbound.LogSinkPort and outbound.EventSinkPort of the core: it sends each log.* event of a call
// as one add_log request to the endpoint of its settings. It holds no queue and no goroutine, and it is safe for
// concurrent use. The token and the key live only in its client and never appear in a result or an error.
type Destination struct {
	client          *http_client.Client
	log_group_name  string
	log_stream_name string
}

var (
	_ outbound.LogSinkPort   = (*Destination)(nil)
	_ outbound.EventSinkPort = (*Destination)(nil)
)

// Delivery is what Deliver answers: the result of the port, the outcome of the last request that was made (the zero value
// when none was made), the number of events that were handed to the shared client (one request each) and the number of
// events that were skipped because they are not log events. A worker that retries uses the Outcome and NextDelay.
type Delivery struct {
	Result  dtos.SendResult
	Outcome http_client.Outcome
	Sent    int
	Skipped int
}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewDestination validates the settings and returns a destination with the default limits of the core. An invalid
// setting returns a *telemetry.ValidationError with every problem (the five required settings are always required).
// Pass the result to telemetry.WithLogSink.
func NewDestination(settings Settings) (*Destination, error) {
	return NewDestinationWithLimits(settings, value_objects.DefaultLimits())
}

// NewDestinationWithLimits is NewDestination with the limits of the core: a request larger than the batch limit is not
// sent and its event is dropped with the reason batch_too_large. An invalid setting returns the *config.ValidationError
// of ValidateSettings as it is.
func NewDestinationWithLimits(settings Settings, limits telemetry.Limits) (*Destination, error) {
	if validation_error := ValidateSettings(settings); validation_error != nil {
		return nil, validation_error
	}
	client, client_error := http_client.NewClient(settings.ClientSettings(), limits)
	if client_error != nil {
		return nil, client_error
	}
	return &Destination{client: client, log_group_name: settings.LogGroupName, log_stream_name: settings.LogStreamName}, nil
}

// ----- SEND -----------------------------------------------------------------

// Send sends the events and counts each event once; it is Deliver without the details. It never panics and never returns
// an error: see Deliver.
func (destination *Destination) Send(ctx context.Context, events []value_objects.Envelope) dtos.SendResult {
	return destination.Deliver(ctx, events).Result
}

// Deliver sends each log.* event as ONE add_log request, in order, and says how it went. An event whose type does not
// start with "log." is skipped: no request is made and it counts as accepted (the adapter took it and has no place for
// it) and in Delivery.Skipped. For a translated event a 2xx accepts it; a transient outcome (408, 429, 5xx, a timeout, a
// network error) fails it; a permanent one (any other status, a request over the limit) drops it; an event that cannot be
// translated is dropped with the reason serialization_failed and no request. The Result.Reason is the reason of the first
// event that was not accepted, and Delivery.Outcome is the outcome of the last request made.
// Result.RetryAfterMS is the Retry-After of the last event that was not accepted, when its failure was transient and
// carried one (0 otherwise). It makes no retry. A panic
// inside the destination is recovered: the events not counted yet are failed with the reason destination_failed. A nil
// destination counts every event as failed with that reason.
func (destination *Destination) Deliver(ctx context.Context, events []value_objects.Envelope) (delivery Delivery) {
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

	count_reason := func(reason string) {
		if delivery.Result.Reason == "" {
			delivery.Result.Reason = reason
		}
	}
	for _, event := range events {
		if !helpers.IsLogEvent(event.EventType) {
			delivery.Skipped++
			delivery.Result.Accepted++
			continue
		}
		body, build_error := helpers.BuildAddLogRequest(event, destination.log_group_name, destination.log_stream_name)
		if build_error != nil {
			delivery.Result.Dropped++
			count_reason(adapter_error.ReasonTranslationFailed)
			continue
		}
		delivery.Sent++
		delivery.Outcome = destination.client.Send(ctx, body)
		switch delivery.Outcome.Class {
		case client_helpers.ClassSuccess:
			delivery.Result.Accepted++
		case client_helpers.ClassPermanent:
			delivery.Result.Dropped++
			count_reason(delivery.Outcome.Reason)
			delivery.Result.RetryAfterMS = 0
		default:
			delivery.Result.Failed++
			count_reason(delivery.Outcome.Reason)
			delivery.Result.RetryAfterMS = 0
			if delivery.Outcome.RetryAfterOK {
				delivery.Result.RetryAfterMS = delivery.Outcome.RetryAfterMilliseconds
			}
		}
	}
	return delivery
}

// NextDelay says what to do after the attempt number attempt (0 for the first one) that gave outcome: retry is true only
// for a transient outcome while there are attempts left, and delay_ms is the larger of the backoff with jitter and the
// Retry-After of the outcome. random is in [0, 1]. It delegates to the shared client; a nil destination never retries.
func (destination *Destination) NextDelay(attempt int, outcome http_client.Outcome, random float64) (delay_ms int64, retry bool) {
	if destination == nil {
		return 0, false
	}
	return destination.client.NextDelay(attempt, outcome, random)
}
