package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

const (
	enum_source     = "contracts/enums/severity.json"
	registry_source = "contracts/events/registry.yaml"
	catalog_source  = "contracts/errors/catalog.yaml"
)

func valid_contract() domain.Contract {
	return domain.Contract{
		Enums: []domain.Enum{
			{Name: "failure_category", Source: "contracts/enums/failure_category.json", Values: []string{"validation"}},
			{Name: "severity", Source: enum_source, Values: []string{"INFO", "ERROR"}},
		},
		EventTypes: []domain.EventType{
			{Name: "log.entry.recorded", Schema: "schemas/log/entry_recorded.schema.json", Source: registry_source},
		},
		ErrorCodes: []domain.ErrorCode{
			{Code: "event_schema_invalid", Category: "validation", DefaultMessage: "invalid", Source: catalog_source},
		},
		DataSchemas: []string{"schemas/log/entry_recorded.schema.json"},
	}
}

func TestAValidContractHasNoBreach(test *testing.T) {
	if validation_error := valid_contract().Validate(); validation_error != nil {
		test.Fatalf("unexpected error: %v", validation_error)
	}
}

func TestEveryBreachIsReportedWithItsFile(test *testing.T) {
	cases := []struct {
		name     string
		break_it func(contract *domain.Contract)
		sentinel error
		file     string
	}{
		{"a value that is not an identifier", func(contract *domain.Contract) {
			contract.Enums[1].Values = []string{"two words"}
		}, domain.ErrInvalidEnumValue, enum_source},
		{"a duplicated enum value", func(contract *domain.Contract) {
			contract.Enums[1].Values = []string{"INFO", "INFO"}
		}, domain.ErrDuplicateEnumValue, enum_source},
		{"values that differ only in case", func(contract *domain.Contract) {
			contract.Enums[1].Values = []string{"info", "INFO"}
		}, domain.ErrDuplicateEnumValue, enum_source},
		{"an event type with two segments", func(contract *domain.Contract) {
			contract.EventTypes[0].Name = "log.entry"
		}, domain.ErrInvalidEventType, registry_source},
		{"an event type in capitals", func(contract *domain.Contract) {
			contract.EventTypes[0].Name = "Log.entry.recorded"
		}, domain.ErrInvalidEventType, registry_source},
		{"a duplicated event type", func(contract *domain.Contract) {
			contract.EventTypes = append(contract.EventTypes, contract.EventTypes[0])
		}, domain.ErrDuplicateEventType, registry_source},
		{"a schema that is not the one of the type", func(contract *domain.Contract) {
			contract.EventTypes[0].Schema = "schemas/log/other.schema.json"
		}, domain.ErrSchemaPathMismatch, registry_source},
		{"a registered type without schema", func(contract *domain.Contract) {
			contract.DataSchemas = nil
		}, domain.ErrSchemaMissing, registry_source},
		{"a schema that is not registered", func(contract *domain.Contract) {
			contract.DataSchemas = append(contract.DataSchemas, "schemas/log/unregistered.schema.json")
		}, domain.ErrSchemaNotRegistered, "contracts/schemas/log/unregistered.schema.json"},
		{"a code in capitals", func(contract *domain.Contract) {
			contract.ErrorCodes[0].Code = "Event-Schema"
		}, domain.ErrInvalidErrorCode, catalog_source},
		{"a duplicated code", func(contract *domain.Contract) {
			contract.ErrorCodes = append(contract.ErrorCodes, contract.ErrorCodes[0])
		}, domain.ErrDuplicateErrorCode, catalog_source},
		{"an unknown category", func(contract *domain.Contract) {
			contract.ErrorCodes[0].Category = "nonsense"
		}, domain.ErrUnknownErrorCategory, catalog_source},
		{"no failure category enum", func(contract *domain.Contract) {
			contract.Enums = contract.Enums[1:]
		}, domain.ErrUnknownErrorCategory, catalog_source},
		{"a blank message", func(contract *domain.Contract) {
			contract.ErrorCodes[0].DefaultMessage = "  "
		}, domain.ErrMissingDefaultMessage, catalog_source},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			contract := valid_contract()
			each.break_it(&contract)
			validation_error := contract.Validate()
			if !errors.Is(validation_error, each.sentinel) {
				test.Fatalf("error %v does not wrap %v", validation_error, each.sentinel)
			}
			if !strings.Contains(validation_error.Error(), each.file) {
				test.Errorf("error %q does not name %s", validation_error, each.file)
			}
		})
	}
}
