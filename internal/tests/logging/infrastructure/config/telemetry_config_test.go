package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/config"
)

// The struct has no behaviour, so this test protects its contract with composition_root/config and
// .env.example: the variable names, the defaults of the old project and the separator of the extra patterns.
func Test_telemetry_config_env_tags(t *testing.T) {
	test_cases := []struct {
		field_name       string
		expected_env_tag string
	}{
		{"Host", "LOGGER_HOST,default=http://localhost"},
		{"Port", "LOGGER_PORT,default=3000"},
		{"Path", "LOGGER_PATH,default=/logs"},
		{"ConsoleEnabled", "LOGGER_CONSOLE_ENABLED,default=true"},
		{"HTTPEnabled", "LOGGER_HTTP_ENABLED,default=false"},
		{"MaskExtraPatterns", "LOGGER_MASK_EXTRA_PATTERNS, delimiter=;"},
		{"AWSHTTPEnabled", "LOGGER_AWS_HTTP_ENABLED,default=false"},
		{"AWSHTTPHost", "LOGGER_AWS_HTTP_HOST,default=http://localhost"},
		{"AWSHTTPPort", "LOGGER_AWS_HTTP_PORT,default=8080"},
		{"AWSHTTPPath", "LOGGER_AWS_HTTP_PATH,default=/api/v1/cloudwatch/logs/add"},
		{"AWSHTTPLogGroup", "LOGGER_AWS_HTTP_LOG_GROUP"},
		{"AWSHTTPLogStream", "LOGGER_AWS_HTTP_LOG_STREAM"},
	}

	config_type := reflect.TypeOf(config.TelemetryConfig{})
	if config_type.NumField() != len(test_cases) {
		t.Fatalf("TelemetryConfig has %d fields, want %d", config_type.NumField(), len(test_cases))
	}

	for _, test_case := range test_cases {
		t.Run(test_case.field_name, func(t *testing.T) {
			field, found := config_type.FieldByName(test_case.field_name)
			if !found {
				t.Fatalf("field %s not found", test_case.field_name)
			}
			if field.Tag.Get("env") != test_case.expected_env_tag {
				t.Errorf("env tag = %q, want %q", field.Tag.Get("env"), test_case.expected_env_tag)
			}
		})
	}
}

func Test_telemetry_config_has_no_log_level_variable(t *testing.T) {
	config_type := reflect.TypeOf(config.TelemetryConfig{})
	for index := range config_type.NumField() {
		if strings.Contains(strings.ToUpper(config_type.Field(index).Tag.Get("env")), "LEVEL") {
			t.Errorf("the minimum level follows APP_ENV, there must be no level variable: %s", config_type.Field(index).Name)
		}
	}
}
