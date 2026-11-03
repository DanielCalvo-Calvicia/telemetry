package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/console"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

const (
	console_package_folder  = "../../../pkg/logging/console"
	console_fixtures_folder = "../../../../integration-tests/fixtures/console"
)

func Test_the_console_destination_writes_the_masked_event_of_the_core(t *testing.T) {
	var buffer locked_buffer
	sink, build_error := console.NewDestinationWithWriter(console.DefaultSettings(), &buffer)
	if build_error != nil {
		t.Fatal(build_error)
	}
	config := fast_delivery(telemetry.Microservice())
	config.Resource.ServiceName = "sensor-hub"
	core, new_error := telemetry.New(config, telemetry.WithLogSink(sink))
	if new_error != nil {
		t.Fatal(new_error)
	}

	core.Logger().Info(context.Background(), telemetry.LogEntry{
		Category: "auth", Message: "login", Fields: map[string]any{"password": "placeholder-secret-not-real", "user": "bob"},
	})

	waiting.Eventually(t, waiting.DefaultTimeout, "the console receives the line", func() bool { return strings.Contains(buffer.String(), "\n") })
	line := buffer.String()
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("expected exactly one line, got %q", line)
	}
	if strings.Contains(line, "placeholder-secret-not-real") {
		t.Fatalf("a secret reached the console: %s", line)
	}
	var parsed struct {
		EventType  string         `json:"event_type"`
		Severity   string         `json:"severity"`
		Data       map[string]any `json:"data"`
		Attributes map[string]any `json:"attributes"`
	}
	if decode_error := json.Unmarshal([]byte(line), &parsed); decode_error != nil {
		t.Fatal(decode_error)
	}
	if parsed.EventType != "log.entry.recorded" || parsed.Severity != "INFO" || parsed.Data["message"] != "login" ||
		parsed.Attributes["user"] != "bob" || parsed.Attributes["password"] == "placeholder-secret-not-real" {
		t.Fatalf("the line is %s", line)
	}
}

func Test_the_console_destination_in_text_format_and_with_a_minimum_severity(t *testing.T) {
	var buffer locked_buffer
	sink, build_error := console.NewDestinationWithWriter(console.Settings{Format: "text", MinimumSeverity: "WARN"}, &buffer)
	if build_error != nil {
		t.Fatal(build_error)
	}
	config := fast_delivery(telemetry.Microservice())
	config.Resource.ServiceName = "sensor-hub"
	core, new_error := telemetry.New(config, telemetry.WithLogSink(sink))
	if new_error != nil {
		t.Fatal(new_error)
	}
	core.Logger().Info(context.Background(), telemetry.LogEntry{Category: "c", Message: "quiet"})
	core.Logger().Error(context.Background(), telemetry.LogEntry{Category: "c", Function: "run", Message: "loud"})
	waiting.Eventually(t, waiting.DefaultTimeout, "the console receives the loud line", func() bool { return strings.Contains(buffer.String(), "loud") })
	waiting.StaysTrue(t, 30*time.Millisecond, "the console holds one line", func() bool { return strings.Count(buffer.String(), "\n") == 1 })
	if !strings.Contains(buffer.String(), "ERROR    [c] run: loud\n") || strings.Contains(buffer.String(), "quiet") ||
		strings.Count(buffer.String(), "\n") != 1 {
		t.Fatalf("the output is %q", buffer.String())
	}
}

func Test_the_public_constructors_refuse_invalid_settings(t *testing.T) {
	_, standard_error := console.NewDestination(console.Settings{Format: "xml", MinimumSeverity: "INFO"})
	_, writer_error := console.NewDestinationWithWriter(console.Settings{Format: "json", MinimumSeverity: "nope"}, &bytes.Buffer{})
	for _, got_error := range []error{standard_error, writer_error} {
		if _, is_validation_error := got_error.(*telemetry.ValidationError); !is_validation_error {
			t.Errorf("expected a *telemetry.ValidationError, got %T %v", got_error, got_error)
		}
	}
	if standard_sink, build_error := console.NewDestination(console.DefaultSettings()); build_error != nil || standard_sink == nil {
		t.Errorf("the default settings must build the standard output destination: %v", build_error)
	}
}

func parse_console_package(t *testing.T) map[string]exported_identifier {
	t.Helper()
	file_set := token.NewFileSet()
	packages, parse_error := parser.ParseDir(file_set, console_package_folder, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if parse_error != nil {
		t.Fatal(parse_error)
	}
	package_found, has_package := packages["console"]
	if !has_package {
		t.Fatalf("no package console in %s", console_package_folder)
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
					switch typed_spec := spec.(type) {
					case *ast.TypeSpec:
						doc := typed_spec.Doc
						if doc == nil && len(typed_declaration.Specs) == 1 {
							doc = typed_declaration.Doc
						}
						if typed_spec.Name.IsExported() {
							identifiers[typed_spec.Name.Name] = exported_identifier{typed_spec.Name.Name, "type", doc.Text()}
						}
					case *ast.ValueSpec:
						doc := typed_spec.Doc
						if doc == nil && len(typed_declaration.Specs) == 1 {
							doc = typed_declaration.Doc
						}
						for _, name := range typed_spec.Names {
							if name.IsExported() {
								identifiers[name.Name] = exported_identifier{name.Name, typed_declaration.Tok.String(), doc.Text()}
							}
						}
					}
				}
			}
		}
	}
	return identifiers
}

func Test_the_public_console_package_exports_every_name_of_the_shared_list(t *testing.T) {
	content, read_error := os.ReadFile(filepath.Join(console_fixtures_folder, "public_names.json"))
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
	exported := parse_console_package(t)
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

func Test_every_exported_identifier_of_the_console_package_has_a_doc_comment_that_starts_with_its_name(t *testing.T) {
	exported := parse_console_package(t)
	if len(exported) == 0 {
		t.Fatal("the public package exports nothing")
	}
	for _, identifier := range exported {
		if !strings.HasPrefix(identifier.doc, identifier.name+" ") {
			t.Errorf("the %s %s has no doc comment that starts with its name (got %q)", identifier.kind, identifier.name, identifier.doc)
		}
	}
}
