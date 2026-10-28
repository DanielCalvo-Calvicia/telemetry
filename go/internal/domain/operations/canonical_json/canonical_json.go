// Package canonical_json serializes a value to the canonical JSON of RFC 8785 (JSON Canonicalization Scheme, JCS):
// the keys of an object sorted by UTF-16 code units, no insignificant whitespace, strings escaped by the rule of the
// RFC, numbers in the ECMAScript form and UTF-8 output. The same logical event gives the same bytes in Go and in
// Python (REQ-CNF-003, REQ-CTR-004). It uses the standard library only and does no I/O (REQ-ARC-005).
package canonical_json

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

var (
	time_type           = reflect.TypeOf(time.Time{})
	json_number_type    = reflect.TypeOf(json.Number(""))
	json_marshaler_type = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	text_marshaler_type = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

// CanonicalJSON returns the canonical JSON (RFC 8785) of a value. It accepts the generated value_objects.Envelope, any
// struct with json tags (the names, "-" and omitempty are honored like encoding/json does), maps with string keys,
// slices, strings, booleans, integers, floats, json.Number and nil. A time.Time is written as the string of
// FormatUTCTime. It returns a classified error (see domain.ClassifiedError) for NaN, an infinity, an integer beyond
// +-MaxExactInteger, a string that is not valid UTF-8, a non-UTC time and a value that has no JSON form.
func CanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	if write_error := write_value(&buffer, reflect.ValueOf(value)); write_error != nil {
		return nil, write_error
	}
	return buffer.Bytes(), nil
}

func unsupported(detail string) error {
	return domain.NewClassifiedError(value_objects.ErrorCodeEventSchemaInvalid, domain.ErrUnsupportedValue, errors.New(detail))
}

func write_value(buffer *bytes.Buffer, value reflect.Value) error {
	if !value.IsValid() {
		buffer.WriteString("null")
		return nil
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			buffer.WriteString("null")
			return nil
		}
	}
	if value.Type() == time_type {
		text, time_error := FormatUTCTime(value.Interface().(time.Time))
		if time_error != nil {
			return time_error
		}
		return write_string(buffer, text)
	}
	if value.Type() == json_number_type {
		return write_json_number(buffer, value.String())
	}
	if value.Kind() != reflect.Interface && (value.Type().Implements(json_marshaler_type) || value.Type().Implements(text_marshaler_type)) {
		return write_marshaled(buffer, value)
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		return write_value(buffer, value.Elem())
	case reflect.Bool:
		buffer.WriteString(strconv.FormatBool(value.Bool()))
	case reflect.String:
		return write_string(buffer, value.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return write_integer(buffer, value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if value.Uint() > uint64(MaxExactInteger) {
			return domain.NewClassifiedError(
				value_objects.ErrorCodeLimitExceeded, domain.ErrNumberOutOfRange,
				errors.New("the integer "+strconv.FormatUint(value.Uint(), 10)+" is beyond the exact range"))
		}
		return write_integer(buffer, int64(value.Uint()))
	case reflect.Float32:
		// The shortest digits of the 32-bit value, not of its widened 64-bit value.
		narrow, _ := strconv.ParseFloat(strconv.FormatFloat(value.Float(), 'g', -1, 32), 64)
		return write_float(buffer, narrow)
	case reflect.Float64:
		return write_float(buffer, value.Float())
	case reflect.Slice, reflect.Array:
		return write_array(buffer, value)
	case reflect.Map:
		return write_map(buffer, value)
	case reflect.Struct:
		return write_struct(buffer, value)
	default:
		return unsupported("the type " + value.Type().String() + " has no JSON form")
	}
	return nil
}

// write_marshaled handles a type that has its own JSON form: it asks encoding/json for it and writes that form again
// in the canonical way.
func write_marshaled(buffer *bytes.Buffer, value reflect.Value) error {
	marshaled, marshal_error := json.Marshal(value.Interface())
	if marshal_error != nil {
		return domain.NewClassifiedError(value_objects.ErrorCodeEventSchemaInvalid, domain.ErrUnsupportedValue, marshal_error)
	}
	decoder := json.NewDecoder(bytes.NewReader(marshaled))
	decoder.UseNumber()
	var decoded any
	if decode_error := decoder.Decode(&decoded); decode_error != nil {
		return domain.NewClassifiedError(value_objects.ErrorCodeEventSchemaInvalid, domain.ErrUnsupportedValue, decode_error)
	}
	return write_value(buffer, reflect.ValueOf(decoded))
}

func write_integer(buffer *bytes.Buffer, number int64) error {
	text, format_error := FormatInt(number)
	if format_error != nil {
		return format_error
	}
	buffer.WriteString(text)
	return nil
}

func write_float(buffer *bytes.Buffer, number float64) error {
	text, format_error := FormatFloat(number)
	if format_error != nil {
		return format_error
	}
	buffer.WriteString(text)
	return nil
}

// write_json_number writes a number that keeps its text (json.Number): an integer text is an integer, any other is a
// double.
func write_json_number(buffer *bytes.Buffer, text string) error {
	if !strings.ContainsAny(text, ".eE") {
		integer, parse_error := strconv.ParseInt(text, 10, 64)
		if parse_error != nil {
			return domain.NewClassifiedError(
				value_objects.ErrorCodeLimitExceeded, domain.ErrNumberOutOfRange, parse_error)
		}
		return write_integer(buffer, integer)
	}
	number, parse_error := strconv.ParseFloat(text, 64)
	if parse_error != nil {
		return domain.NewClassifiedError(value_objects.ErrorCodeEventSchemaInvalid, domain.ErrNumberOutOfRange, parse_error)
	}
	return write_float(buffer, number)
}

func write_array(buffer *bytes.Buffer, value reflect.Value) error {
	if value.Kind() == reflect.Slice {
		if value.IsNil() {
			buffer.WriteString("null")
			return nil
		}
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return unsupported("a byte slice has no canonical JSON form")
		}
	}
	buffer.WriteByte('[')
	for position := 0; position < value.Len(); position++ {
		if position > 0 {
			buffer.WriteByte(',')
		}
		if element_error := write_value(buffer, value.Index(position)); element_error != nil {
			return element_error
		}
	}
	buffer.WriteByte(']')
	return nil
}

