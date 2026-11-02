package http_client_test

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
)

const (
	http_schema_file            = "../../../../../../../contracts/config/http_client.schema.json"
	http_environment_file       = "../../../../../../../docs/configuration/environment-variables.md"
	http_example_file           = "../../../../../../../.env.example"
	http_environment_title      = "## Shared HTTP client"
	http_environment_name_start = "TELEMETRY_HTTP_"
)

func names_of(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func require_same_set(t *testing.T, expected_label string, expected map[string]bool, actual_label string, actual map[string]bool) {
	t.Helper()
	for _, name := range names_of(expected) {
		if !actual[name] {
			t.Errorf("%s is missing from %s (it is in %s)", name, actual_label, expected_label)
		}
	}
	for _, name := range names_of(actual) {
		if !expected[name] {
			t.Errorf("%s is extra in %s (it is not in %s)", name, actual_label, expected_label)
		}
	}
}

func read_file_text(t *testing.T, file_path string) string {
	t.Helper()
	content, read_error := os.ReadFile(file_path)
	if read_error != nil {
		t.Fatal(read_error)
	}
	return string(content)
}

func Test_the_tags_of_settings_are_the_names_of_the_schema_the_document_and_the_example(t *testing.T) {
	// The tags of Settings.
	settings_type := reflect.TypeOf(http_client.Settings{})
	tag_names := map[string]bool{}
	json_names := []string{}
	env_by_json := map[string]string{}
	for position := 0; position < settings_type.NumField(); position++ {
		field := settings_type.Field(position)
		env_tag := field.Tag.Get("env")
		json_tag := field.Tag.Get("json")
		if env_tag == "" || json_tag == "" {
			t.Errorf("the field %s needs a json and an env tag", field.Name)
		}
		if strings.Contains(env_tag, "default=") {
			t.Errorf("the env tag of %s has a default: %q", field.Name, env_tag)
		}
		if strings.Contains(json_tag, ",") {
			t.Errorf("the json tag of %s has options: %q", field.Name, json_tag)
		}
		tag_names[env_tag] = true
		json_names = append(json_names, json_tag)
		env_by_json[json_tag] = env_tag
	}

	// The schema: the order of the properties and the x-env of each one.
	schema_text := read_file_text(t, http_schema_file)
	var schema struct {
		Properties map[string]struct {
			Env string `json:"x-env"`
		} `json:"properties"`
	}
	if decode_error := json.Unmarshal([]byte(schema_text), &schema); decode_error != nil {
		t.Fatal(decode_error)
	}
	schema_names := map[string]bool{}
	for property_name, property := range schema.Properties {
		schema_names[property.Env] = true
		if env_by_json[property_name] != property.Env {
			t.Errorf("%s: the schema has x-env %q and Settings has the tag %q", property_name, property.Env, env_by_json[property_name])
		}
	}
	require_same_set(t, "the schema", schema_names, "the env tags of Settings", tag_names)

	// The order of the properties of the schema is the order of the fields (and of the problems).
	order_decoder := json.NewDecoder(strings.NewReader(schema_text))
	schema_order := []string{}
	in_properties := false
	depth := 0
	for {
		token, token_error := order_decoder.Token()
		if token_error != nil {
			break
		}
		if delimiter, is_delimiter := token.(json.Delim); is_delimiter {
			if delimiter == '{' || delimiter == '[' {
				depth++
			} else {
				depth--
				if in_properties && depth == 1 {
					in_properties = false
				}
			}
			continue
		}
		if text, is_text := token.(string); is_text && depth == 1 && text == "properties" {
			in_properties = true
			continue
		}
		if text, is_text := token.(string); is_text && in_properties && depth == 2 {
			schema_order = append(schema_order, text)
			// skip the value of the property: its tokens are read by the loop and ignored at depth 3
		}
	}
	if !reflect.DeepEqual(schema_order, json_names) {
		t.Errorf("the properties of the schema are %v and the json tags of Settings are %v", schema_order, json_names)
	}

	// The table of the document.
	document := read_file_text(t, http_environment_file)
	_, section_text, found := strings.Cut(document, http_environment_title)
	if !found {
		t.Fatalf("the section %q is not in the document", http_environment_title)
	}
	if next_section := strings.Index(section_text, "\n## "); next_section >= 0 {
		section_text = section_text[:next_section]
	}
	row_pattern := regexp.MustCompile("(?m)^\\| `(" + http_environment_name_start + "[A-Z0-9_]+)`")
	document_names := map[string]bool{}
	for _, match := range row_pattern.FindAllStringSubmatch(section_text, -1) {
		document_names[match[1]] = true
	}
	require_same_set(t, "the env tags of Settings", tag_names, "the table of docs/configuration/environment-variables.md", document_names)

	// The example file.
	line_pattern := regexp.MustCompile(`(?m)^#?\s*(` + http_environment_name_start + `[A-Z0-9_]+)=`)
	example_names := map[string]bool{}
	for _, match := range line_pattern.FindAllStringSubmatch(read_file_text(t, http_example_file), -1) {
		example_names[match[1]] = true
	}
	require_same_set(t, "the env tags of Settings", tag_names, ".env.example", example_names)
}
