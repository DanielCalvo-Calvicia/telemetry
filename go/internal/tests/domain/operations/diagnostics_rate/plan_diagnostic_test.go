package diagnostics_rate_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/diagnostics_rate"
)

// rate_cases_file is the case file that the Python tests read too (the same file).
const rate_cases_file = "../../../../../../integration-tests/fixtures/delivery/diagnostics_rate_cases.json"

type rate_call struct {
	Allow            bool `json:"allow"`
	AttachSuppressed int  `json:"attach_suppressed"`
}

type rate_state struct {
	WindowStartMS int64 `json:"window_start_ms"`
	Count         int   `json:"count"`
	Suppressed    int   `json:"suppressed"`
}

type rate_case struct {
	Name          string      `json:"name"`
	Limit         int         `json:"limit"`
	WindowMS      int64       `json:"window_ms"`
	Calls         []int64     `json:"calls"`
	Expected      []rate_call `json:"expected"`
	ExpectedState rate_state  `json:"expected_state"`
}

func Test_every_shared_rate_case_gives_the_same_answers_and_the_same_final_state(t *testing.T) {
	content, read_error := os.ReadFile(rate_cases_file)
	if read_error != nil {
		t.Fatal(read_error)
	}
	var file struct {
		Cases []rate_case `json:"cases"`
	}
	if decode_error := json.Unmarshal(content, &file); decode_error != nil {
		t.Fatal(decode_error)
	}
	if len(file.Cases) != 30 {
		t.Fatalf("expected 30 cases, got %d", len(file.Cases))
	}
	for _, shared_case := range file.Cases {
		t.Run(shared_case.Name, func(t *testing.T) {
			state := rate_state{WindowStartMS: diagnostics_rate.NoWindow}
			if len(shared_case.Calls) != len(shared_case.Expected) {
				t.Fatalf("%d calls and %d answers", len(shared_case.Calls), len(shared_case.Expected))
			}
			for position, now_ms := range shared_case.Calls {
				plan := diagnostics_rate.PlanDiagnostic(now_ms, state.WindowStartMS, state.Count, state.Suppressed, shared_case.Limit, shared_case.WindowMS)
				state = rate_state{WindowStartMS: plan.WindowStartMS, Count: plan.Count, Suppressed: plan.Suppressed}
				answer := rate_call{Allow: plan.Allow, AttachSuppressed: plan.AttachSuppressed}
				if answer != shared_case.Expected[position] {
					t.Errorf("call %d at %d ms: got %+v, expected %+v", position, now_ms, answer, shared_case.Expected[position])
				}
			}
			if state != shared_case.ExpectedState {
				t.Errorf("final state %+v, expected %+v", state, shared_case.ExpectedState)
			}
		})
	}
}

func Test_the_defaults_of_the_rate_limit_are_ten_per_second_and_a_queue_of_sixty_four(t *testing.T) {
	if domain.DefaultDiagnosticsRateLimit != 10 || domain.DefaultDiagnosticsRateWindowMilliseconds != 1000 || domain.DefaultDiagnosticsQueueSize != 64 {
		t.Fatal("the defaults of the diagnostics channel changed")
	}
}
