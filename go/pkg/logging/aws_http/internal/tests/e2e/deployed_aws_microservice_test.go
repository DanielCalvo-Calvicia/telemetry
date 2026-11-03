package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http"
)

// =============================================================================
// INPUTS OF THE TEST: set here what you want to send and where
// =============================================================================

const (
	// run_against_deployed_service must be true for the test to call the deployed aws_microservice. While it is
	// false the test is skipped, so a normal "go test ./..." never calls a real service. It is false in everything
	// that is committed (CLAUDE.md): keep your own values in a file that git ignores, for example a copy of this
	// file named deployed_aws_microservice_test.go.local.
	run_against_deployed_service = false

	// Where the deployed aws_microservice listens (the add_log endpoint is host:port/path).
	deployed_host = "http://localhost" // scheme and host
	deployed_port = "8080"
	deployed_path = "/api/v1/cloudwatch/logs/add"

	// Where CloudWatch Logs stores the line; the endpoint creates them when they do not exist.
	deployed_log_group  = "my-log-group"
	deployed_log_stream = "my-log-stream"

	// The line that is sent. The test appends the UTC time to the message so that each run is easy to find.
	line_application = "telemetry-e2e"
	line_category    = "e2e"
	line_function    = "Test_one_log_line_reaches_the_deployed_aws_microservice"
	line_message     = "telemetry e2e"
	line_field_key   = "source"
	line_field_value = "telemetry aws_http adapter"

	// How long the call to the deployed service may take.
	send_timeout = 15 * time.Second
)

// Test_one_log_line_reaches_the_deployed_aws_microservice sends ONE log event, through the AWS HTTP adapter, to the
// aws_microservice that is already deployed, and fails when the endpoint does not accept it (a 2xx answer means
// CloudWatch Logs accepted the entry). The test prints the unique text of the entry: search for it in the log stream.
//
// Set the inputs in the constants above, set run_against_deployed_service to true and run, from the module folder:
//
//	go test ./internal/tests/e2e -run Test_one_log_line_reaches_the_deployed_aws_microservice -v -count=1
func Test_one_log_line_reaches_the_deployed_aws_microservice(t *testing.T) {
	if !run_against_deployed_service {
		t.Skip("set run_against_deployed_service to true in deployed_aws_microservice_test.go to send a line to the deployed aws_microservice")
	}

	settings := aws_http.DefaultSettings()
	settings.Host = deployed_host
	settings.Port = deployed_port
	settings.Path = deployed_path
	settings.LogGroupName = deployed_log_group
	settings.LogStreamName = deployed_log_stream
	destination, build_error := aws_http.NewDestination(settings)
	if build_error != nil {
		t.Fatalf("building the adapter: %v", build_error)
	}

	now := time.Now().UTC()
	unique_text := fmt.Sprintf("%s %s", line_message, now.Format("2006-01-02T15:04:05.000Z"))
	occurred_at := now.Format("2006-01-02T15:04:05.000000Z")
	event := value_objects.Envelope{
		SchemaVersion: "0.1",
		EventId:       "01900000-0000-7000-8000-000000000001",
		EventType:     "log.entry.recorded",
		OccurredAt:    occurred_at,
		RecordedAt:    occurred_at,
		Severity:      "INFO",
		Resource:      value_objects.Resource{ServiceName: line_application, InstanceId: "e2e"},
		Data:          map[string]any{"category": line_category, "function": line_function, "message": unique_text},
		Attributes:    value_objects.Attributes{line_field_key: line_field_value},
		Context:       &value_objects.Context{CorrelationId: "e2e-" + now.Format("150405")},
	}

	ctx, cancel := context.WithTimeout(context.Background(), send_timeout)
	defer cancel()
	delivery := destination.Deliver(ctx, []value_objects.Envelope{event})
	if delivery.Result.Accepted != 1 || delivery.Skipped != 0 {
		t.Fatalf("the deployed aws_microservice did not accept the log line: result %+v, outcome %+v", delivery.Result, delivery.Outcome)
	}

	t.Logf("accepted by %s:%s -> log group %q, log stream %q", deployed_host, deployed_port, deployed_log_group, deployed_log_stream)
	t.Logf("search this text in CloudWatch Logs: %q", unique_text)
}
