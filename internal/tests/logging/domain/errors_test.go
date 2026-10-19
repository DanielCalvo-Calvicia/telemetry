package domain_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain"
)

func Test_sentinels_have_expected_messages(t *testing.T) {
	test_cases := []struct {
		name            string
		sentinel        error
		expected_string string
	}{
		{"log entry creation failed", domain.ErrLogEntryCreationFailed, "log entry: creation failed"},
		{"invalid level", domain.ErrInvalidLevel, "invalid log level"},
		{"invalid environment", domain.ErrInvalidEnvironment, "invalid environment: expected development or production"},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			if test_case.sentinel == nil || test_case.sentinel.Error() != test_case.expected_string {
				t.Errorf("sentinel = %v, want message %q", test_case.sentinel, test_case.expected_string)
			}
		})
	}

	if errors.Is(domain.ErrLogEntryCreationFailed, domain.ErrInvalidLevel) {
		t.Error("the sentinels must be distinct")
	}
}

func Test_join_errors_keeps_every_cause(t *testing.T) {
	joined := domain.JoinErrors(domain.ErrInvalidLevel, domain.ErrLogEntryCreationFailed)

	if !errors.Is(joined, domain.ErrInvalidLevel) || !errors.Is(joined, domain.ErrLogEntryCreationFailed) {
		t.Error("joined error lost a cause")
	}
	if err := domain.JoinErrors(); err != nil {
		t.Errorf("JoinErrors() = %v, want nil", err)
	}
}
