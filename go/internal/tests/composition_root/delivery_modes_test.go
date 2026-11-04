package composition_root_test

import (
	"context"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/config"
	infra_config "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
)

// Each profile gives the behavior of its delivery mode (REQ-CFG-004): the first call of a destination fails with a
// transient failure and the next ones are accepted. With best_effort the batch is sent once and lost; with retry it is
// sent again and delivered.
func Test_each_profile_reaches_the_behavior_of_its_delivery_mode(t *testing.T) {
	cases := []struct {
		name            string
		profile         func() infra_config.Config
		expected_calls  int
		expected_resent bool
	}{
		{"microservice is best_effort", infra_config.Microservice, 1, false},
		{"cronjob is retry", infra_config.Cronjob, 2, true},
		{"worker is retry", infra_config.Worker, 2, true},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			cfg := test_case.profile()
			cfg.Resource.ServiceName = "service"
			cfg.Delivery.FlushIntervalMS = 5
			cfg.Delivery.RetryBaseDelayMS, cfg.Delivery.RetryMaxDelayMS = 1, 5
			sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 1, Reason: "status_503"}, dtos.SendResult{Accepted: 1})
			core := new_core(t, cfg, config.WithLogSink(sink))
			t.Cleanup(func() { core.Shutdown(context.Background()) })

			core.Emit(context.Background(), log_input())

			waiting.Eventually(t, waiting.DefaultTimeout, "the first call is made", func() bool { return sink.Calls() >= 1 })
			if test_case.expected_resent {
				waiting.Eventually(t, waiting.DefaultTimeout, "the batch is sent again", func() bool { return sink.Calls() == test_case.expected_calls })
			}
			waiting.StaysTrue(t, 80*time.Millisecond, "no more calls than the mode allows", func() bool { return sink.Calls() == test_case.expected_calls })
		})
	}
}

func Test_a_core_with_a_retry_profile_reports_the_batch_as_sent_after_a_retry(t *testing.T) {
	cfg := infra_config.Worker()
	cfg.Resource.ServiceName = "service"
	cfg.Delivery.FlushIntervalMS = 5
	cfg.Delivery.RetryBaseDelayMS, cfg.Delivery.RetryMaxDelayMS = 1, 5
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 1, Reason: "status_503"}, dtos.SendResult{Accepted: 1})
	core := new_core(t, cfg, config.WithLogSink(sink))

	core.Emit(context.Background(), log_input())
	result := core.Shutdown(context.Background())

	if result.Sent != 1 || result.Dropped != 0 || sink.Calls() != 2 {
		t.Fatalf("result %+v after %d calls", result, sink.Calls())
	}
}

func Test_the_retry_settings_of_the_configuration_reach_the_delivery(t *testing.T) {
	cfg := infra_config.Worker()
	cfg.Resource.ServiceName = "service"
	cfg.Delivery.FlushIntervalMS = 5
	cfg.Delivery.RetryMaxAttempts = 3
	cfg.Delivery.RetryBaseDelayMS, cfg.Delivery.RetryMaxDelayMS = 1, 5
	sink := fakes.NewScriptedLogSink(dtos.SendResult{Failed: 1, Reason: "status_503"})
	core := new_core(t, cfg, config.WithLogSink(sink))

	core.Emit(context.Background(), log_input())
	result := core.Shutdown(context.Background())

	if sink.Calls() != 3 || result.Sent != 0 || result.Dropped != 1 {
		t.Fatalf("result %+v after %d calls, expected 3 attempts and the event dropped", result, sink.Calls())
	}
}
