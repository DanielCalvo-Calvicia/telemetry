package batching_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/batching"
)

// fixtures_file is the case file that the Python tests read too (the same file).
const fixtures_file = "../../../../../../integration-tests/fixtures/delivery/batching_cases.json"

type expected_batch struct {
	AtMS int64 `json:"at_ms"`
	IDs  []int `json:"ids"`
}

type batching_case struct {
	Name            string           `json:"name"`
	BatchSize       int              `json:"batch_size"`
	FlushIntervalMS int64            `json:"flush_interval_ms"`
	SendDurationMS  int64            `json:"send_duration_ms"`
	Arrivals        []int64          `json:"arrivals"`
	ExpectedBatches []expected_batch `json:"expected_batches"`
}

type queued_event struct {
	id        int
	arrive_at int64
}

const never = int64(-1)

// simulate runs the algorithm that the description of the shared file gives, with PlanBatch as the decision.
func simulate(test_case batching_case) []expected_batch {
	batches := []expected_batch{}
	var queue []queued_event
	next_event := 0
	now, free_at := int64(0), int64(0)
	for guard := 0; guard < 100000; guard++ {
		for next_event < len(test_case.Arrivals) && test_case.Arrivals[next_event] <= now {
			queue = append(queue, queued_event{id: next_event + 1, arrive_at: test_case.Arrivals[next_event]})
			next_event++
		}
		next_decision := never
		if now >= free_at {
			oldest := int64(0)
			if len(queue) > 0 {
				oldest = queue[0].arrive_at
			}
			plan := batching.PlanBatch(now, len(queue), oldest, test_case.BatchSize, test_case.FlushIntervalMS)
			switch plan.Action {
			case batching.PlanSend:
				ids := []int{}
				for _, event := range queue[:plan.Take] {
					ids = append(ids, event.id)
				}
				queue = queue[plan.Take:]
				batches = append(batches, expected_batch{AtMS: now, IDs: ids})
				free_at = now + test_case.SendDurationMS
				continue
			case batching.PlanWait:
				next_decision = now + plan.WaitMS
			}
		}
		if now < free_at {
			next_decision = free_at
		}
		next_arrival := never
		if next_event < len(test_case.Arrivals) {
			next_arrival = test_case.Arrivals[next_event]
		}
		switch {
		case next_decision == never && next_arrival == never:
			return batches
		case next_decision == never:
			now = next_arrival
		case next_arrival == never || next_decision < next_arrival:
			now = next_decision
		default:
			now = next_arrival
		}
	}
	return batches
}

func Test_every_shared_batching_case_gives_the_expected_batches(t *testing.T) {
	content, read_error := os.ReadFile(fixtures_file)
	if read_error != nil {
		t.Fatal(read_error)
	}
	var fixture struct {
		Cases []batching_case `json:"cases"`
	}
	if decode_error := json.Unmarshal(content, &fixture); decode_error != nil {
		t.Fatal(decode_error)
	}
	if len(fixture.Cases) != 39 {
		t.Fatalf("expected 39 cases, got %d", len(fixture.Cases))
	}
	for _, test_case := range fixture.Cases {
		t.Run(test_case.Name, func(t *testing.T) {
			got := simulate(test_case)
			want := test_case.ExpectedBatches
			if want == nil {
				want = []expected_batch{}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v\nwant %+v", got, want)
			}
		})
	}
}

func Test_the_plan_of_the_edge_cases(t *testing.T) {
	cases := []struct {
		name               string
		now, oldest        int64
		queued, batch_size int
		flush              int64
		want               batching.Plan
	}{
		{"nothing queued is idle", 5, 0, 0, 10, 100, batching.Plan{Action: batching.PlanIdle, WaitMS: -1}},
		{"a full batch leaves at once", 1, 1, 10, 10, 100, batching.Plan{Action: batching.PlanSend, Take: 10}},
		{"more than a batch takes a batch", 1, 1, 25, 10, 100, batching.Plan{Action: batching.PlanSend, Take: 10}},
		{"an old batch leaves with what it has", 100, 0, 3, 10, 100, batching.Plan{Action: batching.PlanSend, Take: 3}},
		{"the boundary is inclusive", 100, 0, 1, 10, 100, batching.Plan{Action: batching.PlanSend, Take: 1}},
		{"one millisecond before the boundary waits", 99, 0, 1, 10, 100, batching.Plan{Action: batching.PlanWait, WaitMS: 1}},
		{"a young batch waits for the rest of the interval", 30, 10, 2, 10, 100, batching.Plan{Action: batching.PlanWait, WaitMS: 80}},
	}
	for _, test_case := range cases {
		if got := batching.PlanBatch(test_case.now, test_case.queued, test_case.oldest, test_case.batch_size, test_case.flush); got != test_case.want {
			t.Errorf("%s: got %+v, want %+v", test_case.name, got, test_case.want)
		}
	}
}

func Test_take_never_exceeds_the_batch_size_or_the_queued_events(t *testing.T) {
	for queued := 0; queued <= 30; queued++ {
		for batch_size := 1; batch_size <= 12; batch_size++ {
			for _, now := range []int64{0, 49, 50, 51, 500} {
				plan := batching.PlanBatch(now, queued, 0, batch_size, 50)
				if plan.Take > batch_size || plan.Take > queued || plan.Take < 0 {
					t.Fatalf("queued %d batch %d now %d: %+v", queued, batch_size, now, plan)
				}
				if plan.Action == batching.PlanSend && plan.Take < 1 {
					t.Fatalf("a send with nothing to take: %+v", plan)
				}
				if plan.Action == batching.PlanWait && plan.WaitMS <= 0 {
					t.Fatalf("a wait that is not positive: %+v", plan)
				}
			}
		}
	}
}
