package sdk_test

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/inbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/inbound/sdk"
)

// call is one call that the recording logger got.
type call struct {
	method string
	entry  dtos.LogEntryInput
}

type recording_logger struct {
	mutex sync.Mutex
	calls []call
}

var _ inbound.LoggerPort = (*recording_logger)(nil)

func (logger *recording_logger) record(method string, entry dtos.LogEntryInput) {
	logger.mutex.Lock()
	defer logger.mutex.Unlock()
	logger.calls = append(logger.calls, call{method: method, entry: entry})
}

func (logger *recording_logger) Trace(_ context.Context, entry dtos.LogEntryInput) {
	logger.record("TRACE", entry)
}
func (logger *recording_logger) Debug(_ context.Context, entry dtos.LogEntryInput) {
	logger.record("DEBUG", entry)
}
func (logger *recording_logger) Info(_ context.Context, entry dtos.LogEntryInput) {
	logger.record("INFO", entry)
}
func (logger *recording_logger) Warn(_ context.Context, entry dtos.LogEntryInput) {
	logger.record("WARN", entry)
}
func (logger *recording_logger) Error(_ context.Context, entry dtos.LogEntryInput) {
	logger.record("ERROR", entry)
}
func (logger *recording_logger) Critical(_ context.Context, entry dtos.LogEntryInput) {
	logger.record("CRITICAL", entry)
}

type panicking_logger struct{ recording_logger }

func (logger *panicking_logger) Info(_ context.Context, _ dtos.LogEntryInput) {
	panic("the logger failed")
}

type value_logger struct{}

func (value_logger) LogValue() slog.Value {
	return slog.GroupValue(slog.Int("a", 1), slog.String("b", "x"))
}

type panicking_valuer struct{}

func (panicking_valuer) LogValue() slog.Value { panic("a valuer that fails") }

func record_of(level slog.Level, message string, attrs ...slog.Attr) slog.Record {
	record := slog.NewRecord(time.Now(), level, message, 0)
	record.AddAttrs(attrs...)
	return record
}

func handle(t *testing.T, handler slog.Handler, record slog.Record) {
	t.Helper()
	if handle_error := handler.Handle(context.Background(), record); handle_error != nil {
		t.Fatalf("Handle returned %v", handle_error)
	}
}

func Test_the_level_maps_to_the_severity(t *testing.T) {
	rows := []struct {
		level slog.Level
		want  string
	}{
		{-100, "TRACE"}, {-8, "TRACE"}, {-5, "TRACE"}, {-4, "DEBUG"}, {-1, "DEBUG"}, {0, "INFO"}, {3, "INFO"},
		{4, "WARN"}, {7, "WARN"}, {8, "ERROR"}, {11, "ERROR"}, {12, "CRITICAL"}, {1000, "CRITICAL"},
	}
	for _, row := range rows {
		logger := &recording_logger{}
		handle(t, sdk.NewSlogHandler(logger, "app"), record_of(row.level, "m"))
		if len(logger.calls) != 1 || logger.calls[0].method != row.want {
			t.Errorf("level %d: %v, want %s", row.level, logger.calls, row.want)
		}
	}
}

func Test_enabled_is_always_true(t *testing.T) {
	handler := sdk.NewSlogHandler(&recording_logger{}, "app")
	for _, level := range []slog.Level{-100, -4, 0, 4, 8, 100} {
		if !handler.Enabled(context.Background(), level) {
			t.Errorf("level %d is not enabled", level)
		}
	}
}

func Test_the_message_the_default_category_and_the_attributes(t *testing.T) {
	logger := &recording_logger{}
	handle(t, sdk.NewSlogHandler(logger, "app"), record_of(slog.LevelInfo, "hello", slog.String("user", "bob"), slog.Int("n", 3)))

	want := dtos.LogEntryInput{Category: "app", Message: "hello", Fields: map[string]any{"user": "bob", "n": int64(3)}}
	if !reflect.DeepEqual(logger.calls[0].entry, want) {
		t.Fatalf("entry %+v, want %+v", logger.calls[0].entry, want)
	}
}

func Test_category_and_function_attributes_are_not_fields(t *testing.T) {
	logger := &recording_logger{}
	handle(t, sdk.NewSlogHandler(logger, "app"),
		record_of(slog.LevelWarn, "m", slog.String("category", "storage"), slog.String("function", "check"), slog.Bool("ok", true)))

	want := dtos.LogEntryInput{Category: "storage", Function: "check", Message: "m", Fields: map[string]any{"ok": true}}
	if !reflect.DeepEqual(logger.calls[0].entry, want) {
		t.Fatalf("entry %+v, want %+v", logger.calls[0].entry, want)
	}
}

