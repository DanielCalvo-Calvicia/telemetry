package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/event_validation"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

const (
	test_wall_time = "2026-10-28T10:00:00.123456Z"
	test_event_id  = "01900000-0000-7000-8000-000000000001"
)

func permissive_validator(_ string, _ any) error { return nil }

func test_resource(test *testing.T) value_objects.ResourceIdentity {
	test.Helper()
	return resource_from(test, building_resource{ServiceName: "sensor-hub", InstanceID: "hub-01"})
}

type test_services struct {
	clock         *fakes.FakeClock
	id_generator  *fakes.FakeIDGenerator
	configuration *fakes.FakeConfiguration
	service       *services.EventBuilderService
}

func new_test_services(test *testing.T, validate_data services.EventDataValidator) test_services {
	test.Helper()
	event_id, id_error := value_objects.NewEventID(test_event_id)
	if id_error != nil {
		test.Fatal(id_error)
	}
	fake_clock := fakes.NewFakeClock(utc_from_text(test, test_wall_time))
	fake_id_generator := fakes.NewFakeIDGenerator(event_id)
	fake_configuration := fakes.NewFakeConfiguration(dtos.Configuration{Resource: test_resource(test)})
	return test_services{
		clock:         fake_clock,
		id_generator:  fake_id_generator,
		configuration: fake_configuration,
		service:       services.NewEventBuilderService(fake_clock, fake_id_generator, fake_configuration, validate_data),
	}
}

func log_input(data map[string]any) dtos.NewEventInput {
	return dtos.NewEventInput{EventType: "log.entry.recorded", Severity: value_objects.SeverityInfo, Data: data}
}

func valid_log_data() map[string]any {
	return map[string]any{"category": "http", "message": "started"}
}

func canonical_text(test *testing.T, envelope value_objects.Envelope) string {
	test.Helper()
	bytes, serialize_error := canonical_json.CanonicalJSON(envelope)
	if serialize_error != nil {
		test.Fatal(serialize_error)
	}
	return string(bytes)
}

func TestTheSameInputWithFreshIdenticalFakesGivesIdenticalBytes(test *testing.T) {
	first, first_error := new_test_services(test, event_validation.ValidateEvent).service.Build(context.Background(), log_input(valid_log_data()))
	second, second_error := new_test_services(test, event_validation.ValidateEvent).service.Build(context.Background(), log_input(valid_log_data()))
	if first_error != nil || second_error != nil {
		test.Fatal(first_error, second_error)
	}
	if canonical_text(test, first) != canonical_text(test, second) {
		test.Fatal("the same input gave different bytes")
	}
}

func TestRecordedAtIsTheClockEvenWhenOccurredAtIsGivenAndDifferent(test *testing.T) {
	built_services := new_test_services(test, event_validation.ValidateEvent)
	given := utc_from_text(test, "2026-01-02T03:04:05.000001Z")
	input := log_input(valid_log_data())
	input.OccurredAt = &given
	built, build_error := built_services.service.Build(context.Background(), input)
	if build_error != nil {
		test.Fatal(build_error)
	}
	if built.OccurredAt != "2026-01-02T03:04:05.000001Z" || built.RecordedAt != test_wall_time {
		test.Fatalf("occurred_at %s, recorded_at %s", built.OccurredAt, built.RecordedAt)
	}
}

func TestObservedAtInDataSetsOccurredAtAndOverridesAGivenOne(test *testing.T) {
	built_services := new_test_services(test, permissive_validator)
	given := utc_from_text(test, "2026-01-02T03:04:05.000001Z")
	data := valid_log_data()
	data["observed_at"] = "2026-05-06T07:08:09.654321Z"
	input := log_input(data)
	input.OccurredAt = &given
	built, build_error := built_services.service.Build(context.Background(), input)
	if build_error != nil {
		test.Fatal(build_error)
	}
	if built.OccurredAt != "2026-05-06T07:08:09.654321Z" || built.RecordedAt != test_wall_time {
		test.Fatalf("occurred_at %s, recorded_at %s", built.OccurredAt, built.RecordedAt)
	}
}

