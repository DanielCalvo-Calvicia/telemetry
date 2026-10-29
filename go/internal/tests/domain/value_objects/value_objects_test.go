package value_objects_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

func TestNewSeverityAcceptsEveryValueOfTheGeneratedEnum(test *testing.T) {
	values := value_objects.SeverityValues()
	if len(values) != 6 {
		test.Fatalf("the contract has six severities, got %d", len(values))
	}
	for _, value := range values {
		severity, construction_error := value_objects.NewSeverity(string(value))
		if construction_error != nil || severity != value {
			test.Errorf("%s: got %q, %v", value, severity, construction_error)
		}
	}
	if _, construction_error := value_objects.NewSeverity("info"); !errors.Is(construction_error, domain.ErrSeverityInvalid) {
		test.Errorf("got %v", construction_error)
	}
}

func TestNewActorKindAcceptsEveryValueOfTheGeneratedEnum(test *testing.T) {
	for _, value := range value_objects.ActorKindValues() {
		actor_kind, construction_error := value_objects.NewActorKind(string(value))
		if construction_error != nil || actor_kind != value {
			test.Errorf("%s: got %q, %v", value, actor_kind, construction_error)
		}
	}
	if _, construction_error := value_objects.NewActorKind("robot"); !errors.Is(construction_error, domain.ErrActorKindInvalid) {
		test.Errorf("got %v", construction_error)
	}
}

func TestAnInvalidEventIDReturnsTheSentinelItself(test *testing.T) {
	for _, text := range []string{"550e8400-e29b-41d4-a716-446655440000", "", "a-b-c-d", "018f3c1e-7b2a-7c3d-9e4f-0123456789ab-00"} {
		_, construction_error := value_objects.NewEventID(text)
		if construction_error != domain.ErrEventIDInvalid {
			test.Errorf("%q: got %v, want the sentinel unwrapped", text, construction_error)
		}
	}
	event_id, construction_error := value_objects.NewEventID("018f3c1e-7b2a-7c3d-bbbb-0123456789ab")
	if construction_error != nil || event_id.String() != "018f3c1e-7b2a-7c3d-bbbb-0123456789ab" {
		test.Errorf("variant b: got %v, %v", event_id, construction_error)
	}
}

func TestAUTCInstantTruncatesToMicrosecondsAndKeepsTrailingZeros(test *testing.T) {
	instant, construction_error := value_objects.NewUTCInstant(time.Date(2026, 10, 28, 10, 0, 0, 123_456_999, time.UTC))
	if construction_error != nil {
		test.Fatal(construction_error)
	}
	if instant.String() != "2026-10-28T10:00:00.123456Z" || instant.Time().Nanosecond() != 123_456_000 {
		test.Errorf("got %s / %d", instant.String(), instant.Time().Nanosecond())
	}
	round, _ := value_objects.NewUTCInstant(time.Date(5, 1, 2, 3, 4, 5, 0, time.UTC))
	if round.String() != "0005-01-02T03:04:05.000000Z" {
		test.Errorf("got %s", round.String())
	}
	if _, offset := instant.Time().Zone(); offset != 0 {
		test.Errorf("the time is not in UTC: %d", offset)
	}
}

func TestAUTCInstantDropsTheMonotonicReading(test *testing.T) {
	instant, construction_error := value_objects.NewUTCInstant(time.Now().UTC())
	if construction_error != nil {
		test.Fatal(construction_error)
	}
	if instant.Time().String() != instant.Time().Round(0).String() {
		test.Errorf("the monotonic reading was kept: %s", instant.Time())
	}
}

func TestAnInstantOutsideUTCOrTheYearsIsRejectedWithErrNonUTCTime(test *testing.T) {
	for _, instant := range []time.Time{
		time.Date(2026, 10, 28, 12, 0, 0, 0, time.FixedZone("CEST", 2*60*60)),
		time.Date(0, 12, 31, 0, 0, 0, 0, time.UTC),
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		_, construction_error := value_objects.NewUTCInstant(instant)
		if !errors.Is(construction_error, domain.ErrNonUTCTime) {
			test.Errorf("%v: got %v", instant, construction_error)
		}
	}
}

