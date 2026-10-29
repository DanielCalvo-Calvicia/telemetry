package contract_folder

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

// utc_time_fragment is the local definition of an instant in UTC in the envelope; the generator maps it to a string.
const utc_time_fragment = "/$defs/utc_time"

// attributes_name is the base name of the schema of the attributes, which becomes a map and not a struct.
const attributes_name = "attributes"

// registry_file is the shape of events/registry.yaml.
type registry_file struct {
	Events []registry_entry `yaml:"events"`
}

type registry_entry struct {
	EventType string `yaml:"event_type"`
	Schema    string `yaml:"schema"`
}

// catalog_file is the shape of errors/catalog.yaml.
type catalog_file struct {
	Errors []catalog_entry `yaml:"errors"`
}

type catalog_entry struct {
	Code           string `yaml:"code"`
	Category       string `yaml:"category"`
	DefaultMessage string `yaml:"default_message"`
}

// target is what a $ref to a document points to.
type target struct {
	kind domain.FieldKind
	name string
}

// pending_struct is a schema document that becomes a struct.
type pending_struct struct {
	declared document
	name     string
}

// build_contract turns the parsed documents, the registry and the catalog into the model.
func build_contract(documents []document, registry registry_file, catalog catalog_file) (domain.Contract, error) {
	contract := domain.Contract{}
	registered := map[string]string{}
	for _, entry := range registry.Events {
		contract.EventTypes = append(contract.EventTypes, domain.EventType{
			Name: entry.EventType, Schema: entry.Schema, Source: domain.SourcePrefix + domain.RegistryPath,
		})
		registered[entry.Schema] = entry.EventType
	}
	for _, entry := range catalog.Errors {
		contract.ErrorCodes = append(contract.ErrorCodes, domain.ErrorCode{
			Code: entry.Code, Category: entry.Category, DefaultMessage: entry.DefaultMessage, Source: domain.SourcePrefix + domain.CatalogPath,
		})
	}

	targets := map[string]target{}
	var pending []pending_struct
	var failures []error
	for _, declared := range documents {
		if is_enum_file(declared.path) {
			enum, enum_error := read_enum(declared)
			if enum_error != nil {
				failures = append(failures, enum_error)
				continue
			}
			contract.Enums = append(contract.Enums, enum)
			targets[declared.id] = target{kind: domain.KindEnum, name: enum.Name}
			continue
		}
		name, is_struct := struct_name(declared.path, registered)
		if is_data_schema(declared.path) {
			contract.DataSchemas = append(contract.DataSchemas, declared.path)
		}
		switch {
		case name == attributes_name && path.Dir(declared.path) == "schemas/"+common_folder:
			targets[declared.id] = target{kind: domain.KindAttributes}
			contract.Attributes = read_attributes_limits(declared)
		case is_struct:
			targets[declared.id] = target{kind: domain.KindStruct, name: name}
			pending = append(pending, pending_struct{declared: declared, name: name})
		}
	}
	for _, awaiting := range pending {
		built, struct_error := map_struct(awaiting, targets)
		if struct_error != nil {
			failures = append(failures, struct_error)
			continue
		}
		contract.Structs = append(contract.Structs, built)
	}
	return contract, errors.Join(failures...)
}

// is_data_schema tells if a schema path is under a domain folder (schemas/<domain>/...), not the envelope and not
// the common objects: the place of the data schemas of the event types.
func is_data_schema(file_path string) bool {
	relative := strings.TrimPrefix(file_path, "schemas/")
	return strings.Contains(relative, "/") && !strings.HasPrefix(relative, common_folder+"/")
}

// struct_name returns the name of the struct a schema file becomes and whether it becomes one: the envelope and the
// common objects are named by their file, and the data schema of a registered event type by the event type.
func struct_name(file_path string, registered map[string]string) (string, bool) {
	if event_type, found := registered[file_path]; found {
		return strings.ReplaceAll(event_type, ".", "_"), true
	}
	if is_data_schema(file_path) {
		return "", false
	}
	return strings.TrimSuffix(path.Base(file_path), schema_suffix), true
}

// read_enum reads the values of an enum file.
func read_enum(declared document) (domain.Enum, error) {
	source := domain.SourcePrefix + declared.path
	values, is_list := declared.body["enum"].([]any)
	if !is_list {
		return domain.Enum{}, fmt.Errorf("%w: %s: no enum list", application.ErrUnsupportedSchema, source)
	}
	enum := domain.Enum{Name: strings.TrimSuffix(path.Base(declared.path), ".json"), Source: source}
	for _, value := range values {
		text, is_text := value.(string)
		if !is_text {
			return domain.Enum{}, fmt.Errorf("%w: %s: the value %v is not a string", application.ErrUnsupportedSchema, source, value)
		}
		enum.Values = append(enum.Values, text)
	}
	return enum, nil
}