func TestAnInvalidObservedAtIsAnEventSchemaInvalidThatNeverShowsTheValue(test *testing.T) {
	for _, observed_value := range []any{42, "2026-05-06T07:08:09+00:00", "secret-value-123", nil} {
		built_services := new_test_services(test, permissive_validator)
		data := valid_log_data()
		data["observed_at"] = observed_value
		_, build_error := built_services.service.Build(context.Background(), log_input(data))
		var classified *domain.ClassifiedError
		if !errors.As(build_error, &classified) || classified.Code != value_objects.ErrorCodeEventSchemaInvalid {
			test.Fatalf("%v: got %v", observed_value, build_error)
		}
		if !errors.Is(build_error, domain.ErrEventSchemaInvalid) || !strings.Contains(build_error.Error(), "data.observed_at") {
			test.Errorf("the message must name data.observed_at: %v", build_error)
		}
		if strings.Contains(build_error.Error(), "secret-value-123") || strings.Contains(build_error.Error(), "2026-05-06") {
			test.Errorf("the message shows the value: %v", build_error)
		}
	}
}

func TestAnIDGeneratorFailureIsAnEventBuildFailed(test *testing.T) {
	built_services := new_test_services(test, permissive_validator)
	cause := errors.New("no ids")
	built_services.id_generator.FailWith(cause)
	built, build_error := built_services.service.Build(context.Background(), log_input(valid_log_data()))
	if !errors.Is(build_error, application.ErrEventBuildFailed) || !errors.Is(build_error, cause) {
		test.Fatalf("got %v", build_error)
	}
	if canonical_text(test, built) != canonical_text(test, value_objects.Envelope{}) {
		test.Error("an error must come with an empty envelope")
	}
	if _, instance_error := built_services.id_generator.NewInstanceID(context.Background()); !errors.Is(instance_error, cause) {
		test.Errorf("NewInstanceID must fail too: %v", instance_error)
	}
	built_services.id_generator.FailWith(nil)
	if _, retry_error := built_services.service.Build(context.Background(), log_input(valid_log_data())); retry_error != nil {
		test.Errorf("the fake must work again: %v", retry_error)
	}
}

func TestAConfigurationFailureIsAnEventBuildFailed(test *testing.T) {
	built_services := new_test_services(test, permissive_validator)
	cause := errors.New("no configuration")
	built_services.configuration.FailWith(cause)
	_, build_error := built_services.service.Build(context.Background(), log_input(valid_log_data()))
	if !errors.Is(build_error, application.ErrEventBuildFailed) || !errors.Is(build_error, cause) {
		test.Fatalf("got %v", build_error)
	}
}

// panicking_port panics in every method that the builder service uses.
type panicking_port struct{ panics_in string }

func (port panicking_port) Now(_ context.Context) value_objects.UTCInstant {
	if port.panics_in == "clock" {
		panic("clock exploded")
	}
	return value_objects.UTCInstant{}
}

func (port panicking_port) MonotonicNanoseconds(_ context.Context) int64 { return 0 }

func (port panicking_port) ClockInfo(_ context.Context) dtos.ClockInfo { return dtos.ClockInfo{} }

func (port panicking_port) NewEventID(_ context.Context) (value_objects.EventID, error) {
	panic("id generator exploded: secret-data")
}

func (port panicking_port) NewInstanceID(_ context.Context) (base_value_objects.Identifier, error) {
	panic("not used")
}

func (port panicking_port) Configuration(_ context.Context) (dtos.Configuration, error) {
	panic("configuration exploded")
}

func TestAPanickingPortOrValidatorBecomesAnEventBuildFailed(test *testing.T) {
	good := new_test_services(test, permissive_validator)
	cases := map[string]*services.EventBuilderService{
		"clock":         services.NewEventBuilderService(panicking_port{panics_in: "clock"}, good.id_generator, good.configuration, permissive_validator),
		"id generator":  services.NewEventBuilderService(good.clock, panicking_port{}, good.configuration, permissive_validator),
		"configuration": services.NewEventBuilderService(good.clock, good.id_generator, panicking_port{}, permissive_validator),
		"validator":     services.NewEventBuilderService(good.clock, good.id_generator, good.configuration, func(string, any) error { panic("validator exploded") }),
		"nil validator": services.NewEventBuilderService(good.clock, good.id_generator, good.configuration, nil),
	}
	for name, service := range cases {
		built, build_error := service.Build(context.Background(), log_input(valid_log_data()))
		if !errors.Is(build_error, application.ErrEventBuildFailed) || !errors.Is(build_error, application.ErrEventBuildPanicked) {
			test.Errorf("%s: got %v", name, build_error)
			continue
		}
		if strings.Contains(build_error.Error(), "secret-data") || built.EventId != "" {
			test.Errorf("%s: the panic value leaked or the envelope is not empty: %v", name, build_error)
		}
	}
}

