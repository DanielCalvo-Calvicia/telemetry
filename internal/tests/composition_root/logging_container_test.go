package composition_root_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/composition_root/containers"
	logging_config "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/config"
)

func Test_the_logger_is_built_with_every_sink_the_settings_switch_on(t *testing.T) {
	settings := logging_config.TelemetryConfig{
		ConsoleEnabled: true,
		HTTPEnabled:    true, Host: "http://localhost", Port: "3000", Path: "/logs",
		AWSHTTPEnabled: true, AWSHTTPHost: "http://localhost", AWSHTTPPort: "8080", AWSHTTPPath: "/api/v1/cloudwatch/logs/add",
		AWSHTTPLogGroup: "group", AWSHTTPLogStream: "stream",
	}

	logging_container, err := containers.NewLoggingContainer(context.Background(), "production", settings)

	if err != nil || logging_container.Logger == nil {
		t.Fatalf("container = %v, error = %v", logging_container, err)
	}
	if err := logging_container.Shutdown(context.Background()); err != nil {
		t.Errorf("shutdown: %v", err)
	}
}

func Test_a_sink_without_its_settings_stops_the_assembly_with_the_reason(t *testing.T) {
	test_cases := []struct {
		name     string
		app_env  string
		settings logging_config.TelemetryConfig
	}{
		{"http sink without a port", "development", logging_config.TelemetryConfig{HTTPEnabled: true, Host: "http://localhost", Path: "/logs"}},
		{"aws http sink without a log group", "development", logging_config.TelemetryConfig{AWSHTTPEnabled: true, AWSHTTPHost: "http://localhost", AWSHTTPPort: "8080", AWSHTTPPath: "/p", AWSHTTPLogStream: "s"}},
		{"an app env without a level", "staging", logging_config.TelemetryConfig{}},
		{"a mask pattern that is not a regular expression", "development", logging_config.TelemetryConfig{MaskExtraPatterns: []string{"("}}},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			logging_container, err := containers.NewLoggingContainer(context.Background(), test_case.app_env, test_case.settings)

			if logging_container != nil || !errors.Is(err, containers.ErrLoggerBuildFailed) || !errors.Is(err, containers.ErrContainerBuildFailed) {
				t.Errorf("container = %v, error = %v", logging_container, err)
			}
		})
	}
}
