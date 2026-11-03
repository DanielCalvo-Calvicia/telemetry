// Command aws_http_smoke is a sample service that uses the telemetry library the way a real consumer does and sends its
// logs to the add_log endpoint of aws_microservice (CloudWatch Logs) through the AWS HTTP adapter, a Go module of its
// own. It is the manual end-to-end check of a deployment: run it, then look for its entries in the log group and log
// stream. It also proves that a project outside the core module can import the adapter.
//
// What it does: builds the core with the microservice profile, passes the console destination and the AWS HTTP adapter
// in code, handles two simulated requests (a call context per request, one level deeper per layer) and logs at every
// severity, with a sensitive field to prove the masking. The minimum severity of both destinations is INFO, so the
// DEBUG entry is not sent.
//
// The library reads no environment variable (ADR-004): this sample, which is the consumer, reads these and passes them in.
//
//	TELEMETRY_AWS_HTTP_HOST             scheme and host of aws_microservice  (default http://localhost)
//	TELEMETRY_AWS_HTTP_PORT             port of aws_microservice             (default 8080)
//	TELEMETRY_AWS_HTTP_PATH             path of the add_log endpoint         (default /api/v1/cloudwatch/logs/add)
//	TELEMETRY_AWS_HTTP_LOG_GROUP_NAME   log group that receives the entries  (required)
//	TELEMETRY_AWS_HTTP_LOG_STREAM_NAME  log stream that receives the entries (required)
//
// Run: go run .   (from examples/aws_http_smoke)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/console"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

const application_name = "telemetry-e2e-sample"

func main() {
	if run_error := run(); run_error != nil {
		fmt.Fprintln(os.Stderr, "e2e sample failed:", run_error)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	aws_settings := aws_http.DefaultSettings()
	aws_settings.Host = env("TELEMETRY_AWS_HTTP_HOST", "http://localhost")
	aws_settings.Port = env("TELEMETRY_AWS_HTTP_PORT", "8080")
	aws_settings.Path = env("TELEMETRY_AWS_HTTP_PATH", "/api/v1/cloudwatch/logs/add")
	aws_settings.LogGroupName = os.Getenv("TELEMETRY_AWS_HTTP_LOG_GROUP_NAME")
	aws_settings.LogStreamName = os.Getenv("TELEMETRY_AWS_HTTP_LOG_STREAM_NAME")
	aws_destination, aws_error := aws_http.NewDestination(aws_settings)
	if aws_error != nil {
		return fmt.Errorf("building the AWS HTTP adapter: %w", aws_error)
	}
	console_destination, console_error := console.NewDestination(console.DefaultSettings())
	if console_error != nil {
		return fmt.Errorf("building the console destination: %w", console_error)
	}

	config := telemetry.Microservice()
	config.Resource.ServiceName = application_name
	config.Resource.InstanceID = "instance-1"
	core, core_error := telemetry.New(config, telemetry.WithLogSink(console_destination), telemetry.WithLogSink(aws_destination))
	if core_error != nil {
		return fmt.Errorf("building the core: %w", core_error)
	}
	defer func() { _ = core.Shutdown(ctx) }()
	logger := core.Logger()

	logger.Info(ctx, telemetry.LogEntry{Category: "main", Function: "run", Message: "the sample service starts"})
	for _, order_id := range []string{"order-1", "order-2"} {
		if request_error := handle_request(ctx, logger, order_id); request_error != nil {
			return request_error
		}
	}
	logger.Info(ctx, telemetry.LogEntry{Category: "main", Function: "run", Message: "the sample service stops"})
	return nil
}

// handle_request is the inbound layer: it starts the call context of the request and calls the service layer.
func handle_request(ctx context.Context, logger telemetry.Logger, order_id string) error {
	request_call_context, context_error := telemetry.NewCallContext(
		telemetry.WithApplication(application_name), telemetry.WithInstanceID("instance-1"), telemetry.WithCorrelationID(order_id))
	if context_error != nil {
		return fmt.Errorf("building the call context: %w", context_error)
	}
	request_ctx := telemetry.WithCallContext(ctx, request_call_context)

	logger.Info(request_ctx, telemetry.LogEntry{Category: "http", Function: "handle_request", Message: "request received", Fields: map[string]any{"order_id": order_id}})
	place_order(request_ctx, logger, order_id)
	return nil
}

// place_order is the service layer: one level deeper than the inbound layer.
func place_order(ctx context.Context, logger telemetry.Logger, order_id string) {
	service_ctx, _ := telemetry.NextIndentation(ctx)

	// DEBUG is below the minimum severity of both destinations: it is not printed and not sent.
	logger.Debug(service_ctx, telemetry.LogEntry{Category: "service", Function: "place_order", Message: "checking the stock", Fields: map[string]any{"order_id": order_id}})
	logger.Info(service_ctx, telemetry.LogEntry{Category: "service", Function: "place_order", Message: "order placed", Fields: map[string]any{"order_id": order_id, "password": "must-be-masked"}})
	logger.Warn(service_ctx, telemetry.LogEntry{Category: "service", Function: "place_order", Message: "the payment provider answered slowly", Fields: map[string]any{"latency_ms": 2400}})
	logger.Error(service_ctx, telemetry.LogEntry{Category: "service", Function: "place_order", Message: "the confirmation email could not be sent", Fields: map[string]any{"order_id": order_id}})
}

func env(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