func TestANilDataGivesAnEmptyNonNilDataThatSerializesAsBraces(test *testing.T) {
	built_services := new_test_services(test, permissive_validator)
	built, build_error := built_services.service.Build(context.Background(), log_input(nil))
	if build_error != nil {
		test.Fatal(build_error)
	}
	if built.Data == nil || len(built.Data) != 0 || !strings.Contains(canonical_text(test, built), `"data":{}`) {
		test.Fatalf("data must be an empty object: %#v", built.Data)
	}
}

func TestTheCallersMapChangedAfterBuildDoesNotChangeTheEnvelope(test *testing.T) {
	built_services := new_test_services(test, permissive_validator)
	data := valid_log_data()
	built, build_error := built_services.service.Build(context.Background(), log_input(data))
	if build_error != nil {
		test.Fatal(build_error)
	}
	before := canonical_text(test, built)
	data["message"] = "changed"
	data["added"] = true
	if canonical_text(test, built) != before {
		test.Fatal("the envelope shares the map of the caller")
	}
}

func TestTheClockIsReadOnceForTheTimeAndOnceForItsInformation(test *testing.T) {
	built_services := new_test_services(test, permissive_validator)
	if _, build_error := built_services.service.Build(context.Background(), log_input(valid_log_data())); build_error != nil {
		test.Fatal(build_error)
	}
	now_calls, _, clock_info_calls := built_services.clock.Calls()
	if now_calls != 1 || clock_info_calls != 1 {
		test.Fatalf("Now %d times, ClockInfo %d times", now_calls, clock_info_calls)
	}
}

func TestAttributesAreMergedWithTheClockKeysAndTheClockWins(test *testing.T) {
	built_services := new_test_services(test, permissive_validator)
	built_services.clock.SetClockInfo(dtos.ClockInfo{Source: "ntp", SourceKnown: true, Synced: true, SyncedKnown: true})
	attributes, attributes_error := value_objects.NewEventAttributes(map[string]any{"clock_synced": false, "order": 7})
	if attributes_error != nil {
		test.Fatal(attributes_error)
	}
	input := log_input(valid_log_data())
	input.Attributes = &attributes
	built, build_error := built_services.service.Build(context.Background(), input)
	if build_error != nil {
		test.Fatal(build_error)
	}
	if built.Attributes["clock_source"] != "ntp" || built.Attributes["clock_synced"] != true || built.Attributes["order"] != 7 || len(built.Attributes) != 3 {
		test.Fatalf("got %v", built.Attributes)
	}
	if value, _ := attributes.Get("clock_synced"); value != false {
		test.Error("the attributes of the caller were changed")
	}
}

func TestNoAttributesAndAnUnknownClockGiveAnEnvelopeWithoutAttributesActorOrContext(test *testing.T) {
	built_services := new_test_services(test, permissive_validator)
	empty_context, context_error := value_objects.NewCorrelationContext()
	if context_error != nil {
		test.Fatal(context_error)
	}
	input := log_input(valid_log_data())
	input.Context = &empty_context
	built, build_error := built_services.service.Build(context.Background(), input)
	if build_error != nil {
		test.Fatal(build_error)
	}
	if built.Attributes != nil || built.Actor != nil || built.Context != nil {
		test.Fatalf("attributes %v, actor %v, context %v", built.Attributes, built.Actor, built.Context)
	}
	if built.SchemaVersion != domain.SchemaVersion {
		test.Errorf("schema_version %s", built.SchemaVersion)
	}
}