func TestAResourceExposesItsFieldsAndTellsAbsentFromPresent(test *testing.T) {
	resource, construction_error := value_objects.NewResourceIdentity("sensor-hub", "hub-01",
		value_objects.WithEnvironment("staging"), value_objects.WithTimeZone("Europe/Madrid"))
	if construction_error != nil {
		test.Fatal(construction_error)
	}
	if resource.ServiceName() != "sensor-hub" || resource.InstanceID() != "hub-01" {
		test.Errorf("got %q %q", resource.ServiceName(), resource.InstanceID())
	}
	if environment, is_present := resource.Environment(); !is_present || environment != "staging" {
		test.Errorf("environment: %q %v", environment, is_present)
	}
	if time_zone, is_present := resource.TimeZone(); !is_present || time_zone != "Europe/Madrid" {
		test.Errorf("time zone: %q %v", time_zone, is_present)
	}
	if _, is_present := resource.ServiceVersion(); is_present {
		test.Error("the service version was not given")
	}
	if _, is_present := resource.Host(); is_present {
		test.Error("the host was not given")
	}
	full, _ := value_objects.NewResourceIdentity("s", "i", value_objects.WithServiceVersion("1.0"), value_objects.WithHost("h"))
	if service_version, is_present := full.ServiceVersion(); !is_present || service_version != "1.0" {
		test.Errorf("service version: %q %v", service_version, is_present)
	}
	if host, is_present := full.Host(); !is_present || host != "h" {
		test.Errorf("host: %q %v", host, is_present)
	}
}

func TestAResourceReportsEveryInvalidFieldWithItsSentinelAndTheCause(test *testing.T) {
	_, construction_error := value_objects.NewResourceIdentity("", "\t",
		value_objects.WithEnvironment(""), value_objects.WithServiceVersion(" "), value_objects.WithHost("a\nb"),
		value_objects.WithTimeZone("Europe Madrid"))
	for _, sentinel := range []error{
		domain.ErrResourceServiceNameInvalid, domain.ErrResourceInstanceIDInvalid, domain.ErrResourceEnvironmentInvalid,
		domain.ErrResourceServiceVersionInvalid, domain.ErrResourceHostInvalid, domain.ErrTimeZoneInvalid,
		base_value_objects.ErrIdentifierBlank, base_value_objects.ErrIdentifierHasControlCharacters,
	} {
		if !errors.Is(construction_error, sentinel) {
			test.Errorf("%v is missing from %v", sentinel, construction_error)
		}
	}
	if _, time_zone_error := value_objects.NewResourceIdentity("s", "i", value_objects.WithTimeZone("")); !errors.Is(time_zone_error, domain.ErrTimeZoneInvalid) {
		test.Errorf("a present but empty time zone: %v", time_zone_error)
	}
}

func TestAnActorExposesItsFieldsAndReportsItsErrors(test *testing.T) {
	actor, construction_error := value_objects.NewActorIdentity(value_objects.ActorKindAgent, "planner",
		value_objects.WithActorType("llm"), value_objects.WithActorVersion("2"))
	if construction_error != nil {
		test.Fatal(construction_error)
	}
	actor_type, has_type := actor.Type()
	actor_version, has_version := actor.Version()
	if actor.Kind() != value_objects.ActorKindAgent || actor.ID() != "planner" || !has_type || actor_type != "llm" || !has_version || actor_version != "2" {
		test.Errorf("got %v", actor)
	}
	bare, _ := value_objects.NewActorIdentity(value_objects.ActorKindDevice, "d")
	if _, has_type := bare.Type(); has_type {
		test.Error("the type was not given")
	}
	if _, has_version := bare.Version(); has_version {
		test.Error("the version was not given")
	}
	_, construction_error = value_objects.NewActorIdentity("robot", "", value_objects.WithActorType(""), value_objects.WithActorVersion(""))
	for _, sentinel := range []error{
		domain.ErrActorKindInvalid, domain.ErrActorIDInvalid, domain.ErrActorTypeInvalid, domain.ErrActorVersionInvalid,
		base_value_objects.ErrIdentifierBlank,
	} {
		if !errors.Is(construction_error, sentinel) {
			test.Errorf("%v is missing from %v", sentinel, construction_error)
		}
	}
}

