// Package http_client is the shared HTTP infrastructure that every HTTP adapter reuses (REQ-OUT-005, REQ-OUT-006): the
// settings and their validation, the batch body, the authentication header, the classification of a response,
// Retry-After, the backoff with jitter and a client that sends one body and returns a classified outcome. The calls to
// net/http live in external/ and the translation of the failures to fixed reasons in error/.
package http_client

import (
	"context"
	"fmt"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/retry_delay"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/error"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/external"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/helpers"
)

// =============================================================================
// HTTP CLIENT
// =============================================================================

const (
	// ReasonBatchTooLarge is the reason of a body over the batch limit: it is not sent.
	ReasonBatchTooLarge = "batch_too_large"
	// ReasonSerializationFailed is the reason of a batch that could not be turned into a body.
	ReasonSerializationFailed = "serialization_failed"
	// ReasonCompressionFailed is the reason of a body that could not be compressed.
	ReasonCompressionFailed = "compression_failed"
	// ReasonClientFailed is the reason of an unexpected failure of the client itself (a recovered panic).
	ReasonClientFailed = "client_failed"
)

// Outcome is what one attempt to send a body gave. Reason is empty on success, status_<code> for a response that is
// not 2xx, and one of the fixed reasons otherwise (timeout, tls_verification_failed, network_error,
// response_unreadable, batch_too_large, serialization_failed, compression_failed, client_failed). It never holds a
// configured value: no URL, no token and no key.
type Outcome struct {
	Class                  helpers.Class
	StatusCode             int
	RetryAfterMilliseconds int64
	RetryAfterOK           bool
	Reason                 string
}

// Client sends one body to the destination of its settings and returns a classified Outcome. It holds no global state
// and is safe for concurrent use. It does not retry by itself: the caller asks NextDelay what to do after each
// attempt. Rate limiting is not implemented; a Retry-After of the destination is honored through NextDelay.
type Client struct {
	settings Settings
	limits   value_objects.Limits
}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewClient returns a Client for the settings. It validates them first and returns the *config.ValidationError as it
// is when they are not valid. The limits give the size of a batch that is never exceeded (MaxBatchBytes).
func NewClient(settings Settings, limits value_objects.Limits) (*Client, error) {
	if validation_error := ValidateSettings(settings); validation_error != nil {
		return nil, validation_error
	}
	return &Client{settings: settings, limits: limits}, nil
}

// ----- METHODS --------------------------------------------------------------

// MaxAttempts returns the number of attempts at most for one batch, the first included.
func (client *Client) MaxAttempts() int {
	return client.settings.MaxAttempts
}

// Send sends one body. The body is gzipped when the settings say so. A body larger than the batch limit (counted plain,
// before the compression) is not sent: the outcome is permanent with the reason batch_too_large and there is no network
// call. Otherwise it makes one request: a 2xx is success, any other status has the reason status_<code>, a failure of
// the transport is transient with a fixed reason. Send never panics: an unexpected failure is transient with the reason
// client_failed.
func (client *Client) Send(ctx context.Context, body []byte) (outcome Outcome) {
	defer func() {
		if recover() != nil {
			outcome = Outcome{Class: helpers.ClassTransient, Reason: ReasonClientFailed}
		}
	}()
	if len(body) > client.limits.MaxBatchBytes() {
		return Outcome{Class: helpers.ClassPermanent, Reason: ReasonBatchTooLarge}
	}
	body_to_send := body
	if client.settings.Compress {
		compressed_body, compress_error := helpers.CompressGzip(body)
		if compress_error != nil {
			return Outcome{Class: helpers.ClassPermanent, Reason: ReasonCompressionFailed}
		}
		body_to_send = compressed_body
	}
	auth_name, auth_value, _ := helpers.AuthHeader(client.settings.BearerToken, client.settings.APIKeyHeader, client.settings.APIKeyValue)
	response := external.PostRequest(ctx, external.Request{
		URL:                 client.settings.URL,
		Method:              client.settings.Method,
		TimeoutMilliseconds: client.settings.TimeoutMS,
		SkipTLSVerification: client.settings.SkipTLSVerification,
		AuthHeaderName:      auth_name,
		AuthHeaderValue:     auth_value,
		Compressed:          client.settings.Compress,
		Body:                body_to_send,
	})
	return outcome_of(response)
}

// SendBatch builds the body of the events (the canonical JSON of {schema_version, events}) and sends it with Send. A
// failure to build it is permanent with the reason serialization_failed.
func (client *Client) SendBatch(ctx context.Context, events []value_objects.Envelope) Outcome {
	body, build_error := helpers.BuildBatchBody(events)
	if build_error != nil {
		return Outcome{Class: helpers.ClassPermanent, Reason: ReasonSerializationFailed}
	}
	return client.Send(ctx, body)
}

// NextDelay says what to do after the attempt number attempt (0 for the first one) that gave outcome: retry is true
// only for a transient outcome while attempt+1 is below MaxAttempts, and delay_ms is the larger of the backoff with
// jitter and the Retry-After of the outcome. random is in [0, 1]; the caller gives it so the policy can be tested.
func (client *Client) NextDelay(attempt int, outcome Outcome, random float64) (delay_ms int64, retry bool) {
	return retry_delay.RetryDelay(attempt, client.settings.MaxAttempts, outcome.Class == helpers.ClassTransient, outcome.RetryAfterMilliseconds, outcome.RetryAfterOK,
		client.settings.RetryBaseDelayMS, client.settings.RetryMaxDelayMS, random)
}

// ----- HELPERS --------------------------------------------------------------

// outcome_of turns what came back into an Outcome. No text of a transport error is copied: only fixed reasons.
func outcome_of(response external.Response) Outcome {
	if response.TransportErr != nil {
		return Outcome{Class: helpers.ClassTransient, Reason: adapter_error.MapTransportError(response.TransportErr).Reason()}
	}
	outcome := Outcome{Class: helpers.ClassifyStatus(response.StatusCode), StatusCode: response.StatusCode}
	if response.RetryAfter != "" {
		outcome.RetryAfterMilliseconds, outcome.RetryAfterOK = helpers.ParseRetryAfter(response.RetryAfter, time.Now().UTC())
	}
	if response.BodyReadErr != nil {
		outcome.Class = helpers.ClassTransient
		outcome.Reason = adapter_error.MapReadError(response.BodyReadErr).Reason()
		return outcome
	}
	if outcome.Class != helpers.ClassSuccess {
		outcome.Reason = fmt.Sprintf("status_%d", response.StatusCode)
	}
	return outcome
}
