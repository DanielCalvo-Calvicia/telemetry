package value_objects_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

func Test_new_level(t *testing.T) {
	test_cases := []struct {
		name           string
		raw_level      string
		expected_level value_objects.Level
		expected_error error
	}{
		{"trace", "TRACE", value_objects.LevelTrace, nil},
		{"debug", "DEBUG", value_objects.LevelDebug, nil},
		{"info", "INFO", value_objects.LevelInfo, nil},
		{"warn", "WARN", value_objects.LevelWarn, nil},
		{"error", "ERROR", value_objects.LevelError, nil},
		{"fatal", "FATAL", value_objects.LevelFatal, nil},
		{"lower case is not accepted", "info", "", value_objects.ErrInvalidLevel},
		{"unknown", "VERBOSE", "", value_objects.ErrInvalidLevel},
		{"empty", "", "", value_objects.ErrInvalidLevel},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			level, err := value_objects.NewLevel(test_case.raw_level)
			if !errors.Is(err, test_case.expected_error) {
				t.Fatalf("NewLevel() error = %v, want %v", err, test_case.expected_error)
			}
			if level != test_case.expected_level {
				t.Errorf("NewLevel() = %q, want %q", level, test_case.expected_level)
			}
			if level.IsValid() != (test_case.expected_error == nil) {
				t.Errorf("IsValid() = %v", level.IsValid())
			}
		})
	}
}

func Test_level_string_and_validity(t *testing.T) {
	if value_objects.LevelWarn.String() != "WARN" {
		t.Errorf("String() = %q", value_objects.LevelWarn.String())
	}
	if value_objects.Level("nope").IsValid() {
		t.Error("an unknown level must not be valid")
	}
}

func Test_is_at_least(t *testing.T) {
	test_cases := []struct {
		name     string
		level    value_objects.Level
		minimum  value_objects.Level
		expected bool
	}{
		{"same level", value_objects.LevelInfo, value_objects.LevelInfo, true},
		{"more severe", value_objects.LevelError, value_objects.LevelInfo, true},
		{"less severe", value_objects.LevelDebug, value_objects.LevelInfo, false},
		{"trace is the lowest", value_objects.LevelTrace, value_objects.LevelTrace, true},
		{"fatal is the highest", value_objects.LevelFatal, value_objects.LevelFatal, true},
		{"unknown level", "NOPE", value_objects.LevelTrace, false},
		{"unknown minimum", value_objects.LevelFatal, "NOPE", false},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			if test_case.level.IsAtLeast(test_case.minimum) != test_case.expected {
				t.Errorf("%q.IsAtLeast(%q) = %v, want %v", test_case.level, test_case.minimum, !test_case.expected, test_case.expected)
			}
		})
	}
}

func Test_minimum_level_for_an_environment(t *testing.T) {
	test_cases := []struct {
		name           string
		environment    string
		expected_level value_objects.Level
		expected_error error
	}{
		{"development", "development", value_objects.LevelDebug, nil},
		{"production", "production", value_objects.LevelInfo, nil},
		{"unknown", "staging", "", value_objects.ErrInvalidEnvironment},
		{"empty", "", "", value_objects.ErrInvalidEnvironment},
		{"case matters", "Production", "", value_objects.ErrInvalidEnvironment},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			minimum_level, err := value_objects.MinimumLevelFor(test_case.environment)
			if !errors.Is(err, test_case.expected_error) || minimum_level != test_case.expected_level {
				t.Errorf("MinimumLevelFor(%q) = %q, %v; want %q, %v", test_case.environment, minimum_level, err, test_case.expected_level, test_case.expected_error)
			}
		})
	}
}
