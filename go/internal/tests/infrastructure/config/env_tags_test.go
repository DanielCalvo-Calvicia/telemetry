package config_test

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
)

const (
	config_schema_file          = "../../../../../contracts/config/config.schema.json"
	environment_variables_file  = "../../../../../docs/configuration/environment-variables.md"
	environment_example_file    = "../../../../../.env.example"
	environment_variables_title = "## Core configuration of the new standard"
)

// tags_of_config returns the env tags of Config by the path of the field (the json tags, as in the schema).
func tags_of_config(config_type reflect.Type, path_prefix string, tags map[string]string) {
	for position := 0; position < config_type.NumField(); position++ {
		field := config_type.Field(position)
		json_name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		path := path_prefix + json_name
		if field.Type.Kind() == reflect.Struct {
			tags_of_config(field.Type, path+".", tags)
			continue
		}
		tags[path] = field.Tag.Get("env")
	}
}

// names_of_set returns the sorted names of a set.
func names_of_set(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// require_same_names fails, naming each missing and extra name, when the two sets differ.
func require_same_names(t *testing.T, expected_label string, expected map[string]bool, actual_label string, actual map[string]bool) {
	t.Helper()
	for _, name := range names_of_set(expected) {
		if !actual[name] {
			t.Errorf("%s is missing from %s (it is in %s)", name, actual_label, expected_label)
		}
	}
	for _, name := range names_of_set(actual) {
		if !expected[name] {
			t.Errorf("%s is extra in %s (it is not in %s)", name, actual_label, expected_label)
		}
	}
}

func read_test_file(t *testing.T, file_path string) string {
	t.Helper()
	content, read_error := os.ReadFile(file_path)
	if read_error != nil {
		t.Fatal(read_error)
	}
	return string(content)
}

func Test_the_env_tags_of_config_are_the_names_of_the_schema_the_documents_and_the_example(t *testing.T) {
	// The tags of Config.
	tags_by_path := map[string]string{}
	tags_of_config(reflect.TypeOf(config.Config{}), "", tags_by_path)
	tag_names := map[string]bool{}
	for path, tag := range tags_by_path {
		if tag == "" {
			t.Errorf("the field %s has no env tag", path)
		}
		if strings.Contains(tag, "default=") {
			t.Errorf("the env tag of %s has a default: %q", path, tag)
		}
		tag_names[tag] = true
	}

	// The schema: every property, with its x-env, and the same set of paths as Config.
	var schema struct {
		Properties map[string]struct {
			Properties map[string]struct {
				Env string `json:"x-env"`
			} `json:"properties"`
		} `json:"properties"`
	}
	if decode_error := json.Unmarshal([]byte(read_test_file(t, config_schema_file)), &schema); decode_error != nil {
		t.Fatal(decode_error)
	}
	schema_names := map[string]bool{}
	schema_paths := map[string]bool{}
	for section_name, section := range schema.Properties {
		for field_name, field := range section.Properties {
			path := section_name + "." + field_name
			schema_paths[path] = true
			schema_names[field.Env] = true
			if tags_by_path[path] != field.Env {
				t.Errorf("%s: the schema has x-env %q and Config has the tag %q", path, field.Env, tags_by_path[path])
			}
		}
	}
	config_paths := map[string]bool{}
	for path := range tags_by_path {
		config_paths[path] = true
	}
	require_same_names(t, "the schema", schema_paths, "Config", config_paths)
	require_same_names(t, "the schema", schema_names, "the env tags of Config", tag_names)

	// The table of the document.
	document := read_test_file(t, environment_variables_file)
	_, section_text, found := strings.Cut(document, environment_variables_title)
	if !found {
		t.Fatalf("the section %q is not in the document", environment_variables_title)
	}
	if next_section := strings.Index(section_text, "\n## "); next_section >= 0 {
		section_text = section_text[:next_section]
	}
	row_pattern := regexp.MustCompile("(?m)^\\| `(TELEMETRY_[A-Z0-9_]+)`")
	document_names := map[string]bool{}
	for _, match := range row_pattern.FindAllStringSubmatch(section_text, -1) {
		document_names[match[1]] = true
	}
	require_same_names(t, "the env tags of Config", tag_names, "the table of docs/configuration/environment-variables.md", document_names)

	// The example file.
	line_pattern := regexp.MustCompile(`(?m)^#?\s*(TELEMETRY_[A-Z0-9_]+)=`)
	example_names := map[string]bool{}
	for _, match := range line_pattern.FindAllStringSubmatch(read_test_file(t, environment_example_file), -1) {
		example_names[match[1]] = true
	}
	require_same_names(t, "the env tags of Config", tag_names, ".env.example", example_names)
}
