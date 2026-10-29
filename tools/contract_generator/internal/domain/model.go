// Package domain holds the model of a contract: the enums, the event types, the error codes and the structs that the
// generator turns into code. It imports nothing from the other layers.
package domain

import "sort"

// SourcePrefix is put before the path of a contract file, relative to the contracts folder, to name it in a header or
// in a message. It is a fixed text, so no machine path ever reaches the output.
const SourcePrefix = "contracts/"

// FailureCategoryEnum is the name of the enum whose values are the categories an error code may have.
const FailureCategoryEnum = "failure_category"

// FieldKind is the kind of the value of a field of a struct.
type FieldKind int

// The kinds of field the generator knows.
const (
	// KindString is a JSON string, also an instant in UTC (the utc_time definition).
	KindString FieldKind = iota
	// KindNumber is a JSON number.
	KindNumber
	// KindInteger is a JSON integer.
	KindInteger
	// KindBoolean is a JSON boolean.
	KindBoolean
	// KindEnum is a reference to an enum; Ref names it.
	KindEnum
	// KindStruct is a reference to another struct; Ref names it.
	KindStruct
	// KindAttributes is a reference to the attributes schema: a map of scalars and lists of scalars.
	KindAttributes
	// KindMap is an object with no declared property: a map of any value.
	KindMap
)

// Enum is a closed set of string values, read from one JSON file of contracts/enums.
type Enum struct {
	// Name is the base name of the file in snake_case (severity).
	Name string
	// Source is the path of the file (contracts/enums/severity.json).
	Source string
	// Values are the values in the order of the file.
	Values []string
}

// EventType is an entry of the registry of event types.
type EventType struct {
	// Name is the type in the notation domain.entity.action.
	Name string
	// Schema is the path of its data schema, relative to the contracts folder, as the registry writes it.
	Schema string
	// Source is the path of the registry.
	Source string
}

// ErrorCode is an entry of the catalog of error codes.
type ErrorCode struct {
	// Code is the stable code in lowercase snake_case.
	Code string
	// Category is the failure category, a value of the enum failure_category.
	Category string
	// DefaultMessage is the message of the reference text.
	DefaultMessage string
	// Source is the path of the catalog.
	Source string
}

// Field is a property of a struct.
type Field struct {
	// JSONName is the name in the schema, in snake_case.
	JSONName string
	// Kind is the kind of the value.
	Kind FieldKind
	// Ref names the enum or the struct a KindEnum or KindStruct field points to, in snake_case.
	Ref string
	// Required is true when the schema lists the property as required.
	Required bool
	// MinLength is the minimum length of a string, in characters; 0 means no minimum.
	MinLength int
	// MaxLength is the maximum length of a string, in characters; 0 means no maximum.
	MaxLength int
	// Unsupported lists, comma separated, the keywords of the property that the generated validation cannot honour
	// (pattern, format, an inline enum...). It stops the generator only when the struct is validated.
	Unsupported string
}

// Struct is an object schema: the envelope, a common object or the data of an event type.
type Struct struct {
	// Name is the name in snake_case words (resource, log_entry_recorded).
	Name string
	// Source is the path of the schema file.
	Source string
	// Fields are the properties in the order of the schema.
	Fields []Field
	// Closed is true when the schema has additionalProperties false: a field it does not name is invalid.
	Closed bool
	// Unsupported lists, comma separated, the keywords of the schema that the generated validation cannot honour.
	Unsupported string
}

// OrderedFields returns the fields with the required ones first and then the optional ones, each group in
// alphabetical order, so the output does not depend on the order of the file and a Python dataclass can give
// defaults to the last ones.
func (declared Struct) OrderedFields() []Field {
	ordered := make([]Field, len(declared.Fields))
	copy(ordered, declared.Fields)
	sort.SliceStable(ordered, func(left int, right int) bool {
		if ordered[left].Required != ordered[right].Required {
			return ordered[left].Required
		}
		return ordered[left].JSONName < ordered[right].JSONName
	})
	return ordered
}

// AttributesLimits are the limits of the attributes schema that the generated validation applies. A zero means no
// limit.
type AttributesLimits struct {
	// Source is the path of the attributes schema; it is empty when the contract has none.
	Source string
	// MaxProperties is the most keys an attributes object may have.
	MaxProperties int
	// MinKeyLength is the shortest key, in characters.
	MinKeyLength int
	// MaxKeyLength is the longest key, in characters.
	MaxKeyLength int
	// MaxStringLength is the longest string value, in characters.
	MaxStringLength int
}

// Contract is the whole contract folder, already read.
type Contract struct {
	// Enums are the enums of contracts/enums.
	Enums []Enum
	// EventTypes are the registered event types.
	EventTypes []EventType
	// ErrorCodes are the codes of the catalog.
	ErrorCodes []ErrorCode
	// Structs are the envelope, the common objects and the data of every registered event type.
	Structs []Struct
	// DataSchemas are the paths, relative to the contracts folder, of every event data schema found in the folder
	// (schemas/<domain>/<entity>_<action>.schema.json), registered or not.
	DataSchemas []string
	// Attributes are the limits of the attributes schema.
	Attributes AttributesLimits
}

// Sorted returns a copy of the contract with every list in a deterministic order: the enums, the event types, the
// error codes and the structs by name, and the data schemas by path. The values of an enum keep the order of their
// file because that order is part of the contract.
func (contract Contract) Sorted() Contract {
	sorted := Contract{
		Enums:       append([]Enum(nil), contract.Enums...),
		EventTypes:  append([]EventType(nil), contract.EventTypes...),
		ErrorCodes:  append([]ErrorCode(nil), contract.ErrorCodes...),
		Structs:     append([]Struct(nil), contract.Structs...),
		DataSchemas: append([]string(nil), contract.DataSchemas...),
		Attributes:  contract.Attributes,
	}
	sort.SliceStable(sorted.Enums, func(left int, right int) bool { return sorted.Enums[left].Name < sorted.Enums[right].Name })
	sort.SliceStable(sorted.EventTypes, func(left int, right int) bool {
		return sorted.EventTypes[left].Name < sorted.EventTypes[right].Name
	})
	sort.SliceStable(sorted.ErrorCodes, func(left int, right int) bool {
		return sorted.ErrorCodes[left].Code < sorted.ErrorCodes[right].Code
	})
	sort.SliceStable(sorted.Structs, func(left int, right int) bool {
		return PascalCase(sorted.Structs[left].Name) < PascalCase(sorted.Structs[right].Name)
	})
	sort.Strings(sorted.DataSchemas)
	return sorted
}

// GeneratedFile is a file the generator writes.
type GeneratedFile struct {
	// Name is the file name, without a folder.
	Name string
	// Content is the exact content of the file.
	Content []byte
}

// The paths, relative to the contracts folder, of the two YAML files of the contract.
const (
	// RegistryPath is the registry of event types.
	RegistryPath = "events/registry.yaml"
	// CatalogPath is the catalog of error codes.
	CatalogPath = "errors/catalog.yaml"
)
