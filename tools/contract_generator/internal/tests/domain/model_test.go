package domain_test

import (
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

func TestFieldsAreOrderedRequiredFirstThenAlphabetical(test *testing.T) {
	declared := domain.Struct{Name: "thing", Fields: []domain.Field{
		{JSONName: "zeta", Required: true},
		{JSONName: "beta"},
		{JSONName: "alpha"},
		{JSONName: "eta", Required: true},
	}}
	var got []string
	for _, field := range declared.OrderedFields() {
		got = append(got, field.JSONName)
	}
	expected := []string{"eta", "zeta", "alpha", "beta"}
	for position := range expected {
		if got[position] != expected[position] {
			test.Fatalf("order is %v, want %v", got, expected)
		}
	}
	if declared.Fields[0].JSONName != "zeta" {
		test.Error("OrderedFields must not reorder the struct itself")
	}
}

func TestTheContractIsSortedWithoutChangingTheOriginal(test *testing.T) {
	contract := domain.Contract{
		Enums:       []domain.Enum{{Name: "b", Values: []string{"Z", "A"}}, {Name: "a"}},
		EventTypes:  []domain.EventType{{Name: "b.b.b"}, {Name: "a.a.a"}},
		ErrorCodes:  []domain.ErrorCode{{Code: "y"}, {Code: "x"}},
		Structs:     []domain.Struct{{Name: "log_entry_recorded"}, {Name: "actor"}},
		DataSchemas: []string{"schemas/b/x.schema.json", "schemas/a/x.schema.json"},
	}
	sorted := contract.Sorted()
	if sorted.Enums[0].Name != "a" || sorted.EventTypes[0].Name != "a.a.a" || sorted.ErrorCodes[0].Code != "x" ||
		sorted.Structs[0].Name != "actor" || sorted.DataSchemas[0] != "schemas/a/x.schema.json" {
		test.Errorf("not sorted: %+v", sorted)
	}
	if sorted.Enums[1].Values[0] != "Z" {
		test.Error("the values of an enum keep the order of the file")
	}
	if contract.Enums[0].Name != "b" || contract.DataSchemas[0] != "schemas/b/x.schema.json" {
		test.Error("Sorted changed the original")
	}
}
