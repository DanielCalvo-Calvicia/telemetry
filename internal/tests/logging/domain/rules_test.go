package domain_test

import (
	"slices"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain"
)

func Test_valid_levels_are_ordered_from_least_to_most_severe(t *testing.T) {
	expected_levels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

	if !slices.Equal(domain.ValidLevels, expected_levels) {
		t.Errorf("ValidLevels = %v, want %v", domain.ValidLevels, expected_levels)
	}
}

func Test_level_constants(t *testing.T) {
	constant_levels := []string{domain.LevelTrace, domain.LevelDebug, domain.LevelInfo, domain.LevelWarn, domain.LevelError, domain.LevelFatal}

	if !slices.Equal(constant_levels, domain.ValidLevels) {
		t.Errorf("level constants = %v, want %v", constant_levels, domain.ValidLevels)
	}
}

func Test_minimum_level_by_environment(t *testing.T) {
	if len(domain.MinimumLevelByEnvironment) != 2 ||
		domain.MinimumLevelByEnvironment[domain.EnvironmentDevelopment] != domain.LevelDebug ||
		domain.MinimumLevelByEnvironment[domain.EnvironmentProduction] != domain.LevelInfo {
		t.Errorf("MinimumLevelByEnvironment = %v", domain.MinimumLevelByEnvironment)
	}
	for _, minimum_level := range domain.MinimumLevelByEnvironment {
		if !slices.Contains(domain.ValidLevels, minimum_level) {
			t.Errorf("minimum level %q is not a valid level", minimum_level)
		}
	}
}
