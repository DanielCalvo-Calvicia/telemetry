package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes/fake_receiver"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/console"
	logging_http "github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/http"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

const (
	http_package_folder  = "../../../pkg/logging/http"
	http_fixtures_folder = "../../../../integration-tests/fixtures/http_destination"
)

// http_receiver is a local receiver on 127.0.0.1 that records the bodies it gets and answers a status.
type http_receiver struct {
	server *httptest.Server
	mutex  sync.Mutex
	bodies []string
}

func new_http_receiver(t *testing.T, status int) *http_receiver {
	t.Helper()
	receiver := &http_receiver{}
	receiver.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		receiver.mutex.Lock()
		receiver.bodies = append(receiver.bodies, string(body))
		receiver.mutex.Unlock()
		writer.WriteHeader(status)
	}))
	t.Cleanup(receiver.server.Close)
	return receiver
}

func (receiver *http_receiver) received() []string {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()
	return append([]string(nil), receiver.bodies...)
}

func new_core_with(t *testing.T, sinks ...telemetry.LogSinkPort) *telemetry.Telemetry {
	t.Helper()
	config := telemetry.Microservice()
	config.Resource.ServiceName = "sensor-hub"
	options := make([]telemetry.Option, 0, len(sinks))
	for _, sink := range sinks {
		options = append(options, telemetry.WithLogSink(sink))
	}
	core, new_error := telemetry.New(config, options...)
	if new_error != nil {
		t.Fatal(new_error)
	}
	return core
}

func log_input() telemetry.NewEventInput {
	return telemetry.NewEventInput{
		EventType: "log.entry.recorded",
		Severity:  telemetry.SeverityInfo,
		Data:      map[string]any{"category": "http", "message": "hello"},
	}
}

func Test_two_destinations_one_answers_503_and_the_other_still_gets_the_line(t *testing.T) {
	receiver := fake_receiver.New(fake_receiver.Config{})
	t.Cleanup(receiver.Close)
	receiver.Script(fake_receiver.Answer{Status: http.StatusServiceUnavailable}, fake_receiver.Answer{Status: http.StatusServiceUnavailable})
	http_sink, http_error := logging_http.NewDestination(func() logging_http.Settings {
		settings := logging_http.DefaultSettings()
		settings.URL = receiver.URL()
		return settings
	}())
	if http_error != nil {
		t.Fatal(http_error)
	}
	var buffer bytes.Buffer
	console_sink, console_error := console.NewDestinationWithWriter(console.DefaultSettings(), &buffer)
	if console_error != nil {
		t.Fatal(console_error)
	}
	core := new_core_with(t, http_sink, console_sink)

	core.Logger().Info(context.Background(), telemetry.LogEntry{Category: "http", Message: "through the logger"})
	result := core.Emit(context.Background(), log_input())

	if strings.Count(buffer.String(), "\n") != 2 || !strings.Contains(buffer.String(), `"message":"hello"`) {
		t.Fatalf("the console must hold both lines, it holds %q", buffer.String())
	}
	if result != (telemetry.EmitResult{Accepted: 1, Failed: 1, Reason: "status_503"}) {
		t.Fatalf("the result is %+v", result)
	}
	requests := receiver.Requests()
	if len(requests) != 2 || !requests[1].Valid || len(requests[1].Events) != 1 || requests[1].AnsweredStatus != http.StatusServiceUnavailable ||
		!strings.Contains(string(requests[1].Events[0]), `"message":"hello"`) {
		t.Fatalf("the receiver got %+v", requests)
	}
}

func Test_two_destinations_one_never_answers_and_the_other_still_gets_the_line(t *testing.T) {
	closed_server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closed_url := closed_server.URL
	closed_server.Close()
	settings := logging_http.DefaultSettings()
	settings.URL = closed_url
	http_sink, http_error := logging_http.NewDestination(settings)
	if http_error != nil {
		t.Fatal(http_error)
	}
	var buffer bytes.Buffer
	console_sink, console_error := console.NewDestinationWithWriter(console.DefaultSettings(), &buffer)
	if console_error != nil {
		t.Fatal(console_error)
	}
	core := new_core_with(t, console_sink, http_sink)

	result := core.Emit(context.Background(), log_input())

	if result != (telemetry.EmitResult{Accepted: 1, Failed: 1, Reason: "network_error"}) {
		t.Fatalf("the result is %+v", result)
	}
	if strings.Count(buffer.String(), "\n") != 1 {
		t.Fatalf("the console holds %q", buffer.String())
	}
}

