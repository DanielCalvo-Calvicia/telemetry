package e2e_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

// logging_fixtures_folder is the folder of the logging cases that the Python tests read too (the same files).
const logging_fixtures_folder = "../../../../integration-tests/fixtures/logging"

type logging_call struct {
	Method   string         `json:"method"`
	Category string         `json:"category"`
	Function string         `json:"function"`
	Message  string         `json:"message"`
	Fields   map[string]any `json:"fields"`
}

type logging_record struct {
	GoLevel     int            `json:"go_level"`
	PythonLevel int            `json:"python_level"`
	Message     string         `json:"message"`
	Attrs       map[string]any `json:"attrs"`
}

type logging_level_row struct {
	Description string `json:"description"`
	GoLevel     int    `json:"go_level"`
	PythonLevel int    `json:"python_level"`
	Severity    string `json:"severity"`
}

type logging_expected struct {
	Deliveries map[string][][]string `json:"deliveries"`
}

type logging_fixture struct {
	Description string `json:"description"`
	Kind        string `json:"kind"`
	case_setup
	Calls           []logging_call      `json:"calls"`
	DefaultCategory string              `json:"default_category"`
	Records         []logging_record    `json:"records"`
	Cases           []logging_level_row `json:"cases"`
	Expected        *logging_expected   `json:"expected"`
}

// recording_logger is a telemetry.Logger that remembers the method that was called, to test the mapping of the levels.
type recording_logger struct {
	methods []string
}

func (logger *recording_logger) Trace(_ context.Context, _ telemetry.LogEntry) {
	logger.methods = append(logger.methods, "TRACE")
}
func (logger *recording_logger) Debug(_ context.Context, _ telemetry.LogEntry) {
	logger.methods = append(logger.methods, "DEBUG")
}
func (logger *recording_logger) Info(_ context.Context, _ telemetry.LogEntry) {
	logger.methods = append(logger.methods, "INFO")
}
func (logger *recording_logger) Warn(_ context.Context, _ telemetry.LogEntry) {
	logger.methods = append(logger.methods, "WARN")
}
func (logger *recording_logger) Error(_ context.Context, _ telemetry.LogEntry) {
	logger.methods = append(logger.methods, "ERROR")
}
func (logger *recording_logger) Critical(_ context.Context, _ telemetry.LogEntry) {
	logger.methods = append(logger.methods, "CRITICAL")
}

// call_logger makes the call of the case through the method it names.
func call_logger(t *testing.T, ctx context.Context, logger telemetry.Logger, call logging_call) {
	t.Helper()
	entry := telemetry.LogEntry{Category: call.Category, Function: call.Function, Message: call.Message, Fields: call.Fields}
	switch call.Method {
	case "trace":
		logger.Trace(ctx, entry)
	case "debug":
		logger.Debug(ctx, entry)
	case "info":
		logger.Info(ctx, entry)
	case "warn":
		logger.Warn(ctx, entry)
	case "error":
		logger.Error(ctx, entry)
	case "critical":
		logger.Critical(ctx, entry)
	default:
		t.Fatalf("the method %q is not known", call.Method)
	}
}

// slog_attr_of makes the slog attribute of a JSON value: a scalar keeps its kind and a map or a list is Any.
func slog_attr_of(key string, value any) slog.Attr {
	switch typed_value := value.(type) {
	case string:
		return slog.String(key, typed_value)
	case bool:
		return slog.Bool(key, typed_value)
	case float64:
		return slog.Float64(key, typed_value)
	default:
		return slog.Any(key, typed_value)
	}
}

func slog_record_of(record logging_record) slog.Record {
	slog_record := slog.NewRecord(time.Now(), slog.Level(record.GoLevel), record.Message, 0)
	for _, key := range slices.Sorted(func(yield func(string) bool) {
		for key := range record.Attrs {
			if !yield(key) {
				return
			}
		}
	}) {
		slog_record.AddAttrs(slog_attr_of(key, record.Attrs[key]))
	}
	return slog_record
}

func check_deliveries(t *testing.T, description string, expected *logging_expected, readers map[string][]delivery_reader) {
	t.Helper()
	if expected == nil {
		t.Fatalf("%s: the case has no expected", description)
	}
	settle_deliveries(t, description, expected.Deliveries, readers)
	compare_deliveries(t, description, expected.Deliveries, readers)
}

func Test_every_shared_logging_case_gives_the_same_result(t *testing.T) {
	if _, stat_error := os.Stat(logging_fixtures_folder); stat_error != nil {
		t.Fatalf("the folder of the logging cases is missing: %v", stat_error)
	}
	file_paths, glob_error := filepath.Glob(filepath.Join(logging_fixtures_folder, "*.json"))
	if glob_error != nil {
		t.Fatal(glob_error)
	}
	if len(file_paths) == 0 {
		t.Fatalf("no case in %s", logging_fixtures_folder)
	}

	for _, file_path := range file_paths {
		t.Run(filepath.Base(file_path), func(t *testing.T) {
			content, read_error := os.ReadFile(file_path)
			if read_error != nil {
				t.Fatal(read_error)
			}
			var fixture logging_fixture
			if decode_error := json.Unmarshal(content, &fixture); decode_error != nil {
				t.Fatal(decode_error)
			}

			switch fixture.Kind {
			case "levels":
				if len(fixture.Cases) == 0 {
					t.Fatalf("%s: no rows", fixture.Description)
				}
				for _, row := range fixture.Cases {
					recorder := &recording_logger{}
					handler := telemetry.NewSlogHandler(recorder, "app")
					record := slog.NewRecord(time.Now(), slog.Level(row.GoLevel), "m", 0)
					if handle_error := handler.Handle(context.Background(), record); handle_error != nil {
						t.Fatalf("%s: %v", row.Description, handle_error)
					}
					if !reflect.DeepEqual(recorder.methods, []string{row.Severity}) {
						t.Errorf("%s: go level %d gave %v, want %s", row.Description, row.GoLevel, recorder.methods, row.Severity)
					}
				}
			case "log", "handler":
				core, readers, new_error := build_case_core(t, fixture.case_setup)
				if new_error != nil {
					t.Fatalf("%s: New failed: %v", fixture.Description, new_error)
				}
				ctx := case_context(t, fixture.case_setup)
				if fixture.Kind == "log" {
					if len(fixture.Calls) == 0 {
						t.Fatalf("%s: no calls", fixture.Description)
					}
					for _, call := range fixture.Calls {
						call_logger(t, ctx, core.Logger(), call)
					}
				} else {
					if len(fixture.Records) == 0 {
						t.Fatalf("%s: no records", fixture.Description)
					}
					handler := telemetry.NewSlogHandler(core.Logger(), fixture.DefaultCategory)
					for _, record := range fixture.Records {
						if handle_error := handler.Handle(ctx, slog_record_of(record)); handle_error != nil {
							t.Fatalf("%s: %v", fixture.Description, handle_error)
						}
					}
				}
				check_deliveries(t, fixture.Description, fixture.Expected, readers)
			default:
				t.Fatalf("%s: the kind %q is not known (log, handler, levels)", fixture.Description, fixture.Kind)
			}
		})
	}
}