func TestACorrelationContextExposesItsFieldsAndTheEmptyOneIsValid(test *testing.T) {
	empty, construction_error := value_objects.NewCorrelationContext()
	if construction_error != nil {
		test.Fatal(construction_error)
	}
	if _, is_present := empty.TraceID(); is_present {
		test.Error("the empty context has a trace id")
	}
	if _, is_present := empty.SpanID(); is_present {
		test.Error("the empty context has a span id")
	}
	if _, is_present := empty.ExecutionID(); is_present {
		test.Error("the empty context has an execution id")
	}
	if _, is_present := empty.CorrelationID(); is_present {
		test.Error("the empty context has a correlation id")
	}
	full, construction_error := value_objects.NewCorrelationContext(
		value_objects.WithTraceID("0af7651916cd43dd8448eb211c80319c"), value_objects.WithSpanID("b7ad6b7169203331"),
		value_objects.WithExecutionID("run-7"), value_objects.WithCorrelationID("order-9"))
	if construction_error != nil {
		test.Fatal(construction_error)
	}
	trace_id, _ := full.TraceID()
	span_id, _ := full.SpanID()
	execution_id, _ := full.ExecutionID()
	correlation_id, _ := full.CorrelationID()
	if trace_id != "0af7651916cd43dd8448eb211c80319c" || span_id != "b7ad6b7169203331" || execution_id != "run-7" || correlation_id != "order-9" {
		test.Errorf("got %q %q %q %q", trace_id, span_id, execution_id, correlation_id)
	}
}

func TestACorrelationContextReportsEveryInvalidFieldWithItsSentinelAndTheCause(test *testing.T) {
	_, construction_error := value_objects.NewCorrelationContext(
		value_objects.WithTraceID("ABC"), value_objects.WithSpanID(""), value_objects.WithExecutionID(""), value_objects.WithCorrelationID("\t"))
	for _, sentinel := range []error{
		domain.ErrContextTraceIDInvalid, domain.ErrContextSpanIDInvalid, domain.ErrContextExecutionIDInvalid,
		domain.ErrContextCorrelationIDInvalid, base_value_objects.ErrHexDigitsInvalid, base_value_objects.ErrHexDigitsWrongWidth,
		base_value_objects.ErrIdentifierBlank,
	} {
		if !errors.Is(construction_error, sentinel) {
			test.Errorf("%v is missing from %v", sentinel, construction_error)
		}
	}
}

func TestEventAttributesKeepsTheKindOfEveryScalarAndCopiesLists(test *testing.T) {
	attributes, construction_error := value_objects.NewEventAttributes(map[string]any{
		"text": "a", "flag": true, "int": 3, "int8": int8(1), "uint16": uint16(2), "int64": int64(4), "uint": uint(5),
		"float": 0.5, "float32": float32(1.5), "strings": []string{"a", "b"}, "ints": []int{1, 2}, "any": []any{"a", 1, false},
		"empty": []any{}, "nil_list": []string(nil),
	})
	if construction_error != nil {
		test.Fatal(construction_error)
	}
	if attributes.Len() != 14 {
		test.Errorf("got %d attributes", attributes.Len())
	}
	if value, exists := attributes.Get("strings"); !exists || !reflect.DeepEqual(value, []any{"a", "b"}) {
		test.Errorf("strings: %#v", value)
	}
	if value, exists := attributes.Get("float32"); !exists || value != float32(1.5) {
		test.Errorf("float32: %#v", value)
	}
	if _, exists := attributes.Get("missing"); exists {
		test.Error("a key that does not exist was found")
	}
}