func Test_a_category_that_is_not_a_string_is_a_field(t *testing.T) {
	logger := &recording_logger{}
	handle(t, sdk.NewSlogHandler(logger, "app"), record_of(slog.LevelInfo, "m", slog.Int("category", 5), slog.Int("function", 6)))

	entry := logger.calls[0].entry
	if entry.Category != "app" || entry.Function != "" || !reflect.DeepEqual(entry.Fields, map[string]any{"category": int64(5), "function": int64(6)}) {
		t.Fatalf("entry %+v", entry)
	}
}

func Test_a_group_is_flattened_with_dots_and_a_valuer_is_resolved(t *testing.T) {
	logger := &recording_logger{}
	handle(t, sdk.NewSlogHandler(logger, "app"), record_of(slog.LevelInfo, "m",
		slog.Group("req", slog.String("id", "7"), slog.Group("inner", slog.Int("n", 1))),
		slog.Group("", slog.String("inline", "x")),
		slog.Group("empty"),
		slog.Any("valuer", value_logger{}),
		slog.Attr{},
	))

	want := map[string]any{"req.id": "7", "req.inner.n": int64(1), "inline": "x", "valuer.a": int64(1), "valuer.b": "x"}
	if !reflect.DeepEqual(logger.calls[0].entry.Fields, want) {
		t.Fatalf("fields %v, want %v", logger.calls[0].entry.Fields, want)
	}
}

func Test_the_kinds_of_a_slog_value_map_to_their_go_value(t *testing.T) {
	logger := &recording_logger{}
	instant := time.Date(2026, 10, 28, 12, 0, 0, 0, time.FixedZone("x", 7200))
	handle(t, sdk.NewSlogHandler(logger, "app"), record_of(slog.LevelInfo, "m",
		slog.Uint64("u", 5), slog.Float64("f", 0.5), slog.Duration("d", time.Second), slog.Time("t", instant),
		slog.Any("err", errors.New("boom")), slog.Any("list", []string{"a"}), slog.Any("map", map[string]any{"a": 1}),
	))

	want := map[string]any{
		"u": uint64(5), "f": 0.5, "d": time.Second, "t": instant.UTC(),
		"err": errors.New("boom"), "list": []string{"a"}, "map": map[string]any{"a": 1},
	}
	if !reflect.DeepEqual(logger.calls[0].entry.Fields, want) {
		t.Fatalf("fields %#v, want %#v", logger.calls[0].entry.Fields, want)
	}
}

