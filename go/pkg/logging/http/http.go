// Package http is the public HTTP destination of the telemetry library: it sends the events of a core as a batch to the
// URL the project configures, with a minimum severity (the default destination of the library). A project builds it with
// NewDestination and passes it to the core: telemetry.New(config, telemetry.WithLogSink(destination)). It does not queue
// and does not retry by itself, and a failure of the receiver never reaches the caller of a log call: it is a result.
// The package reads no environment variable and no file: the project passes the Settings in.
package http

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/dependencies"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	internal_http "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/http"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

// Settings are the settings of the HTTP destination: the URL (required), the Method, the TimeoutMS, Compress,
// SkipTLSVerification, the authentication (BearerToken, or APIKeyHeader and APIKeyValue), the retry policy
// (MaxAttempts, RetryBaseDelayMS, RetryMaxDelayMS) and the MinimumSeverity (TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL,
// in upper case).
type Settings = internal_http.Settings

// Destination is the HTTP destination. It satisfies the log sink port of the core; its Deliver method also gives the
// outcome of the request and its NextDelay method the retry policy, for a worker that retries.
type Destination = internal_http.HTTPDestination

// Delivery is what Destination.Deliver answers: the result, the outcome of the request, and how many events were sent
// and how many the minimum severity filtered.
type Delivery = internal_http.Delivery

// DefaultSettings returns the settings with their defaults: POST, a timeout of 3000 ms, 5 attempts, a backoff from 200 ms
// to 30000 ms and a minimum severity of INFO. The URL is empty: set it before NewDestination.
func DefaultSettings() Settings { return internal_http.DefaultSettings() }

// NewDestination validates the settings and returns a destination with the default limits. An invalid setting returns a
// *telemetry.ValidationError with every problem (the URL is always required). Pass the result to telemetry.WithLogSink.
func NewDestination(settings Settings) (*Destination, error) {
	return NewDestinationWithLimits(settings, value_objects.DefaultLimits())
}

// NewDestinationWithLimits is NewDestination with the limits of the core: a batch larger than the batch limit is not sent
// and its events are dropped with the reason batch_too_large.
func NewDestinationWithLimits(settings Settings, limits telemetry.Limits) (*Destination, error) {
	return dependencies.NewHTTPDestination(settings, limits)
}
