// Package sdk holds the adapters that let a project's own logging call the core: today, a log/slog handler.
package sdk

import (
	"context"
	"log/slog"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/inbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// SLOG HANDLER
// =============================================================================

const (
	category_attribute_key = "category"
	function_attribute_key = "function"
)

// handler_field is one attribute of a record, flat: its key already has the prefix of the groups and its value is
// resolved.
type handler_field struct {
	key   string
	value slog.Value
}

// SlogHandler is a slog.Handler that turns each record into one log.entry.recorded event through a LoggerPort (REQ-LOG-002).
//
// Mapping: the severity comes from the level of the record (below -4 TRACE, -4 to -1 DEBUG, 0 to 3 INFO, 4 to 7 WARN,
// 8 to 11 ERROR, 12 and above CRITICAL); the message is the message of the record; an attribute named category (a
// string) sets the category and one named function (a string) sets the function, and neither is a field; every other
// attribute is a field (a group is flattened with the key group.key; a slog.LogValuer is resolved) and goes through the
// rule of the fields of the logger. Without a category attribute, the default category is used.
//
// Two choices that may change later: the handler does NOT read the source of the call (the program counter of the
// record), so there is no automatic function, and it does NOT use the time of the record: occurred_at is the time of the
// call, from the clock port of the core. A time attribute is written in UTC.
//
// The level is not filtered here (Enabled is always true): the filtering of the level belongs to T-04.2. The handler
// never panics and Handle always returns nil, because a log call never fails the caller (ADR-005). It holds no state
// that changes: WithAttrs and WithGroup return new handlers, so one handler serves any number of goroutines.
type SlogHandler struct {
	logger           inbound.LoggerPort
	default_category string
	fields           []handler_field
	group_prefix     string
}

var _ slog.Handler = (*SlogHandler)(nil)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewSlogHandler returns a SlogHandler that records through the logger and uses default_category for a record that has
// no category attribute.
func NewSlogHandler(logger inbound.LoggerPort, default_category string) *SlogHandler {
	return &SlogHandler{logger: logger, default_category: default_category}
}

// ----- METHODS --------------------------------------------------------------

// Enabled always returns true: the filtering by level belongs to T-04.2.
func (handler *SlogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle records the record through the logger. It never panics and always returns nil.
func (handler *SlogHandler) Handle(ctx context.Context, record slog.Record) error {
	defer func() {
		_ = recover()
	}()
	if handler == nil || handler.logger == nil {
		return nil
	}

	entry := dtos.LogEntryInput{Category: handler.default_category, Message: record.Message}
	fields := map[string]any{}
	apply_field := func(field handler_field) {
		if field.key == category_attribute_key && field.value.Kind() == slog.KindString {
			entry.Category = field.value.String()
			return
		}
		if field.key == function_attribute_key && field.value.Kind() == slog.KindString {
			entry.Function = field.value.String()
			return
		}
		fields[field.key] = field_value_of(field.value)
	}
	for _, field := range handler.fields {
		apply_field(field)
	}
	record.Attrs(func(attr slog.Attr) bool {
		for _, field := range flatten_attribute(handler.group_prefix, attr) {
			apply_field(field)
		}
		return true
	})
	if len(fields) > 0 {
		entry.Fields = fields
	}

	switch severity_of_level(record.Level) {
	case value_objects.SeverityTrace:
		handler.logger.Trace(ctx, entry)
	case value_objects.SeverityDebug:
		handler.logger.Debug(ctx, entry)
	case value_objects.SeverityInfo:
		handler.logger.Info(ctx, entry)
	case value_objects.SeverityWarn:
		handler.logger.Warn(ctx, entry)
	case value_objects.SeverityError:
		handler.logger.Error(ctx, entry)
	default:
		handler.logger.Critical(ctx, entry)
	}
	return nil
}

// WithAttrs returns a new handler that also has the attributes, under the group that is open now. The handler it is
// called on does not change.
func (handler *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	copied := *handler
	copied.fields = append([]handler_field(nil), handler.fields...)
	for _, attr := range attrs {
		copied.fields = append(copied.fields, flatten_attribute(handler.group_prefix, attr)...)
	}
	return &copied
}

// WithGroup returns a new handler in which the attributes added after it have the name of the group as a prefix
// (group.key). An empty name returns the handler as it is, as slog asks. The handler it is called on does not change.
func (handler *SlogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return handler
	}
	copied := *handler
	copied.group_prefix = handler.group_prefix + name + "."
	return &copied
}

// ----- HELPERS --------------------------------------------------------------

// severity_of_level maps a slog level to the severity of the contract.
func severity_of_level(level slog.Level) value_objects.Severity {
	switch {
	case level < slog.LevelDebug:
		return value_objects.SeverityTrace
	case level < slog.LevelInfo:
		return value_objects.SeverityDebug
	case level < slog.LevelWarn:
		return value_objects.SeverityInfo
	case level < slog.LevelError:
		return value_objects.SeverityWarn
	case level < slog.LevelError+4:
		return value_objects.SeverityError
	default:
		return value_objects.SeverityCritical
	}
}

// flatten_attribute resolves the attribute and returns its leaves, each with the prefix and, for a group, the key of the
// group. An empty attribute and an empty group have no leaves; a group with an empty key is inlined.
func flatten_attribute(prefix string, attr slog.Attr) []handler_field {
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		group_prefix := prefix
		if attr.Key != "" {
			group_prefix = prefix + attr.Key + "."
		}
		var leaves []handler_field
		for _, member := range value.Group() {
			leaves = append(leaves, flatten_attribute(group_prefix, member)...)
		}
		return leaves
	}
	if attr.Key == "" && value.Kind() == slog.KindAny && value.Any() == nil {
		return nil
	}
	return []handler_field{{key: prefix + attr.Key, value: value}}
}

// field_value_of is the Go value of a slog value, as a field of the logger. A time is written in UTC.
func field_value_of(value slog.Value) any {
	go_value := value.Any()
	if instant, is_time := go_value.(time.Time); is_time {
		return instant.UTC()
	}
	return go_value
}
