package http_destination_test

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	http_destination "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/http"
)

const (
	client_schema_file      = "../../../../../../../contracts/config/http_client.schema.json"
	destination_schema_file = "../../../../../../../contracts/config/http_destination.schema.json"
	environment_file        = "../../../../../../../docs/configuration/environment-variables.md"
	example_file            = "../../../../../../../.env.example"
	environment_name_start  = "TELEMETRY_HTTP_"
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

// schema_properties returns the property names of a schema in the order of the file and the x-env of each one.
func schema_properties(t *testing.T, file_path string) ([]string, map[string]string) {
	t.Helper()
	schema_text := read_file_text(t, file_path)
	var schema struct {
		Properties map[string]struct {
			Env string `json:"x-env"`
		} `json:"properties"`
	}
	if decode_error := json.Unmarshal([]byte(schema_text), &schema); decode_error != nil {
		t.Fatal(decode_error)
	}
	env_by_property := map[string]string{}
	for property_name, property := range schema.Properties {
		env_by_property[property_name] = property.Env
	}

	order_decoder := json.NewDecoder(strings.NewReader(schema_text))
	order := []string{}
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
			order = append(order, text)
		}
	}
	return order, env_by_property
}

// section_of returns the text of a section of the document, from its title to the next "## " heading.
func section_of(t *testing.T, document string, title string) string {
	t.Helper()
	_, section_text, found := strings.Cut(document, title)
	if !found {
		t.Fatalf("the section %q is not in the document", title)
	}
	if next_section := strings.Index(section_text, "\n## "); next_section >= 0 {
		section_text = section_text[:next_section]
	}
	return section_text
}

func Test_the_tags_of_settings_are_the_names_of_the_two_schemas_the_document_and_the_example(t *testing.T) {
	// The tags of Settings.
	settings_type := reflect.TypeOf(http_destination.Settings{})
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

	// The two schemas: the client properties first, then the ones of the destination.
	client_order, client_env := schema_properties(t, client_schema_file)
	destination_order, destination_env := schema_properties(t, destination_schema_file)
	schema_names := map[string]bool{}
	for _, env_by_property := range []map[string]string{client_env, destination_env} {
		for property_name, env_name := range env_by_property {
			schema_names[env_name] = true
			if env_by_json[property_name] != env_name {
				t.Errorf("%s: the schema has x-env %q and Settings has the tag %q", property_name, env_name, env_by_json[property_name])
			}
		}
	}
	require_same_set(t, "the two schemas", schema_names, "the env tags of Settings", tag_names)
	expected_order := append(append([]string{}, client_order...), destination_order...)
	if !reflect.DeepEqual(expected_order, json_names) {
		t.Errorf("the properties of the schemas are %v and the json tags of Settings are %v", expected_order, json_names)
	}

	// The table of the document: the section of the client and the one of the destination together.
	document := read_file_text(t, environment_file)
	section_text := section_of(t, document, "## Shared HTTP client") + section_of(t, document, "## HTTP destination (")
	row_pattern := regexp.MustCompile("(?m)^\\| `(" + environment_name_start + "[A-Z0-9_]+)`")
	document_names := map[string]bool{}
	for _, match := range row_pattern.FindAllStringSubmatch(section_text, -1) {
		document_names[match[1]] = true
	}
	require_same_set(t, "the env tags of Settings", tag_names, "the tables of docs/configuration/environment-variables.md", document_names)

	// The example file.
	line_pattern := regexp.MustCompile(`(?m)^#?\s*(` + environment_name_start + `[A-Z0-9_]+)=`)
	example_names := map[string]bool{}
	for _, match := range line_pattern.FindAllStringSubmatch(read_file_text(t, example_file), -1) {
		example_names[match[1]] = true
	}
	require_same_set(t, "the env tags of Settings", tag_names, ".env.example", example_names)
}

func Test_the_client_settings_carry_the_same_tags_as_the_shared_client(t *testing.T) {
	destination_type := reflect.TypeOf(http_destination.Settings{})
	client_type := reflect.TypeOf(http_destination.Settings{}.ClientSettings())
	for position := 0; position < client_type.NumField(); position++ {
		client_field := client_type.Field(position)
		destination_field, found := destination_type.FieldByName(client_field.Name)
		if !found || destination_field.Tag != client_field.Tag || destination_field.Type != client_field.Type {
			t.Errorf("the field %s of the shared client differs in the settings of the destination", client_field.Name)
		}
	}
	if destination_type.NumField() != client_type.NumField()+1 {
		t.Errorf("the destination must add exactly one field to the client, has %d and %d", destination_type.NumField(), client_type.NumField())
	}
}
