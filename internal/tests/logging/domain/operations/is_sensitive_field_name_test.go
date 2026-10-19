package operations_test

import (
	"testing"

	operations "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/operations"
)

func Test_is_sensitive_field_name(t *testing.T) {
	test_cases := []struct {
		field_name string
		expected   bool
	}{
		{"password", true},
		{"db_password", true},
		{"Password", true},
		{"dbPassword", true},
		{"access-key", true},
		{"accessKey", true},
		{"secret", true},
		{"secret_string", true},
		{"SecretString", true},
		{"secret_binary", true},
		{"client_secret", true},
		{"api_key", true},
		{"authorization", true},
		{"access_token", true},
		{"session_token", true},
		{"request_body", true},
		{"body", true},
		{"message_body", true},
		{"MessageBody", true},
		{"content", true},
		{"private_key", true},
		{"x-amz-signature", true},

		{"", false},
		{"key", false},
		{"bucket", false},
		{"secret_id", false},
		{"secret_name", false},
		{"secret_arn", false},
		{"token_count", false},
		{"queue_url", false},
		{"error", false},
		{"message", false},
		{"size", false},
		{"bodyguard", false},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.field_name, func(t *testing.T) {
			if actual := operations.IsSensitiveFieldName(test_case.field_name); actual != test_case.expected {
				t.Errorf("IsSensitiveFieldName(%q) = %v, want %v", test_case.field_name, actual, test_case.expected)
			}
		})
	}
}