func TestEventAttributesIsImmutable(test *testing.T) {
	given_list := []any{"a", "b"}
	given := map[string]any{"name": "x", "tags": given_list}
	attributes, construction_error := value_objects.NewEventAttributes(given)
	if construction_error != nil {
		test.Fatal(construction_error)
	}

	given["name"] = "changed"
	given["added"] = 1
	given_list[0] = "changed"
	if value, _ := attributes.Get("name"); value != "x" || attributes.Len() != 2 {
		test.Errorf("the map that was passed in changed the value: %v", attributes.Entries())
	}
	if value, _ := attributes.Get("tags"); !reflect.DeepEqual(value, []any{"a", "b"}) {
		test.Errorf("the list that was passed in changed the value: %v", value)
	}

	returned := attributes.Entries()
	returned["name"] = "changed"
	delete(returned, "tags")
	if value, _ := attributes.Get("name"); value != "x" || attributes.Len() != 2 {
		test.Errorf("the map of Entries changed the value: %v", attributes.Entries())
	}
	returned_list, _ := attributes.Get("tags")
	returned_list.([]any)[1] = "changed"
	if value, _ := attributes.Get("tags"); !reflect.DeepEqual(value, []any{"a", "b"}) {
		test.Errorf("the list of Get changed the value: %v", value)
	}
	entries_list := attributes.Entries()["tags"].([]any)
	entries_list[0] = "changed"
	if value, _ := attributes.Get("tags"); !reflect.DeepEqual(value, []any{"a", "b"}) {
		test.Errorf("the list of Entries changed the value: %v", value)
	}
}

func TestANilMapIsTheEmptyAttributes(test *testing.T) {
	attributes, construction_error := value_objects.NewEventAttributes(nil)
	if construction_error != nil || attributes.Len() != 0 || len(attributes.Entries()) != 0 {
		test.Errorf("got %v, %v", attributes, construction_error)
	}
	zero := value_objects.EventAttributes{}
	if zero.Len() != 0 || len(zero.Entries()) != 0 {
		test.Error("the zero value is not empty")
	}
}

func TestEventAttributesReportsEveryBrokenEntryWithItsSentinel(test *testing.T) {
	number := 1
	_, construction_error := value_objects.NewEventAttributes(map[string]any{
		"":            "a",
		"nested":      map[string]any{"a": 1},
		"nil":         nil,
		"nested_list": [][]any{{1}},
		"pointer":     &number,
		"array":       [2]int{1, 2},
	})
	if !errors.Is(construction_error, domain.ErrAttributeKeyInvalid) || !errors.Is(construction_error, domain.ErrAttributeValueInvalid) {
		test.Errorf("got %v", construction_error)
	}
	for _, rejected := range []any{map[string]any{}, []map[string]any{{"a": 1}}, []any{nil}, []any{[]any{1}}, struct{}{}, time.Now(), func() {}} {
		validation_error := value_objects.ValidateEventAttributes(map[string]any{"key": rejected})
		if !errors.Is(validation_error, domain.ErrAttributeValueInvalid) {
			test.Errorf("%#v: got %v", rejected, validation_error)
		}
	}
}

func TestEveryValueObjectIsReturnedByValueWithNoSetter(test *testing.T) {
	for _, value_object := range []any{
		value_objects.EventID{}, value_objects.UTCInstant{}, value_objects.ResourceIdentity{},
		value_objects.ActorIdentity{}, value_objects.CorrelationContext{}, value_objects.EventAttributes{},
	} {
		value_type := reflect.TypeOf(value_object)
		if value_type.Kind() != reflect.Struct {
			test.Errorf("%s is not a struct", value_type)
		}
		for position := 0; position < value_type.NumField(); position++ {
			if value_type.Field(position).IsExported() {
				test.Errorf("%s exports the field %s", value_type, value_type.Field(position).Name)
			}
		}
		pointer_type := reflect.PointerTo(value_type)
		for position := 0; position < pointer_type.NumMethod(); position++ {
			method_name := pointer_type.Method(position).Name
			if len(method_name) >= 3 && method_name[:3] == "Set" {
				test.Errorf("%s has the setter %s", value_type, method_name)
			}
		}
	}
}

func TestTheSeveritiesAreListedOnce(test *testing.T) {
	values := value_objects.SeverityValues()
	if len(slices.Compact(slices.Sorted(slices.Values(values)))) != len(values) {
		test.Errorf("repeated severity in %v", values)
	}
}