func Test_the_http_destination_sends_the_batch_body_of_the_core_event(t *testing.T) {
	receiver := new_http_receiver(t, http.StatusOK)
	settings := logging_http.DefaultSettings()
	settings.URL = receiver.server.URL
	http_sink, http_error := logging_http.NewDestinationWithLimits(settings, telemetry.DefaultLimits())
	if http_error != nil {
		t.Fatal(http_error)
	}
	core := new_core_with(t, http_sink)

	result := core.Emit(context.Background(), log_input())

	if result != (telemetry.EmitResult{Accepted: 1}) {
		t.Fatalf("%+v", result)
	}
	var body struct {
		SchemaVersion string `json:"schema_version"`
		Events        []struct {
			EventType string `json:"event_type"`
			Severity  string `json:"severity"`
		} `json:"events"`
	}
	bodies := receiver.received()
	if len(bodies) != 1 {
		t.Fatalf("got %d bodies", len(bodies))
	}
	if decode_error := json.Unmarshal([]byte(bodies[0]), &body); decode_error != nil {
		t.Fatal(decode_error)
	}
	if body.SchemaVersion != "0.1" || len(body.Events) != 1 || body.Events[0].EventType != "log.entry.recorded" || body.Events[0].Severity != "INFO" {
		t.Fatalf("the body is %s", bodies[0])
	}
}

func Test_the_public_http_constructors_refuse_invalid_settings(t *testing.T) {
	_, no_url_error := logging_http.NewDestination(logging_http.DefaultSettings())
	_, limits_error := logging_http.NewDestinationWithLimits(logging_http.Settings{}, telemetry.DefaultLimits())
	for _, got_error := range []error{no_url_error, limits_error} {
		validation_error, is_validation_error := got_error.(*telemetry.ValidationError)
		if !is_validation_error || len(validation_error.Problems) == 0 || validation_error.Problems[0].Field != "url" {
			t.Errorf("expected a *telemetry.ValidationError that names the url, got %T %v", got_error, got_error)
		}
	}
}

func parse_http_package(t *testing.T) map[string]exported_identifier {
	t.Helper()
	file_set := token.NewFileSet()
	packages, parse_error := parser.ParseDir(file_set, http_package_folder, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if parse_error != nil {
		t.Fatal(parse_error)
	}
	package_found, has_package := packages["http"]
	if !has_package {
		t.Fatalf("no package http in %s", http_package_folder)
	}
	identifiers := map[string]exported_identifier{}
	for _, file := range package_found.Files {
		for _, declaration := range file.Decls {
			switch typed_declaration := declaration.(type) {
			case *ast.FuncDecl:
				if typed_declaration.Recv == nil && typed_declaration.Name.IsExported() {
					identifiers[typed_declaration.Name.Name] = exported_identifier{typed_declaration.Name.Name, "func", typed_declaration.Doc.Text()}
				}
			case *ast.GenDecl:
				for _, spec := range typed_declaration.Specs {
					if typed_spec, is_type := spec.(*ast.TypeSpec); is_type && typed_spec.Name.IsExported() {
						doc := typed_spec.Doc
						if doc == nil && len(typed_declaration.Specs) == 1 {
							doc = typed_declaration.Doc
						}
						identifiers[typed_spec.Name.Name] = exported_identifier{typed_spec.Name.Name, "type", doc.Text()}
					}
				}
			}
		}
	}
	return identifiers
}

func Test_the_public_http_package_exports_every_name_of_the_shared_list(t *testing.T) {
	content, read_error := os.ReadFile(filepath.Join(http_fixtures_folder, "public_names.json"))
	if read_error != nil {
		t.Fatal(read_error)
	}
	var names public_names_file
	if decode_error := json.Unmarshal(content, &names); decode_error != nil {
		t.Fatal(decode_error)
	}
	if len(names.Functions) == 0 || len(names.Types) == 0 {
		t.Fatal("public_names.json lists no function or no type")
	}
	exported := parse_http_package(t)
	for _, function := range names.Functions {
		if exported[function.Go].kind != "func" {
			t.Errorf("the function %s is not exported by the public package", function.Go)
		}
	}
	for _, type_name := range names.Types {
		if exported[type_name.Go].kind != "type" {
			t.Errorf("the type %s is not exported by the public package", type_name.Go)
		}
	}
}

func Test_every_exported_identifier_of_the_http_package_has_a_doc_comment_that_starts_with_its_name(t *testing.T) {
	exported := parse_http_package(t)
	if len(exported) == 0 {
		t.Fatal("the public package exports nothing")
	}
	for _, identifier := range exported {
		if !strings.HasPrefix(identifier.doc, identifier.name+" ") {
			t.Errorf("the %s %s has no doc comment that starts with its name (got %q)", identifier.kind, identifier.name, identifier.doc)
		}
	}
}

func Test_the_public_limits_can_be_built_and_refused(t *testing.T) {
	limits, limits_error := telemetry.NewLimits(8, 32, 128, 1024, 4096)
	if limits_error != nil || limits.MaxBatchBytes() != 4096 {
		t.Fatalf("%+v %v", limits, limits_error)
	}
	if _, refused_error := telemetry.NewLimits(8, 32, 128, 1024, 512); refused_error == nil {
		t.Fatal("a batch limit below the event limit must be refused")
	}
	settings := logging_http.DefaultSettings()
	settings.URL = "http://127.0.0.1:9"
	if _, build_error := logging_http.NewDestinationWithLimits(settings, limits); build_error != nil {
		t.Fatal(build_error)
	}
}
