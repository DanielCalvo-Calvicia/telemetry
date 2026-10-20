// Command aws_http_smoke sends one log entry through the telemetry library to the add_log endpoint of
// aws_microservice, so the entry lands in a CloudWatch Logs stream. It is a manual check of a deployment.
//
// Settings (environment variables of this script, not of the library):
//
//	SMOKE_HOST        scheme and host of aws_microservice, e.g. http://10.0.0.5   (default http://localhost)
//	SMOKE_PORT        port of aws_microservice, e.g. 30003 (NodePort of development)  (default 8080)
//	SMOKE_LOG_GROUP   log group that receives the entry                            (required)
//	SMOKE_LOG_STREAM  log stream that receives the entry                           (required)
//	SMOKE_APP_ENV     development or production                                    (default development)
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

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "smoke test failed:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := logging.Config{
		ConsoleEnabled:   true,
		AWSHTTPEnabled:   true,
		AWSHTTPHost:      env("SMOKE_HOST", "http://localhost"),
		AWSHTTPPort:      env("SMOKE_PORT", "8080"),
		AWSHTTPPath:      env("SMOKE_PATH", "/api/v1/cloudwatch/logs/add"),
		AWSHTTPLogGroup:  os.Getenv("SMOKE_LOG_GROUP"),
		AWSHTTPLogStream: os.Getenv("SMOKE_LOG_STREAM"),
	}

	logger, shutdown, err := logging.New(ctx, env("SMOKE_APP_ENV", "development"), cfg)
	if err != nil {
		return fmt.Errorf("building the logger: %w", err)
	}
	defer func() { _ = shutdown(ctx) }()

	request_ctx, err := logging.NewRequestContext(ctx, "telemetry-smoke", "smoke-1")
	if err != nil {
		return fmt.Errorf("building the call context: %w", err)
	}

	// A failed delivery is returned by the logger, so the script reports it instead of hiding it.
	return logger.Info(request_ctx, logging.Entry{
		Category:    "smoke",
		Function:    "run",
		Indentation: logging.StartingIndentation,
		Message:     "telemetry smoke test",
		Fields:      map[string]any{"sent_at": time.Now().UTC().Format(time.RFC3339)},
	})
}

func env(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
