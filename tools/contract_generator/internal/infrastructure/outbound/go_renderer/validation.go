package go_renderer

import (
	"fmt"
	"go/format"
	"strconv"
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

// validation_package_name is the Go package of the generated validators.
const validation_package_name = "event_validation"

// ValidationRenderer renders the Go validators of the data of the event types, in the package event_validation of the
// domain layer. The code calls the rules written by hand in that package and uses only the standard library.
type ValidationRenderer struct{}

// NewValidationRenderer returns the renderer of the Go validators.
func NewValidationRenderer() ValidationRenderer {
	return ValidationRenderer{}
}

// Render returns the generated file, formatted with gofmt. The contract must be sorted.
func (ValidationRenderer) Render(contract domain.Contract) ([]domain.GeneratedFile, error) {
	const file_name = "event_validators_generated.go"
	var body strings.Builder
	body.WriteString("\n// event_validators maps every registered event type to the validator of its data.\n")
	body.WriteString("var event_validators = map[string]Validator{\n")
	for _, event_type := range contract.EventTypes {
		fmt.Fprintf(&body, "\t%s: validate_%s,\n", strconv.Quote(event_type.Name), domain.EventStructName(event_type.Name))
	}
	body.WriteString("}\n")
	for _, declared := range contract.ValidatedStructs() {
		body.WriteString(render_validator(contract, declared))
	}
	content := header_of(validation_package_name, contract.ValidationSources()) + body.String()
	formatted, format_error := format.Source([]byte(content))
	if format_error != nil {
		return nil, fmt.Errorf("%w: %s: %w", application.ErrRenderCode, file_name, format_error)
	}
	return []domain.GeneratedFile{{Name: file_name, Content: formatted}}, nil
}

// render_validator renders the function that validates one struct.
func render_validator(contract domain.Contract, declared domain.Struct) string {
	var text strings.Builder
	fmt.Fprintf(&text, "\n// validate_%s validates an object that mirrors %s.\n", declared.Name, declared.Source)
	fmt.Fprintf(&text, "func validate_%s(value any, path string) *Problem {\n", declared.Name)
	text.WriteString("\treturn ValidateObject(value, path, ObjectRules{\n")
	if required := declared.RequiredNames(); len(required) > 0 {
		quoted := make([]string, len(required))
		for position, name := range required {
			quoted[position] = strconv.Quote(name)
		}
		fmt.Fprintf(&text, "\t\tRequired: []string{%s},\n", strings.Join(quoted, ", "))
	}
	if declared.Closed {
		text.WriteString("\t\tClosed: true,\n")
	}
	if len(declared.Fields) > 0 {
		text.WriteString("\t\tFields: []FieldRule{\n")
		for _, field := range declared.AlphabeticalFields() {
			fmt.Fprintf(&text, "\t\t\t{Name: %s, Check: %s},\n", strconv.Quote(field.JSONName), go_check(contract, field))
		}
		text.WriteString("\t\t},\n")
	}
	text.WriteString("\t})\n}\n")
	return text.String()
}

// go_check returns the expression of the check of the value of a field.
func go_check(contract domain.Contract, field domain.Field) string {
	switch field.Kind {
	case domain.KindString:
		return fmt.Sprintf("CheckString(%d, %d)", field.MinLength, field.MaxLength)
	case domain.KindNumber:
		return "CheckNumber"
	case domain.KindInteger:
		return "CheckInteger"
	case domain.KindBoolean:
		return "CheckBoolean"
	case domain.KindEnum:
		enum, _ := contract.EnumByName(field.Ref)
		quoted := make([]string, len(enum.Values))
		for position, value := range enum.Values {
			quoted[position] = strconv.Quote(value)
		}
		return "CheckEnum(" + strings.Join(quoted, ", ") + ")"
	case domain.KindStruct:
		return "validate_" + field.Ref
	case domain.KindAttributes:
		limits := contract.Attributes
		return fmt.Sprintf("CheckAttributes(AttributesLimits{MaxProperties: %d, MinKeyLength: %d, MaxKeyLength: %d, MaxStringLength: %d})",
			limits.MaxProperties, limits.MinKeyLength, limits.MaxKeyLength, limits.MaxStringLength)
	default:
		return "CheckObject"
	}
}
