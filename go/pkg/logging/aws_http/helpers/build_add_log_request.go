package helpers

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// BUILD ADD_LOG REQUEST
// =============================================================================

// log_event_prefix is what an event type starts with when it is a log event.
const log_event_prefix = "log."

// IsLogEvent tells whether the type of an event starts with "log." (log.entry.recorded and any other log.* type).
// logger.x is not a log event.
func IsLogEvent(event_type string) bool {
	return strings.HasPrefix(event_type, log_event_prefix)
}

// BuildAddLogRequest returns the exact body of the add_log endpoint for one log event: the JSON object with the keys
// log_group_name, log_stream_name, message and timestamp in that order, no space, no HTML escaping and the UTF-8 of the
// text kept. The message is the text of BuildMessage. The timestamp is the occurred_at in milliseconds since the epoch,
// cut and never rounded. It returns an error for an occurred_at that is not a time and for a text that is not valid
// UTF-8; the error never holds a value of the event.
func BuildAddLogRequest(event value_objects.Envelope, log_group_name string, log_stream_name string) ([]byte, error) {
	occurred_at, parse_error := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if parse_error != nil {
		return nil, errors.New("occurred_at is not a time")
	}
	message, message_error := BuildMessage(event)
	if message_error != nil {
		return nil, message_error
	}
	var body bytes.Buffer
	body.WriteString(`{"log_group_name":`)
	if write_error := write_json_text(&body, log_group_name); write_error != nil {
		return nil, write_error
	}
	body.WriteString(`,"log_stream_name":`)
	if write_error := write_json_text(&body, log_stream_name); write_error != nil {
		return nil, write_error
	}
	body.WriteString(`,"message":`)
	if write_error := write_json_text(&body, string(message)); write_error != nil {
		return nil, write_error
	}
	body.WriteString(`,"timestamp":`)
	body.WriteString(strconv.FormatInt(occurred_at.UnixMilli(), 10))
	body.WriteByte('}')
	return body.Bytes(), nil
}

// BuildMessage returns the text that goes in the message of the request: the entry of the old add_log contract, with the
// keys in the order ProducerID, ApplicationID, ProcessID, Application, Category, Function, Indentation, Depth, Level,
// Message, Fields, Timestamp. ProducerID and Application are the service name, ApplicationID the instance id, ProcessID
// the correlation id or empty, Category, Function and Message come from the data of the event (empty when missing),
// Indentation and Depth are 0 (the event has no call tree), Level is MapLevel, Fields is the canonical JSON of the
// attributes or null when there are none, and Timestamp is the occurred_at text as it is.
func BuildMessage(event value_objects.Envelope) ([]byte, error) {
	process_id := ""
	if event.Context != nil {
		process_id = event.Context.CorrelationId
	}
	fields := []byte("null")
	if len(event.Attributes) > 0 {
		canonical_fields, fields_error := canonical_json.CanonicalJSON(event.Attributes)
		if fields_error != nil {
			return nil, errors.New("the attributes have no JSON form")
		}
		fields = canonical_fields
	}

	var message bytes.Buffer
	message.WriteByte('{')
	text_members := []struct {
		key  string
		text string
	}{
		{"ProducerID", event.Resource.ServiceName},
		{"ApplicationID", event.Resource.InstanceId},
		{"ProcessID", process_id},
		{"Application", event.Resource.ServiceName},
		{"Category", data_text(event, "category")},
		{"Function", data_text(event, "function")},
	}
	for _, member := range text_members {
		message.WriteString(`"` + member.key + `":`)
		if write_error := write_json_text(&message, member.text); write_error != nil {
			return nil, write_error
		}
		message.WriteByte(',')
	}
	message.WriteString(`"Indentation":0,"Depth":0,"Level":`)
	// The level, the message and the timestamp are written after the fixed numbers, in the order of the old entry.
	if write_error := write_json_text(&message, MapLevel(event.Severity)); write_error != nil {
		return nil, write_error
	}
	message.WriteString(`,"Message":`)
	if write_error := write_json_text(&message, data_text(event, "message")); write_error != nil {
		return nil, write_error
	}
	message.WriteString(`,"Fields":`)
	message.Write(fields)
	message.WriteString(`,"Timestamp":`)
	if write_error := write_json_text(&message, event.OccurredAt); write_error != nil {
		return nil, write_error
	}
	message.WriteByte('}')
	return message.Bytes(), nil
}

// ----- HELPERS --------------------------------------------------------------

// data_text returns the text under a key of the data of the event, or an empty text when the key is missing or is not a
// text.
func data_text(event value_objects.Envelope, key string) string {
	text, _ := event.Data[key].(string)
	return text
}

// write_json_text writes a text as a JSON string by the same rule as the canonical JSON of the core: only the quotation
// mark, the reverse solidus and the control characters below U+0020 are escaped; everything else (the HTML characters
// and all UTF-8 included) is written as it is. It returns an error for a text that is not valid UTF-8.
func write_json_text(buffer *bytes.Buffer, text string) error {
	if !utf8.ValidString(text) {
		return errors.New("a text is not valid UTF-8")
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
