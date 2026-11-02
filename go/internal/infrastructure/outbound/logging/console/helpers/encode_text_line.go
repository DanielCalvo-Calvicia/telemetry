package helpers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// ENCODE TEXT LINE
// =============================================================================

// severity_width is the width of the severity column: the longest severity, CRITICAL, has eight characters.
const severity_width = 8

// EncodeTextLine returns the readable line of an event and a line feed: the time, the severity padded to eight
// characters, the body and the attributes. A log entry shows its category in brackets, its function when it has one and
// its message; any other event shows its type and the canonical JSON of its data. The attributes follow as key=value, the
// keys in the order of the canonical form and the values in canonical JSON. The line never holds more than the masked
// event holds: no resource and no id. A control character of a text is escaped (see EscapeText). An attribute whose value
// has no canonical JSON is returned as the error.
func EncodeTextLine(event value_objects.Envelope) ([]byte, error) {
	var line strings.Builder
	line.WriteString(event.OccurredAt)
	line.WriteString(" ")
	line.WriteString(string(event.Severity))
	for padding := len(string(event.Severity)); padding < severity_width; padding++ {
		line.WriteString(" ")
	}

	if event.EventType == value_objects.EventTypeLogEntryRecorded {
		line.WriteString(" [" + EscapeText(text_of(event.Data["category"])) + "]")
		if function_name := text_of(event.Data["function"]); function_name != "" {
			line.WriteString(" " + EscapeText(function_name) + ":")
		}
		line.WriteString(" " + EscapeText(text_of(event.Data["message"])))
	} else {
		data := event.Data
		if data == nil {
			data = map[string]any{}
		}
		data_json, data_error := canonical_json.CanonicalJSON(data)
		if data_error != nil {
			return nil, data_error
		}
		line.WriteString(" " + EscapeText(event.EventType) + " " + string(data_json))
	}

	keys := make([]string, 0, len(event.Attributes))
	for key := range event.Attributes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left int, right int) bool { return canonical_json.KeyLess(keys[left], keys[right]) })
	for _, key := range keys {
		value_json, value_error := canonical_json.CanonicalJSON(event.Attributes[key])
		if value_error != nil {
			return nil, value_error
		}
		line.WriteString(" " + EscapeText(key) + "=" + string(value_json))
	}

	line.WriteString("\n")
	return []byte(line.String()), nil
}

// EscapeText makes a text safe for one line: a line feed becomes \n, a carriage return \r, a tab \t, and any other
// character below U+0020 and U+007F becomes \u and four lower case hexadecimal digits. Every other character, the ones
// outside ASCII included, is kept.
func EscapeText(text string) string {
	var escaped strings.Builder
	for _, character := range text {
		switch {
		case character == '\n':
			escaped.WriteString(`\n`)
		case character == '\r':
			escaped.WriteString(`\r`)
		case character == '\t':
			escaped.WriteString(`\t`)
		case character < 0x20 || character == 0x7f:
			escaped.WriteString(fmt.Sprintf(`\u%04x`, character))
		default:
			escaped.WriteRune(character)
		}
	}
	return escaped.String()
}

// ----- HELPERS --------------------------------------------------------------

// text_of returns the value when it is a text, and an empty text for anything else, including a missing value.
func text_of(value any) string {
	text, _ := value.(string)
	return text
}
