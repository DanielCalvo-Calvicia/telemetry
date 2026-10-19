package console_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	console "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/console"
)

var reference_entry = dtos.LogEntryOutboundDTO{
	ProducerID:    "producer",
	ApplicationID: "application-id",
	ProcessID:     "process-id",
	Application:   "app",
	Category:      "service",
	Function:      "Put",
	Indentation:   2,
	Depth:         3,
	Level:         "INFO",
	Message:       "stored",
	Fields:        map[string]any{"bucket": "b"},
	Timestamp:     time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC),
}

type failing_writer struct {
	error_to_return error
}

func (writer failing_writer) Write([]byte) (int, error) {
	return 0, writer.error_to_return
}

func Test_send_writes_one_json_line_with_the_flat_payload(t *testing.T) {
	var output bytes.Buffer
	console_logger := console.NewConsoleLoggerWithWriter(&output)

	if err := console_logger.Send(context.Background(), reference_entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	written_text := output.String()
	if !strings.HasSuffix(written_text, "\n") || strings.Count(written_text, "\n") != 1 {
		t.Fatalf("output must be exactly one line, got %q", written_text)
	}

	var decoded_entry dtos.LogEntryOutboundDTO
	if err := json.Unmarshal([]byte(written_text), &decoded_entry); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if decoded_entry.Message != "stored" || decoded_entry.Level != "INFO" || decoded_entry.Depth != 3 ||
		decoded_entry.Fields["bucket"] != "b" || !decoded_entry.Timestamp.Equal(reference_entry.Timestamp) {
		t.Errorf("decoded entry = %+v", decoded_entry)
	}
	for _, expected_key := range []string{`"ProducerID"`, `"ApplicationID"`, `"ProcessID"`, `"Application"`, `"Category"`, `"Function"`, `"Indentation"`, `"Depth"`, `"Level"`, `"Message"`, `"Fields"`, `"Timestamp"`} {
		if !strings.Contains(written_text, expected_key) {
			t.Errorf("payload is missing %s: %s", expected_key, written_text)
		}
	}
}

func Test_every_send_is_its_own_line(t *testing.T) {
	var output bytes.Buffer
	console_logger := console.NewConsoleLoggerWithWriter(&output)

	for range 3 {
		if err := console_logger.Send(context.Background(), reference_entry); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if strings.Count(output.String(), "\n") != 3 {
		t.Errorf("want 3 lines, got %q", output.String())
	}
}

func Test_a_write_failure_is_translated_and_keeps_its_cause(t *testing.T) {
	disk_failure := errors.New("output closed")
	console_logger := console.NewConsoleLoggerWithWriter(failing_writer{error_to_return: disk_failure})

	err := console_logger.Send(context.Background(), reference_entry)

	if !errors.Is(err, application.ErrLogSinkDeliveryFailed) {
		t.Errorf("error = %v, want ErrLogSinkDeliveryFailed", err)
	}
	if !errors.Is(err, disk_failure) {
		t.Errorf("error = %v, want the root cause kept", err)
	}
}

func Test_an_entry_that_cannot_be_encoded_is_translated(t *testing.T) {
	var output bytes.Buffer
	console_logger := console.NewConsoleLoggerWithWriter(&output)
	unencodable_entry := reference_entry
	unencodable_entry.Fields = map[string]any{"callback": func() {}}

	err := console_logger.Send(context.Background(), unencodable_entry)

	if !errors.Is(err, application.ErrLogSinkDeliveryFailed) {
		t.Errorf("error = %v, want ErrLogSinkDeliveryFailed", err)
	}
	if output.Len() != 0 {
		t.Errorf("nothing must be written, got %q", output.String())
	}
}

func Test_new_console_logger_writes_to_the_standard_output(t *testing.T) {
	read_end, write_end, err := os.Pipe()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	original_stdout := os.Stdout
	os.Stdout = write_end
	defer func() { os.Stdout = original_stdout }()

	console_logger := console.NewConsoleLogger()
	send_error := console_logger.Send(context.Background(), reference_entry)
	_ = write_end.Close()
	os.Stdout = original_stdout

	if send_error != nil {
		t.Fatalf("unexpected error: %v", send_error)
	}
	written_bytes, _ := io.ReadAll(read_end)
	if !strings.Contains(string(written_bytes), `"Message":"stored"`) {
		t.Errorf("standard output = %q", written_bytes)
	}
}
