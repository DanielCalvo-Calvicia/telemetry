package e2e_test

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/file"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

const (
	file_package_folder  = "../../../pkg/logging/file"
	file_fixtures_folder = "../../../../integration-tests/fixtures/file"
)

func Test_the_file_destination_writes_the_masked_event_of_the_core(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "logs")
	settings := file.DefaultSettings()
	settings.Directory = directory
	destination, build_error := file.NewDestination(settings)
	if build_error != nil {
		t.Fatal(build_error)
	}
	config := telemetry.Microservice()
	config.Resource.ServiceName = "sensor-hub"
	core, new_error := telemetry.New(config, telemetry.WithLogSink(destination))
	if new_error != nil {
		t.Fatal(new_error)
	}

	core.Logger().Info(context.Background(), telemetry.LogEntry{
		Category: "auth", Message: "login", Fields: map[string]any{"password": "placeholder-secret-not-real", "user": "bob"},
	})

	content, read_error := os.ReadFile(filepath.Join(directory, "telemetry.jsonl"))
	if read_error != nil {
		t.Fatal(read_error)
	}
	line := string(content)
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("expected exactly one line, got %q", line)
	}
	if strings.Contains(line, "placeholder-secret-not-real") {
		t.Fatalf("a secret reached the file: %s", line)
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
	if counters := destination.Counters(); counters != (file.Counters{}) {
		t.Fatalf("one small line must rotate and delete nothing, got %+v", counters)
	}
}

func Test_the_public_constructor_refuses_invalid_settings_and_a_directory_that_cannot_be_written(t *testing.T) {
	_, no_directory_error := file.NewDestination(file.DefaultSettings())
	occupied := filepath.Join(t.TempDir(), "occupied")
	if write_error := os.WriteFile(occupied, []byte("x"), 0o600); write_error != nil {
		t.Fatal(write_error)
	}
	settings := file.DefaultSettings()
	settings.Directory = occupied
	_, occupied_error := file.NewDestination(settings)
	for _, got_error := range []error{no_directory_error, occupied_error} {
		if _, is_validation_error := got_error.(*telemetry.ValidationError); !is_validation_error {
			t.Errorf("expected a *telemetry.ValidationError, got %T %v", got_error, got_error)
		}
	}
}

func parse_file_package(t *testing.T) map[string]exported_identifier {
	t.Helper()
	file_set := token.NewFileSet()
	packages, parse_error := parser.ParseDir(file_set, file_package_folder, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if parse_error != nil {
		t.Fatal(parse_error)
	}
	package_found, has_package := packages["file"]
	if !has_package {
		t.Fatalf("no package file in %s", file_package_folder)
	}
	identifiers := map[string]exported_identifier{}
	for _, source := range package_found.Files {
		for _, declaration := range source.Decls {
			switch typed_declaration := declaration.(type) {
			case *ast.FuncDecl:
				if typed_declaration.Recv == nil && typed_declaration.Name.IsExported() {
					identifiers[typed_declaration.Name.Name] = exported_identifier{typed_declaration.Name.Name, "func", typed_declaration.Doc.Text()}
				}
			case *ast.GenDecl:
				for _, spec := range typed_declaration.Specs {
					type_spec, is_type := spec.(*ast.TypeSpec)
					if !is_type || !type_spec.Name.IsExported() {
						continue
					}
					doc := type_spec.Doc
					if doc == nil && len(typed_declaration.Specs) == 1 {
						doc = typed_declaration.Doc
					}
					identifiers[type_spec.Name.Name] = exported_identifier{type_spec.Name.Name, "type", doc.Text()}
				}
			}
		}
	}
	return identifiers
}

func Test_the_public_file_package_exports_every_name_of_the_shared_list(t *testing.T) {
	content, read_error := os.ReadFile(filepath.Join(file_fixtures_folder, "public_names.json"))
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
	exported := parse_file_package(t)
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

func Test_every_exported_identifier_of_the_file_package_has_a_doc_comment_that_starts_with_its_name(t *testing.T) {
	exported := parse_file_package(t)
	if len(exported) == 0 {
		t.Fatal("the public package exports nothing")
	}
	for _, identifier := range exported {
		if !strings.HasPrefix(identifier.doc, identifier.name+" ") {
			t.Errorf("the %s %s has no doc comment that starts with its name (got %q)", identifier.kind, identifier.name, identifier.doc)
		}
	}
}
