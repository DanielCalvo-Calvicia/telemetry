package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

// contract_with_data is a contract with one event type whose data reaches a nested struct, an enum and the attributes,
// and with a struct that no event reaches.
func contract_with_data() domain.Contract {
	return domain.Contract{
		EventTypes: []domain.EventType{{Name: "demo.thing.happened"}},
		Enums:      []domain.Enum{{Name: "severity", Source: "contracts/enums/severity.json", Values: []string{"INFO"}}},
		Attributes: domain.AttributesLimits{Source: "contracts/schemas/common/attributes.schema.json", MaxProperties: 3},
		Structs: []domain.Struct{
			{Name: "demo_thing_happened", Source: "contracts/schemas/demo/thing_happened.schema.json", Closed: true, Fields: []domain.Field{
				{JSONName: "resource", Kind: domain.KindStruct, Ref: "resource"},
				{JSONName: "severity", Kind: domain.KindEnum, Ref: "severity", Required: true},
				{JSONName: "labels", Kind: domain.KindAttributes},
				{JSONName: "ghost", Kind: domain.KindStruct, Ref: "no_such_struct"},
			}},
			{Name: "resource", Source: "contracts/schemas/common/resource.schema.json", Fields: []domain.Field{
				{JSONName: "inner", Kind: domain.KindStruct, Ref: "inner"},
				{JSONName: "again", Kind: domain.KindStruct, Ref: "resource"},
			}},
			{Name: "inner", Source: "contracts/schemas/common/inner.schema.json"},
			{Name: "envelope", Source: "contracts/schemas/envelope.schema.json"},
			{Name: "unreached", Source: "contracts/schemas/common/unreached.schema.json", Unsupported: "pattern"},
		},
	}
}

func TestTheStructOfAnEventTypeIsNamedByTheType(test *testing.T) {
	if got := domain.EventStructName("log.entry.recorded"); got != "log_entry_recorded" {
		test.Errorf("got %q", got)
	}
}

func TestFieldsAndRequiredNamesAreAlphabetical(test *testing.T) {
	declared := domain.Struct{Fields: []domain.Field{
		{JSONName: "zeta", Required: true}, {JSONName: "beta"}, {JSONName: "alpha", Required: true},
	}}
	var names []string
	for _, field := range declared.AlphabeticalFields() {
		names = append(names, field.JSONName)
	}
	if strings.Join(names, ",") != "alpha,beta,zeta" || strings.Join(declared.RequiredNames(), ",") != "alpha,zeta" {
		test.Errorf("fields %v, required %v", names, declared.RequiredNames())
	}
	if declared.Fields[0].JSONName != "zeta" {
		test.Error("AlphabeticalFields must not reorder the struct itself")
	}
	if len((domain.Struct{}).RequiredNames()) != 0 {
		test.Error("a struct with no field has no required name")
	}
}

func TestAnEnumIsFoundByName(test *testing.T) {
	contract := contract_with_data()
	if enum, found := contract.EnumByName("severity"); !found || enum.Values[0] != "INFO" {
		test.Errorf("got %+v, %v", enum, found)
	}
	if _, found := contract.EnumByName("nothing"); found {
		test.Error("an enum that does not exist must not be found")
	}
}

func TestTheValidatedStructsAreTheDataOfTheEventTypesAndWhatTheyReach(test *testing.T) {
	var names []string
	for _, declared := range contract_with_data().ValidatedStructs() {
		names = append(names, declared.Name)
	}
	if got := strings.Join(names, ","); got != "demo_thing_happened,inner,resource" {
		test.Errorf("validated: %s (the envelope and a struct no event reaches are not validated, and a cycle ends)", got)
	}
	if len((domain.Contract{}).ValidatedStructs()) != 0 {
		test.Error("a contract with no event type validates nothing")
	}
}

func TestTheSourcesOfTheValidationAreTheRegistryTheStructsTheEnumsAndTheAttributes(test *testing.T) {
	got := strings.Join(contract_with_data().ValidationSources(), "\n")
	expected := strings.Join([]string{
		"contracts/enums/severity.json",
		"contracts/events/registry.yaml",
		"contracts/schemas/common/attributes.schema.json",
		"contracts/schemas/common/inner.schema.json",
		"contracts/schemas/common/resource.schema.json",
		"contracts/schemas/demo/thing_happened.schema.json",
	}, "\n")
	if got != expected {
		test.Errorf("got:\n%s\nwant:\n%s", got, expected)
	}
	if only := (domain.Contract{}).ValidationSources(); len(only) != 1 || only[0] != "contracts/events/registry.yaml" {
		test.Errorf("a contract with no event type has only the registry as a source: %v", only)
	}
}

func TestAConstructThatCannotBeValidatedIsReportedOnlyWhenAnEventReachesIt(test *testing.T) {
	contract := contract_with_data()
	if check_error := contract.CheckValidatable(); check_error != nil {
		test.Fatalf("the unsupported keyword of an unreached struct must not matter: %v", check_error)
	}
	contract.Structs[1].Unsupported = "minProperties"
	contract.Structs[1].Fields[0].Unsupported = "format, pattern"
	check_error := contract.CheckValidatable()
	if !errors.Is(check_error, domain.ErrUnsupportedSchema) {
		test.Fatalf("error %v", check_error)
	}
	for _, expected := range []string{
		"unsupported schema construct: contracts/schemas/common/resource.schema.json: the keyword minProperties",
		`unsupported schema construct: contracts/schemas/common/resource.schema.json: property "inner": the keyword format, pattern`,
	} {
		if !strings.Contains(check_error.Error(), expected) {
			test.Errorf("error %q does not contain %q", check_error, expected)
		}
	}
}
