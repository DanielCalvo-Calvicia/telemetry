package uuidgen_test

import (
	"context"
	"errors"
	"regexp"
	"testing"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
	uuidgen "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/uuidgen"
)

var uuid_v4_pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func Test_new_id_returns_a_well_formed_uuid_v4(t *testing.T) {
	new_id, err := uuidgen.NewIDGenerator().NewID(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !uuid_v4_pattern.MatchString(new_id) {
		t.Errorf("NewID() = %q, want a UUID v4", new_id)
	}
}

func Test_new_id_returns_a_different_id_each_time(t *testing.T) {
	id_generator := uuidgen.NewIDGenerator()
	seen_ids := map[string]bool{}

	for range 1000 {
		new_id, err := id_generator.NewID(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if seen_ids[new_id] {
			t.Fatalf("id %q was generated twice", new_id)
		}
		seen_ids[new_id] = true
	}
}

func Test_a_generation_failure_is_translated_and_keeps_its_cause(t *testing.T) {
	random_source_failure := errors.New("random source unavailable")
	id_generator := uuidgen.NewIDGeneratorWithFunction(func() (string, error) {
		return "", random_source_failure
	})

	new_id, err := id_generator.NewID(context.Background())

	if new_id != "" {
		t.Errorf("NewID() = %q, want no identifier on failure (no fake fallback)", new_id)
	}
	if !errors.Is(err, application.ErrIDGenerationFailed) {
		t.Errorf("error = %v, want ErrIDGenerationFailed", err)
	}
	if !errors.Is(err, random_source_failure) {
		t.Errorf("error = %v, want the root cause kept", err)
	}
}

func Test_a_custom_function_provides_the_ids(t *testing.T) {
	id_generator := uuidgen.NewIDGeneratorWithFunction(func() (string, error) { return "fixed-id", nil })

	if new_id, err := id_generator.NewID(context.Background()); err != nil || new_id != "fixed-id" {
		t.Errorf("NewID() = %q, %v", new_id, err)
	}
}