// member is a key of an object with its already canonical value.
type member struct {
	key   string
	value []byte
}

func write_members(buffer *bytes.Buffer, members []member) error {
	for _, current := range members {
		if !utf8.ValidString(current.key) {
			return unsupported("a key is not valid UTF-8")
		}
	}
	sort.Slice(members, func(left int, right int) bool {
		return utf16_less(members[left].key, members[right].key)
	})
	buffer.WriteByte('{')
	for position, current := range members {
		if position > 0 {
			buffer.WriteByte(',')
		}
		if key_error := write_string(buffer, current.key); key_error != nil {
			return key_error
		}
		buffer.WriteByte(':')
		buffer.Write(current.value)
	}
	buffer.WriteByte('}')
	return nil
}

func write_map(buffer *bytes.Buffer, value reflect.Value) error {
	if value.IsNil() {
		buffer.WriteString("null")
		return nil
	}
	if value.Type().Key().Kind() != reflect.String {
		return unsupported("an object needs string keys")
	}
	members := make([]member, 0, value.Len())
	iterator := value.MapRange()
	for iterator.Next() {
		var member_buffer bytes.Buffer
		if member_error := write_value(&member_buffer, iterator.Value()); member_error != nil {
			return member_error
		}
		members = append(members, member{key: iterator.Key().String(), value: member_buffer.Bytes()})
	}
	return write_members(buffer, members)
}

func write_struct(buffer *bytes.Buffer, value reflect.Value) error {
	value_type := value.Type()
	members := make([]member, 0, value_type.NumField())
	for position := 0; position < value_type.NumField(); position++ {
		field := value_type.Field(position)
		if field.Anonymous {
			return unsupported("an embedded field has no canonical JSON form")
		}
		if !field.IsExported() {
			continue
		}
		tag_name, tag_options, _ := strings.Cut(field.Tag.Get("json"), ",")
		if tag_name == "-" && tag_options == "" {
			continue
		}
		if tag_name == "" {
			tag_name = field.Name
		}
		field_value := value.Field(position)
		if strings.Contains(","+tag_options+",", ",omitempty,") && is_empty_value(field_value) {
			continue
		}
		var member_buffer bytes.Buffer
		if member_error := write_value(&member_buffer, field_value); member_error != nil {
			return member_error
		}
		members = append(members, member{key: tag_name, value: member_buffer.Bytes()})
	}
	return write_members(buffer, members)
}

// is_empty_value is the omitempty rule of encoding/json.
func is_empty_value(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return value.IsNil()
	}
	return false
}

// utf16_less compares two keys by UTF-16 code units, as RFC 8785 section 3.2.3 requires. It differs from the order
// of code points (and of UTF-8 bytes) for the characters outside the Basic Multilingual Plane.
func utf16_less(left string, right string) bool {
	left_units := utf16.Encode([]rune(left))
	right_units := utf16.Encode([]rune(right))
	for position := 0; position < len(left_units) && position < len(right_units); position++ {
		if left_units[position] != right_units[position] {
			return left_units[position] < right_units[position]
		}
	}
	return len(left_units) < len(right_units)
}

// write_string writes a string by RFC 8785 section 3.2.2.2: only the quotation mark, the reverse solidus and the
// control characters below U+0020 are escaped; everything else is written as UTF-8.
func write_string(buffer *bytes.Buffer, text string) error {
	if !utf8.ValidString(text) {
		return unsupported("a string is not valid UTF-8")
	}
	const hexadecimal = "0123456789abcdef"
	buffer.WriteByte('"')
	for _, character := range text {
		switch {
		case character == '"':
			buffer.WriteString(`\"`)
		case character == '\\':
			buffer.WriteString(`\\`)
		case character == '\b':
			buffer.WriteString(`\b`)
		case character == '\t':
			buffer.WriteString(`\t`)
		case character == '\n':
			buffer.WriteString(`\n`)
		case character == '\f':
			buffer.WriteString(`\f`)
		case character == '\r':
			buffer.WriteString(`\r`)
		case character < 0x20:
			buffer.WriteString(`\u00`)
			buffer.WriteByte(hexadecimal[character>>4])
			buffer.WriteByte(hexadecimal[character&0xf])
		default:
			buffer.WriteRune(character)
		}
	}
	buffer.WriteByte('"')
	return nil
}
