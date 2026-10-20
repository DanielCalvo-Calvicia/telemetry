package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	aws_http "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/aws_http"
)

// =============================================================================
// INPUTS OF THE TEST: set here what you want to send and where
// =============================================================================

const (
	// run_against_deployed_service must be true for the test to call the deployed aws_microservice. While it is
	// false the test is skipped, so a normal "go test ./..." never calls a real service.
	run_against_deployed_service = false

	// Where the deployed aws_microservice listens (the add_log endpoint is host:port/path).
	deployed_host = "http://localhost" // scheme and host, e.g. "http://10.0.0.5"
	deployed_port = "30003"            // e.g. the NodePort of development
	deployed_path = "/api/v1/cloudwatch/logs/add"

	// Where CloudWatch Logs stores the line; the endpoint creates them when they do not exist.
	deployed_log_group  = "snowfactory/telemetry-e2e"
	deployed_log_stream = "telemetry"

	// The line that is sent. The test appends the UTC time to the message so that each run is easy to find.
	line_level       = "INFO"
	line_application = "telemetry-e2e"
	line_category    = "e2e"
	line_function    = "Test_one_log_line_reaches_the_deployed_aws_microservice"
	line_message     = "telemetry e2e"
	line_field_key   = "source"
	line_field_value = "telemetry aws_http adapter"

	// How long the call to the deployed service may take.
	send_timeout = 15 * time.Second
)

// Test_one_log_line_reaches_the_deployed_aws_microservice sends ONE log entry, through the aws_http outbound adapter,
// to the aws_microservice that is already deployed, and fails when the endpoint does not accept it (a 2xx answer means
// CloudWatch Logs accepted the entry). The test prints the unique text of the entry: search for it in the log stream.
//
// Set the inputs in the constants above, set run_against_deployed_service to true and run:
//
//	go test ./internal/tests/e2e -run Test_one_log_line_reaches_the_deployed_aws_microservice -v -count=1
func Test_one_log_line_reaches_the_deployed_aws_microservice(t *testing.T) {
	if !run_against_deployed_service {
		t.Skip("set run_against_deployed_service to true in deployed_aws_microservice_test.go to send a line to the deployed aws_microservice")
	}

	aws_http_logger, err := aws_http.NewAWSHTTPLogger(deployed_host, deployed_port, deployed_path, deployed_log_group, deployed_log_stream)
	if err != nil {
		t.Fatalf("building the adapter: %v", err)
	}

	now := time.Now().UTC()
	unique_text := fmt.Sprintf("%s %s", line_message, now.Format("2006-01-02T15:04:05.000Z"))
	entry := dtos.LogEntryOutboundDTO{
		ApplicationID: "e2e",
		ProcessID:     "e2e-" + now.Format("150405"),
		Application:   line_application,
		Category:      line_category,
		Function:      line_function,
		Indentation:   1,
		Level:         line_level,
		Message:       unique_text,
		Fields:        map[string]any{line_field_key: line_field_value},
		Timestamp:     now,
	}

	ctx, cancel := context.WithTimeout(context.Background(), send_timeout)
	defer cancel()
	if err := aws_http_logger.Send(ctx, entry); err != nil {
		t.Fatalf("the deployed aws_microservice did not accept the log line: %v", err)
	}

	t.Logf("accepted by %s:%s -> log group %q, log stream %q", deployed_host, deployed_port, deployed_log_group, deployed_log_stream)
	t.Logf("search this text in CloudWatch Logs: %q", unique_text)
}
