package uuid_generator_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/uuid_generator"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/uuid_generator/helpers"
)

func Test_the_helper_lays_out_the_uuid_as_computed_by_hand(t *testing.T) {
	cases := []struct {
		name         string
		milliseconds int64
		random_bytes [helpers.RandomByteCount]byte
		want         string
	}{
		{"all random bits zero", 0x018f12345678, [10]byte{}, "018f1234-5678-7000-8000-000000000000"},
		{"all random bits one", 0x018f12345678, [10]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, "018f1234-5678-7fff-bfff-ffffffffffff"},
		{"mixed random bytes", 0x018f12345678, [10]byte{0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd}, "018f1234-5678-7bcd-af01-23456789abcd"},
		{"time zero", 0, [10]byte{}, "00000000-0000-7000-8000-000000000000"},
		{"only 48 bits of time are kept", 0x1_0000_0000_0001, [10]byte{}, "00000000-0001-7000-8000-000000000000"},
		{"a negative number keeps its low 48 bits", -1, [10]byte{}, "ffffffff-ffff-7000-8000-000000000000"},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			if got := helpers.LayoutUUIDv7(test_case.milliseconds, test_case.random_bytes); got != test_case.want {
				t.Fatalf("got %s, want %s", got, test_case.want)
			}
			if validation_error := value_objects.ValidateEventID(test_case.want); validation_error != nil {
				t.Fatalf("the expected text is not a valid event id: %v", validation_error)
			}
		})
	}
}

func Test_a_generated_id_is_a_lowercase_uuid_of_version_7_with_the_rfc_variant(t *testing.T) {
	generator := uuid_generator.NewUUIDGenerator()
	for index := 0; index < 200; index++ {
		event_id, generate_error := generator.NewEventID(context.Background())
		if generate_error != nil {
			t.Fatal(generate_error)
		}
		text := event_id.String()
		if text != strings.ToLower(text) || len(text) != 36 {
			t.Fatalf("not a lowercase hyphenated uuid: %s", text)
		}
		if text[14] != '7' {
			t.Fatalf("the version nibble is %c in %s", text[14], text)
		}
		if !strings.ContainsRune("89ab", rune(text[19])) {
			t.Fatalf("the variant nibble is %c in %s", text[19], text)
		}
		if _, id_error := value_objects.NewEventID(text); id_error != nil {
			t.Fatalf("the event id is not accepted: %v", id_error)
		}
	}
}

func Test_twenty_thousand_ids_are_unique_and_the_time_prefix_never_goes_back(t *testing.T) {
	milliseconds := int64(1_700_000_000_000)
	counter := 0
	generator := uuid_generator.NewUUIDGeneratorWithFunctions(
		func() int64 {
			counter++
			if counter%7 == 0 {
				milliseconds++
			}
			return milliseconds
		},
		func(buffer []byte) error {
			for index := range buffer {
				buffer[index] = byte(counter*31 + index*17 + counter/256)
			}
			return nil
		},
	)
	seen := map[string]bool{}
	previous_prefix := ""
	for index := 0; index < 20000; index++ {
		event_id, generate_error := generator.NewEventID(context.Background())
		if generate_error != nil {
			t.Fatal(generate_error)
		}
		text := event_id.String()
		if seen[text] {
			t.Fatalf("duplicate id %s", text)
		}
		seen[text] = true
		prefix := text[:13]
		if prefix < previous_prefix {
			t.Fatalf("the time prefix went back: %s after %s", prefix, previous_prefix)
		}
		previous_prefix = prefix
	}
}

func Test_the_time_prefix_does_not_go_back_when_the_clock_does(t *testing.T) {
	readings := []int64{2000, 1000, 3000}
	index := 0
	generator := uuid_generator.NewUUIDGeneratorWithFunctions(
		func() int64 {
			reading := readings[index]
			index++
			return reading
		},
		func(buffer []byte) error { return nil },
	)
	want_prefixes := []string{"00000000-07d0", "00000000-07d0", "00000000-0bb8"}
	for _, want_prefix := range want_prefixes {
		event_id, generate_error := generator.NewEventID(context.Background())
		if generate_error != nil {
			t.Fatal(generate_error)
		}
		if got := event_id.String()[:13]; got != want_prefix {
			t.Fatalf("got %s, want %s", got, want_prefix)
		}
	}
}

func Test_unique_ids_with_concurrent_callers(t *testing.T) {
	generator := uuid_generator.NewUUIDGenerator()
	var mutex sync.Mutex
	seen := map[string]bool{}
	var wait_group sync.WaitGroup
	for goroutine := 0; goroutine < 8; goroutine++ {
		wait_group.Add(1)
		go func() {
			defer wait_group.Done()
			for index := 0; index < 500; index++ {
				event_id, generate_error := generator.NewEventID(context.Background())
				if generate_error != nil {
					t.Error(generate_error)
					return
				}
				mutex.Lock()
				if seen[event_id.String()] {
					t.Errorf("duplicate id %s", event_id)
				}
				seen[event_id.String()] = true
				mutex.Unlock()
			}
		}()
	}
	wait_group.Wait()
}

func Test_a_failure_of_the_random_source_is_translated_and_keeps_the_cause(t *testing.T) {
	random_failure := errors.New("the random source is down")
	generator := uuid_generator.NewUUIDGeneratorWithFunctions(
		func() int64 { return 1 },
		func(buffer []byte) error { return random_failure },
	)

	_, event_error := generator.NewEventID(context.Background())
	_, instance_error := generator.NewInstanceID(context.Background())
	for _, generate_error := range []error{event_error, instance_error} {
		if !errors.Is(generate_error, application.ErrIDGenerationFailed) || !errors.Is(generate_error, random_failure) {
			t.Fatalf("expected ErrIDGenerationFailed joined with the cause, got %v", generate_error)
		}
		if code := value_objects.CodeOf(generate_error); code != value_objects.ErrorCodeEventBuildFailed {
			t.Fatalf("expected the code event_build_failed, got %q", code)
		}
	}
}

func Test_an_instance_id_is_the_text_of_a_uuid_of_version_7_that_an_identifier_accepts(t *testing.T) {
	instance_id, generate_error := uuid_generator.NewUUIDGenerator().NewInstanceID(context.Background())
	if generate_error != nil {
		t.Fatal(generate_error)
	}
	if validation_error := base_value_objects.ValidateIdentifier(instance_id.String()); validation_error != nil {
		t.Fatal(validation_error)
	}
	if validation_error := value_objects.ValidateEventID(instance_id.String()); validation_error != nil {
		t.Fatalf("the instance id is not a uuid of version 7: %v", validation_error)
	}
}
