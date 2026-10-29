package python_renderer

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

// rules_module is the hand-written module of the rules that the generated validators call.
const rules_module = "telemetry.internal.domain.operations.event_validation.rules"

// ValidationRenderer renders the Python validators of the data of the event types, in the package
// telemetry.internal.domain.operations.event_validation. The code calls the rules written by hand in that package and
// imports only the standard library.
type ValidationRenderer struct{}

// NewValidationRenderer returns the renderer of the Python validators.
func NewValidationRenderer() ValidationRenderer {
	return ValidationRenderer{}
}

// Render returns the generated module. The contract must be sorted.
func (ValidationRenderer) Render(contract domain.Contract) ([]domain.GeneratedFile, error) {
	validated := contract.ValidatedStructs()
	used := map[string]bool{"Validator": true}
	var blocks []string
	for _, declared := range validated {
		used["Problem"], used["ObjectRules"], used["validate_object"] = true, true, true
		blocks = append(blocks, python_validator(contract, declared, used))
	}
	table := "EVENT_VALIDATORS: dict[str, Validator] = {\n"
	if len(contract.EventTypes) == 0 {
		table = "EVENT_VALIDATORS: dict[str, Validator] = {}\n"
	}
	for _, event_type := range contract.EventTypes {
		table += fmt.Sprintf("    %s: validate_%s,\n", strconv.Quote(event_type.Name), domain.EventStructName(event_type.Name))
	}
	if len(contract.EventTypes) > 0 {
		table += "}\n"
	}
	blocks = append(blocks, table)
	imports := rules_import(used)
	if len(validated) > 0 {
		imports = "from typing import Any\n\n" + imports
	}
	content := module(contract.ValidationSources(), "The validators of the data of the event types, generated from the schemas of contracts/schemas.", imports, blocks)
	return []domain.GeneratedFile{{Name: "event_validators_generated.py", Content: []byte(content)}}, nil
}

// rules_import returns the import of the names of the rules module that the generated code uses, one name per line
// when the line would be too long.
func rules_import(used map[string]bool) string {
	names := make([]string, 0, len(used))
	for name := range used {
		names = append(names, name)
	}
	sort.Strings(names)
	single := "from " + rules_module + " import " + strings.Join(names, ", ")
	if len(single) <= line_limit {
		return single + "\n"
	}
	return "from " + rules_module + " import (\n    " + strings.Join(names, ",\n    ") + ",\n)\n"
}

// python_validator renders the function that validates one struct and records the rules it uses.
func python_validator(contract domain.Contract, declared domain.Struct, used map[string]bool) string {
	var text strings.Builder
	fmt.Fprintf(&text, "def validate_%s(value: Any, path: str) -> Problem | None:\n", declared.Name)
	fmt.Fprintf(&text, "    \"\"\"Validate an object that mirrors %s.\"\"\"\n", declared.Source)
	text.WriteString("    return validate_object(\n        value,\n        path,\n        ObjectRules(\n")
	if required := declared.RequiredNames(); len(required) > 0 {
		text.WriteString(tuple_literal("            ", "required=", quote_all(required)))
	}
	if declared.Closed {
		text.WriteString("            closed=True,\n")
	}
	if len(declared.Fields) > 0 {
		used["FieldRule"] = true
		text.WriteString("            fields=(\n")
		for _, field := range declared.AlphabeticalFields() {
			if field.Kind == domain.KindEnum && enum_does_not_fit("                ", field, contract) {
				used["check_enum"] = true
				text.WriteString(long_enum_field("                ", field, contract))
				continue
			}
			check := python_check(contract, field, used)
			text.WriteString(wrap_call("                ", "FieldRule("+strconv.Quote(field.JSONName)+",", check, "),"))
		}
		text.WriteString("            ),\n")
	}
	text.WriteString("        ),\n    )\n")
	return text.String()
}

// quote_all returns every text as a quoted literal.
func quote_all(texts []string) []string {
	quoted := make([]string, len(texts))
	for position, text := range texts {
		quoted[position] = strconv.Quote(text)
	}
	return quoted
}

// tuple_literal returns the line of a keyword argument that is a tuple of literals; when it is longer than the limit
// it puts one item per line.
func tuple_literal(indent string, prefix string, items []string) string {
	trailing := ""
	if len(items) == 1 {
		trailing = ","
	}
	single := indent + prefix + "(" + strings.Join(items, ", ") + trailing + "),"
	if len(single) <= line_limit {
		return single + "\n"
	}
	return indent + prefix + "(\n" + indent + "    " + strings.Join(items, ",\n"+indent+"    ") + ",\n" + indent + "),\n"
}

// wrap_call returns the line of a call with its closing text, with the argument on the next lines when the line would
// be too long.
func wrap_call(indent string, opening string, argument string, closing string) string {
	single := indent + opening + " " + argument + closing
	if len(single) <= line_limit {
		return single + "\n"
	}
	return indent + opening + "\n" + indent + "    " + argument + ",\n" + indent + closing + "\n"
}

// enum_does_not_fit tells if the line of a field whose check is an enum would be longer than the limit.
func enum_does_not_fit(indent string, field domain.Field, contract domain.Contract) bool {
	return len(indent+"FieldRule("+strconv.Quote(field.JSONName)+", "+enum_check(contract, field)+"),") > line_limit
}

// long_enum_field renders a field whose enum does not fit in a line, with one value per line.
func long_enum_field(indent string, field domain.Field, contract domain.Contract) string {
	enum, _ := contract.EnumByName(field.Ref)
	var text strings.Builder
	text.WriteString(indent + "FieldRule(\n" + indent + "    " + strconv.Quote(field.JSONName) + ",\n")
	text.WriteString(indent + "    check_enum(\n" + indent + "        (\n")
	for _, value := range enum.Values {
		text.WriteString(indent + "            " + strconv.Quote(value) + ",\n")
	}
	text.WriteString(indent + "        )\n" + indent + "    ),\n" + indent + "),\n")
	return text.String()
}

// enum_check returns the expression of the check of an enum on one line.
func enum_check(contract domain.Contract, field domain.Field) string {
	enum, _ := contract.EnumByName(field.Ref)
	trailing := ""
	if len(enum.Values) == 1 {
		trailing = ","
	}
	return "check_enum((" + strings.Join(quote_all(enum.Values), ", ") + trailing + "))"
}

// python_check returns the expression of the check of the value of a field and records the rules it uses.
func python_check(contract domain.Contract, field domain.Field, used map[string]bool) string {
	switch field.Kind {
	case domain.KindString:
		used["check_string"] = true
		return fmt.Sprintf("check_string(%d, %d)", field.MinLength, field.MaxLength)
	case domain.KindNumber:
		used["check_number"] = true
		return "check_number"
	case domain.KindInteger:
		used["check_integer"] = true
		return "check_integer"
	case domain.KindBoolean:
		used["check_boolean"] = true
		return "check_boolean"
	case domain.KindEnum:
		used["check_enum"] = true
		return enum_check(contract, field)
	case domain.KindStruct:
		return "validate_" + field.Ref
	case domain.KindAttributes:
		used["AttributesLimits"], used["check_attributes"] = true, true
		limits := contract.Attributes
		return fmt.Sprintf("check_attributes(AttributesLimits(%d, %d, %d, %d))",
			limits.MaxProperties, limits.MinKeyLength, limits.MaxKeyLength, limits.MaxStringLength)
	default:
		used["check_object"] = true
		return "check_object"
	}
}
