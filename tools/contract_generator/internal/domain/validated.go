package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrUnsupportedSchema is returned for a schema that uses a construct the generator cannot turn into a field or into
// validation code. It is wrapped with the path of the file and, when there is one, the property.
var ErrUnsupportedSchema = errors.New("unsupported schema construct")

// EventStructName returns the name of the struct of the data of an event type: the type with underscores instead of
// dots (log.entry.recorded gives log_entry_recorded).
func EventStructName(event_type string) string {
	return strings.ReplaceAll(event_type, ".", "_")
}

// AlphabeticalFields returns the fields in alphabetical order of their JSON name, the order in which the generated
// validation checks them in both languages.
func (declared Struct) AlphabeticalFields() []Field {
	ordered := make([]Field, len(declared.Fields))
	copy(ordered, declared.Fields)
	sort.SliceStable(ordered, func(left int, right int) bool { return ordered[left].JSONName < ordered[right].JSONName })
	return ordered
}

// RequiredNames returns the names of the required fields in alphabetical order.
func (declared Struct) RequiredNames() []string {
	var names []string
	for _, field := range declared.AlphabeticalFields() {
		if field.Required {
			names = append(names, field.JSONName)
		}
	}
	return names
}

// EnumByName returns the enum with a name, and whether it exists.
func (contract Contract) EnumByName(name string) (Enum, bool) {
	for _, enum := range contract.Enums {
		if enum.Name == name {
			return enum, true
		}
	}
	return Enum{}, false
}

// ValidatedStructs returns, in alphabetical order of their name, the struct of the data of every registered event
// type and every struct they reach through a reference: the ones the generated validation covers.
func (contract Contract) ValidatedStructs() []Struct {
	by_name := map[string]Struct{}
	for _, declared := range contract.Structs {
		by_name[declared.Name] = declared
	}
	reached := map[string]Struct{}
	var pending []string
	for _, event_type := range contract.EventTypes {
		pending = append(pending, EventStructName(event_type.Name))
	}
	for len(pending) > 0 {
		name := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		declared, known := by_name[name]
		if _, done := reached[name]; done || !known {
			continue
		}
		reached[name] = declared
		for _, field := range declared.Fields {
			if field.Kind == KindStruct {
				pending = append(pending, field.Ref)
			}
		}
	}
	validated := make([]Struct, 0, len(reached))
	for _, declared := range reached {
		validated = append(validated, declared)
	}
	sort.Slice(validated, func(left int, right int) bool { return validated[left].Name < validated[right].Name })
	return validated
}

// CheckValidatable returns an error, naming the file and the property, for every construct of a validated struct that
// the generated validation cannot honour. A construct that only the generated types ignore is not an error as long as
// no event type reaches it: the envelope and the objects no event data uses may carry a pattern, for example.
func (contract Contract) CheckValidatable() error {
	var failures []error
	for _, declared := range contract.ValidatedStructs() {
		if declared.Unsupported != "" {
			failures = append(failures, fmt.Errorf("%w: %s: the keyword %s cannot be validated",
				ErrUnsupportedSchema, declared.Source, declared.Unsupported))
		}
		for _, field := range declared.AlphabeticalFields() {
			if field.Unsupported != "" {
				failures = append(failures, fmt.Errorf("%w: %s: property %q: the keyword %s cannot be validated",
					ErrUnsupportedSchema, declared.Source, field.JSONName, field.Unsupported))
			}
		}
	}
	return errors.Join(failures...)
}

// ValidationSources returns, sorted, the files the generated validation is made from: the registry, the schema of
// every validated struct, the enums they reference and the attributes schema when one of them uses it.
func (contract Contract) ValidationSources() []string {
	sources := map[string]bool{SourcePrefix + RegistryPath: true}
	for _, declared := range contract.ValidatedStructs() {
		sources[declared.Source] = true
		for _, field := range declared.Fields {
			if enum, found := contract.EnumByName(field.Ref); field.Kind == KindEnum && found {
				sources[enum.Source] = true
			}
			if field.Kind == KindAttributes {
				sources[contract.Attributes.Source] = true
			}
		}
	}
	sorted := make([]string, 0, len(sources))
	for source := range sources {
		sorted = append(sorted, source)
	}
	sort.Strings(sorted)
	return sorted
}
