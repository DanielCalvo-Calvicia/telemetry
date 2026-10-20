package e2e_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	aws_http "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/aws_http"
)

// Test_one_log_line_reaches_the_deployed_aws_microservice sends ONE log entry, through the aws_http outbound adapter,
// to the aws_microservice that is already deployed, and fails when the endpoint does not accept it (a 2xx answer means
// CloudWatch Logs accepted the entry). It is skipped unless the target is given, so the normal test run never calls
// a real service.
//
// Settings:
//
//	E2E_AWS_HOST          scheme and host of the deployed aws_microservice, e.g. http://10.0.0.5  (required)
//	E2E_AWS_PORT          its port, e.g. 30003 (NodePort of development)                          (required)
//	E2E_AWS_LOG_GROUP     log group that receives the entry                                       (required)
//	E2E_AWS_LOG_STREAM    log stream that receives the entry                                      (required)
//	E2E_AWS_PATH          path of the add_log endpoint                  (default /api/v1/cloudwatch/logs/add)
//
// Run: go test ./internal/tests/e2e -run Test_one_log_line_reaches_the_deployed_aws_microservice -v -count=1
//
// The test prints the unique text of the entry: search for it in the log stream of CloudWatch Logs.
func Test_one_log_line_reaches_the_deployed_aws_microservice(t *testing.T) {
	host, port := os.Getenv("E2E_AWS_HOST"), os.Getenv("E2E_AWS_PORT")
	log_group, log_stream := os.Getenv("E2E_AWS_LOG_GROUP"), os.Getenv("E2E_AWS_LOG_STREAM")
	if host == "" || port == "" || log_group == "" || log_stream == "" {
		t.Skip("set E2E_AWS_HOST, E2E_AWS_PORT, E2E_AWS_LOG_GROUP and E2E_AWS_LOG_STREAM to send a line to the deployed aws_microservice")
	}
	path := os.Getenv("E2E_AWS_PATH")
	if path == "" {
		path = "/api/v1/cloudwatch/logs/add"
	}

	aws_http_logger, err := aws_http.NewAWSHTTPLogger(host, port, path, log_group, log_stream)
	if err != nil {
		t.Fatalf("building the adapter: %v", err)
	}

	unique_text := fmt.Sprintf("telemetry e2e %s", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"))
	entry := dtos.LogEntryOutboundDTO{
		ApplicationID: "e2e",
		ProcessID:     "e2e-" + time.Now().UTC().Format("150405"),
		Application:   "telemetry-e2e",
		Category:      "e2e",
		Function:      "Test_one_log_line_reaches_the_deployed_aws_microservice",
		Indentation:   1,
		Level:         "INFO",
		Message:       unique_text,
		Fields:        map[string]any{"source": "telemetry aws_http adapter"},
		Timestamp:     time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := aws_http_logger.Send(ctx, entry); err != nil {
		t.Fatalf("the deployed aws_microservice did not accept the log line: %v", err)
	}

	t.Logf("accepted by %s:%s -> log group %q, log stream %q", host, port, log_group, log_stream)
	t.Logf("search this text in CloudWatch Logs: %q", unique_text)
}