func Test_with_attrs_adds_attributes_and_does_not_change_the_original(t *testing.T) {
	logger := &recording_logger{}
	original := sdk.NewSlogHandler(logger, "app")
	derived := original.WithAttrs([]slog.Attr{slog.String("service", "hub"), slog.String("category", "boot")})
	again := derived.WithAttrs([]slog.Attr{slog.String("extra", "x")})

	handle(t, original, record_of(slog.LevelInfo, "one"))
	handle(t, derived, record_of(slog.LevelInfo, "two", slog.String("own", "y")))
	handle(t, again, record_of(slog.LevelInfo, "three"))

	got := []dtos.LogEntryInput{logger.calls[0].entry, logger.calls[1].entry, logger.calls[2].entry}
	want := []dtos.LogEntryInput{
		{Category: "app", Message: "one"},
		{Category: "boot", Message: "two", Fields: map[string]any{"service": "hub", "own": "y"}},
		{Category: "boot", Message: "three", Fields: map[string]any{"service": "hub", "extra": "x"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries\n got %+v\nwant %+v", got, want)
	}
}

func Test_with_group_prefixes_the_attributes_added_after_it_and_does_not_change_the_original(t *testing.T) {
	logger := &recording_logger{}
	original := sdk.NewSlogHandler(logger, "app")
	before := original.WithAttrs([]slog.Attr{slog.String("a", "1")})
	grouped := before.WithGroup("g").WithAttrs([]slog.Attr{slog.String("b", "2")})
	nested := grouped.WithGroup("h")

	handle(t, original, record_of(slog.LevelInfo, "o", slog.String("x", "0")))
	handle(t, grouped, record_of(slog.LevelInfo, "g", slog.String("c", "3"), slog.String("category", "ignored-as-field")))
	handle(t, nested, record_of(slog.LevelInfo, "n", slog.String("d", "4")))

	if want := map[string]any{"x": "0"}; !reflect.DeepEqual(logger.calls[0].entry.Fields, want) {
		t.Errorf("original %v, want %v", logger.calls[0].entry.Fields, want)
	}
	want_grouped := map[string]any{"a": "1", "g.b": "2", "g.c": "3", "g.category": "ignored-as-field"}
	if !reflect.DeepEqual(logger.calls[1].entry.Fields, want_grouped) {
		t.Errorf("grouped %v, want %v", logger.calls[1].entry.Fields, want_grouped)
	}
	if logger.calls[1].entry.Category != "app" {
		t.Errorf("a category under a group is a field, not the category: %q", logger.calls[1].entry.Category)
	}
	want_nested := map[string]any{"a": "1", "g.b": "2", "g.h.d": "4"}
	if !reflect.DeepEqual(logger.calls[2].entry.Fields, want_nested) {
		t.Errorf("nested %v, want %v", logger.calls[2].entry.Fields, want_nested)
	}
}

func Test_with_group_with_an_empty_name_is_the_same_handler(t *testing.T) {
	handler := sdk.NewSlogHandler(&recording_logger{}, "app")
	if handler.WithGroup("") != slog.Handler(handler) {
		t.Fatal("an empty group name must return the handler itself")
	}
}

func Test_handle_never_panics_and_returns_nil_for_strange_values(t *testing.T) {
	logger := &recording_logger{}
	handler := sdk.NewSlogHandler(logger, "app")
	type unmarshalable struct{ Channel chan int }

	handle(t, handler, record_of(slog.LevelInfo, "strange",
		slog.Any("nil", nil),
		slog.Any("func", func() {}),
		slog.Any("chan", make(chan int)),
		slog.Any("bad", unmarshalable{Channel: make(chan int)}),
		slog.Any("typed_nil", (*unmarshalable)(nil)),
		slog.Any("panicking", panicking_valuer{}),
	))

	if len(logger.calls) != 1 {
		t.Fatalf("%d calls", len(logger.calls))
	}
	fields := logger.calls[0].entry.Fields
	if _, has_nil := fields["nil"]; !has_nil {
		t.Errorf("the nil Any is passed to the logger, which drops it: %v", fields)
	}
}

func Test_a_panicking_logger_does_not_panic_the_handler(t *testing.T) {
	handle(t, sdk.NewSlogHandler(&panicking_logger{}, "app"), record_of(slog.LevelInfo, "m"))
}

func Test_a_nil_logger_and_a_nil_handler_do_nothing(t *testing.T) {
	handle(t, sdk.NewSlogHandler(nil, "app"), record_of(slog.LevelInfo, "m"))
	var nil_handler *sdk.SlogHandler
	if handle_error := nil_handler.Handle(context.Background(), record_of(slog.LevelInfo, "m")); handle_error != nil {
		t.Fatal(handle_error)
	}
}

func Test_one_handler_serves_16_goroutines(t *testing.T) {
	logger := &recording_logger{}
	handler := sdk.NewSlogHandler(logger, "app").WithAttrs([]slog.Attr{slog.String("shared", "x")})
	var group sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			derived := handler.WithGroup("w").WithAttrs([]slog.Attr{slog.Int("worker", worker)})
			for iteration := 0; iteration < 50; iteration++ {
				_ = handler.Handle(context.Background(), record_of(slog.LevelInfo, "m", slog.Int("i", iteration)))
				_ = derived.Handle(context.Background(), record_of(slog.LevelInfo, "m", slog.Int("i", iteration)))
			}
		}()
	}
	group.Wait()
	if len(logger.calls) != 1600 {
		t.Fatalf("%d calls, want 1600", len(logger.calls))
	}
}

func Test_the_handler_works_under_slog_logger(t *testing.T) {
	logger := &recording_logger{}
	slog.New(sdk.NewSlogHandler(logger, "app")).With("k", "v").WithGroup("g").Error("failed", "code", 7)

	want := dtos.LogEntryInput{Category: "app", Message: "failed", Fields: map[string]any{"k": "v", "g.code": int64(7)}}
	if logger.calls[0].method != "ERROR" || !reflect.DeepEqual(logger.calls[0].entry, want) {
		t.Fatalf("%v", logger.calls)
	}
}
