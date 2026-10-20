// Command aws_http_smoke is a sample service that uses the telemetry library the way a real consumer does, in the
// production environment, and sends its logs to the add_log endpoint of aws_microservice (CloudWatch Logs). It is the
// manual end-to-end check of a deployment: run it, then look for its entries in the log group and log stream.
//
// What it does: loads the logger settings from the environment, builds the logger with APP_ENV, handles two
// simulated requests (a call context per request, one level deeper per layer) and logs at every severity, with a
// sensitive field to prove the masking. In production the minimum level is INFO, so the DEBUG entries are not sent.
//
// Settings (same names as the library's env tags; a real service loads them with its single config pass):
//
//	APP_ENV                      development or production                     (default production)
//	LOGGER_CONSOLE_ENABLED       also print every entry on the console         (default true)
//	LOGGER_AWS_HTTP_HOST         scheme and host of aws_microservice           (default http://localhost)
//	LOGGER_AWS_HTTP_PORT         port of aws_microservice, e.g. 30003          (default 8080)
//	LOGGER_AWS_HTTP_PATH         path of the add_log endpoint                  (default /api/v1/cloudwatch/logs/add)
//	LOGGER_AWS_HTTP_LOG_GROUP    log group that receives the entries           (required)
//	LOGGER_AWS_HTTP_LOG_STREAM   log stream that receives the entries          (required)
//
// Run: go run ./examples/aws_http_smoke
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/pkg/logging"
)

const application_name = "telemetry-e2e-sample"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "e2e sample failed:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := logging.Config{
		ConsoleEnabled:   env("LOGGER_CONSOLE_ENABLED", "true") == "true",
		AWSHTTPEnabled:   true,
		AWSHTTPHost:      env("LOGGER_AWS_HTTP_HOST", "http://localhost"),
		AWSHTTPPort:      env("LOGGER_AWS_HTTP_PORT", "8080"),
		AWSHTTPPath:      env("LOGGER_AWS_HTTP_PATH", "/api/v1/cloudwatch/logs/add"),
		AWSHTTPLogGroup:  os.Getenv("LOGGER_AWS_HTTP_LOG_GROUP"),
		AWSHTTPLogStream: os.Getenv("LOGGER_AWS_HTTP_LOG_STREAM"),
	}
	app_env := env("APP_ENV", "production")

	logger, shutdown, err := logging.New(ctx, app_env, cfg)
	if err != nil {
		return fmt.Errorf("building the logger: %w", err)
	}
	defer func() { _ = shutdown(ctx) }()

	_ = logger.Info(ctx, logging.Entry{Category: "main", Function: "run", Message: "the sample service starts", Fields: map[string]any{"app_env": app_env}})

	for _, order_id := range []string{"order-1", "order-2"} {
		if err := handle_request(ctx, logger, order_id); err != nil {
			return err
		}
	}

	_ = logger.Info(ctx, logging.Entry{Category: "main", Function: "run", Message: "the sample service stops"})
	return nil
}

// handle_request is the inbound layer: it starts the call context of the request and calls the service layer.
func handle_request(ctx context.Context, logger logging.Logger, order_id string) error {
	request_ctx, err := logging.NewRequestContext(ctx, application_name, "instance-1")
	if err != nil {
		return fmt.Errorf("building the call context: %w", err)
	}

	_ = logger.Info(request_ctx, logging.Entry{Category: "http", Function: "handle_request", Indentation: logging.StartingIndentation, Message: "request received", Fields: map[string]any{"order_id": order_id}})
	place_order(request_ctx, logger, order_id)
	return nil
}

// place_order is the service layer: one level deeper than the inbound layer.
func place_order(ctx context.Context, logger logging.Logger, order_id string) {
	service_ctx, _ := logging.NextIndentation(ctx)

	// DEBUG is below the production minimum: it is not printed and not sent.
	_ = logger.Debug(service_ctx, logging.Entry{Category: "service", Function: "place_order", Message: "checking the stock", Fields: map[string]any{"order_id": order_id}})
	_ = logger.Info(service_ctx, logging.Entry{Category: "service", Function: "place_order", Message: "order placed", Fields: map[string]any{"order_id": order_id, "password": "must-be-masked"}})
	_ = logger.Warn(service_ctx, logging.Entry{Category: "service", Function: "place_order", Message: "the payment provider answered slowly", Fields: map[string]any{"latency_ms": 2400}})
	_ = logger.Error(service_ctx, logging.Entry{Category: "service", Function: "place_order", Message: "the confirmation email could not be sent", Fields: map[string]any{"order_id": order_id}})
}

func env(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