// map_struct turns an object schema into a struct.
func map_struct(awaiting pending_struct, targets map[string]target) (domain.Struct, error) {
	declared := awaiting.declared
	source := domain.SourcePrefix + declared.path
	if kind, _ := declared.body["type"].(string); kind != "object" {
		return domain.Struct{}, fmt.Errorf("%w: %s: the type is not object", application.ErrUnsupportedSchema, source)
	}
	properties, _ := declared.body["properties"].(map[string]any)
	required := map[string]bool{}
	required_names, _ := declared.body["required"].([]any)
	for _, required_name := range required_names {
		text, _ := required_name.(string)
		if _, found := properties[text]; !found {
			return domain.Struct{}, fmt.Errorf("%w: %s: %v is required but not a property", application.ErrUnsupportedSchema, source, required_name)
		}
		required[text] = true
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	built := domain.Struct{Name: awaiting.name, Source: source, Closed: declared.body["additionalProperties"] == false}
	built.Unsupported = unsupported_keywords(declared.body, struct_keywords)
	for _, name := range names {
		property, _ := properties[name].(map[string]any)
		field, field_error := map_field(declared, name, property, targets)
		if field_error != nil {
			return domain.Struct{}, field_error
		}
		field.Required = required[name]
		built.Fields = append(built.Fields, field)
	}
	return built, nil
}

// map_field turns a property of an object schema into a field.
func map_field(declared document, name string, property map[string]any, targets map[string]target) (domain.Field, error) {
	field := domain.Field{JSONName: name}
	field.Unsupported = unsupported_keywords(property, property_keywords)
	if reference, has_reference := property["$ref"].(string); has_reference {
		return map_reference(declared, field, reference, targets)
	}
	kind, _ := property["type"].(string)
	switch kind {
	case "string":
		field.Kind = domain.KindString
		field.MinLength, field.MaxLength = integer_of(property["minLength"]), integer_of(property["maxLength"])
		if _, has_maximum := property["maxLength"]; has_maximum && field.MaxLength < 1 {
			return field, unsupported(declared, name, "a maxLength below 1")
		}
	case "number":
		field.Kind = domain.KindNumber
	case "integer":
		field.Kind = domain.KindInteger
	case "boolean":
		field.Kind = domain.KindBoolean
	case "object":
		if _, has_properties := property["properties"]; has_properties {
			return field, unsupported(declared, name, "a nested object with properties")
		}
		field.Kind = domain.KindMap
	default:
		return field, unsupported(declared, name, fmt.Sprintf("the type %q", kind))
	}
	return field, nil
}

// map_reference turns a property that is a $ref into a field.
func map_reference(declared document, field domain.Field, reference string, targets map[string]target) (domain.Field, error) {
	base, base_error := url.Parse(declared.id)
	relative, relative_error := url.Parse(reference)
	if base_error != nil || relative_error != nil {
		return field, unsupported(declared, field.JSONName, fmt.Sprintf("the reference %q", reference))
	}
	resolved := base.ResolveReference(relative)
	fragment := resolved.Fragment
	resolved.Fragment = ""
	if fragment != "" {
		if fragment == utc_time_fragment && resolved.String() == declared.id {
			field.Kind = domain.KindString
			return field, nil
		}
		return field, unsupported(declared, field.JSONName, fmt.Sprintf("the reference %q", reference))
	}
	pointed, known := targets[resolved.String()]
	if !known {
		return field, unsupported(declared, field.JSONName, fmt.Sprintf("the reference %q, which is not an enum or a schema of the contract", reference))
	}
	field.Kind = pointed.kind
	field.Ref = pointed.name
	return field, nil
}

// property_keywords are the keywords of a property that the generated validation honours or that only describe it.
var property_keywords = map[string]bool{
	"type": true, "$ref": true, "description": true, "title": true, "$comment": true, "examples": true,
	"minLength": true, "maxLength": true,
}

// struct_keywords are the keywords of an object schema that the generated validation honours or that only describe it.
var struct_keywords = map[string]bool{
	"$schema": true, "$id": true, "$defs": true, "$comment": true, "title": true, "description": true, "type": true,
	"required": true, "properties": true, "additionalProperties": true,
}

// unsupported_keywords returns, sorted and comma separated, the keywords of a schema that are not in the allowed set.
// An additionalProperties that is not a boolean is also unsupported: the validation knows only "closed" and "open".
func unsupported_keywords(body map[string]any, allowed map[string]bool) string {
	var found []string
	for keyword, value := range body {
		_, is_flag := value.(bool)
		if !allowed[keyword] || (keyword == "additionalProperties" && !is_flag) {
			found = append(found, keyword)
		}
	}
	sort.Strings(found)
	return strings.Join(found, ", ")
}

// integer_of returns a JSON number as an int: a whole number written with or without a zero fraction (2 or 2.0). It
// returns 0 for anything else, which means no limit.
func integer_of(value any) int {
	number, is_number := value.(json.Number)
	if !is_number {
		return 0
	}
	if parsed, parse_error := number.Int64(); parse_error == nil {
		return int(parsed)
	}
	parsed, parse_error := number.Float64()
	if parse_error != nil || parsed != math.Trunc(parsed) {
		return 0
	}
	return int(parsed)
}

// read_attributes_limits reads the limits of the attributes schema: the most keys, the length of a key and the length
// of a string value. A limit the schema does not state stays 0, which means no limit.
func read_attributes_limits(declared document) domain.AttributesLimits {
	limits := domain.AttributesLimits{Source: domain.SourcePrefix + declared.path, MaxProperties: integer_of(declared.body["maxProperties"])}
	names, _ := declared.body["propertyNames"].(map[string]any)
	limits.MinKeyLength, limits.MaxKeyLength = integer_of(names["minLength"]), integer_of(names["maxLength"])
	definitions, _ := declared.body["$defs"].(map[string]any)
	scalar, _ := definitions["scalar"].(map[string]any)
	alternatives, _ := scalar["anyOf"].([]any)
	for _, alternative := range alternatives {
		candidate, _ := alternative.(map[string]any)
		if candidate["type"] == "string" {
			limits.MaxStringLength = integer_of(candidate["maxLength"])
		}
	}
	return limits
}

// unsupported builds the error of a property the generator cannot map.
func unsupported(declared document, name string, what string) error {
	return fmt.Errorf("%w: %s%s: property %q: %s", application.ErrUnsupportedSchema, domain.SourcePrefix, declared.path, name, what)
}
