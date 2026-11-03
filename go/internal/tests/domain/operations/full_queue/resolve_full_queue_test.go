package full_queue_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/full_queue"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// fixtures_file is the case file that the Python tests read too (the same file).
const fixtures_file = "../../../../../../integration-tests/fixtures/delivery/full_queue_cases.json"

type queued_event struct {
	ID    int    `json:"id"`
	Class string `json:"class"`
}

type removed_event struct {
	ID     int    `json:"id"`
	Reason string `json:"reason"`
}

type full_queue_case struct {
	Name     string         `json:"name"`
	Policy   string         `json:"policy"`
	Queue    []queued_event `json:"queue"`
	New      queued_event   `json:"new"`
	Expected struct {
		Outcome    string          `json:"outcome"`
		Removed    []removed_event `json:"removed"`
		QueueAfter []int           `json:"queue_after"`
		Reason     string          `json:"reason"`
	} `json:"expected"`
}

func classes_of(queue []queued_event) []value_objects.EventClass {
	classes := make([]value_objects.EventClass, 0, len(queue))
	for _, queued := range queue {
		classes = append(classes, value_objects.EventClass(queued.Class))
	}
	return classes
}

func Test_every_shared_full_queue_case_gives_the_same_resolution_and_queue(t *testing.T) {
	content, read_error := os.ReadFile(fixtures_file)
	if read_error != nil {
		t.Fatal(read_error)
	}
	var fixture struct {
		Cases []full_queue_case `json:"cases"`
	}
	if decode_error := json.Unmarshal(content, &fixture); decode_error != nil {
		t.Fatal(decode_error)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("the fixture has no case")
	}

	for _, test_case := range fixture.Cases {
		t.Run(test_case.Name, func(t *testing.T) {
			classes_before := classes_of(test_case.Queue)
			classes_copy := append([]value_objects.EventClass(nil), classes_before...)

			resolution := full_queue.ResolveFullQueue(value_objects.QueuePolicy(test_case.Policy), value_objects.EventClass(test_case.New.Class), classes_before)

			if !reflect.DeepEqual(classes_before, classes_copy) {
				t.Fatal("the function changed the list of classes it was given")
			}
			if string(resolution.Outcome) != test_case.Expected.Outcome || resolution.Reason != test_case.Expected.Reason {
				t.Fatalf("got %s with %s, want %s with %s", resolution.Outcome, resolution.Reason, test_case.Expected.Outcome, test_case.Expected.Reason)
			}

			// Apply the resolution to the queue, as the queue adapter does, and compare what the fixture says.
			queue_after := []int{}
			removed := []removed_event{}
			for _, queued := range test_case.Queue {
				queue_after = append(queue_after, queued.ID)
			}
			if resolution.Outcome == full_queue.OutcomeDisplace {
				removed = append(removed, removed_event{ID: queue_after[resolution.Index], Reason: resolution.Reason})
				queue_after = append(queue_after[:resolution.Index:resolution.Index], queue_after[resolution.Index+1:]...)
				queue_after = append(queue_after, test_case.New.ID)
			} else if resolution.Index != -1 {
				t.Fatalf("the index is %d for an outcome that removes nothing", resolution.Index)
			}
			if !reflect.DeepEqual(queue_after, test_case.Expected.QueueAfter) {
				t.Fatalf("queue after %v, want %v", queue_after, test_case.Expected.QueueAfter)
			}
			if !reflect.DeepEqual(removed, test_case.Expected.Removed) {
				t.Fatalf("removed %v, want %v", removed, test_case.Expected.Removed)
			}
		})
	}
}

func Test_an_empty_list_of_queued_classes_drops_or_waits_and_never_displaces(t *testing.T) {
	cases := []struct {
		policy    value_objects.QueuePolicy
		outcome   full_queue.Outcome
		reason    string
		new_class value_objects.EventClass
	}{
		{value_objects.QueuePolicyDropNewest, full_queue.OutcomeDropNew, full_queue.ReasonQueueFullNewest, value_objects.EventClassCritical},
		{value_objects.QueuePolicyDropOldest, full_queue.OutcomeDropNew, full_queue.ReasonQueueFullNewest, value_objects.EventClassNormal},
		{value_objects.QueuePolicyBlockWithTimeout, full_queue.OutcomeWait, full_queue.ReasonBlockTimeout, value_objects.EventClassBulk},
	}
	for _, test_case := range cases {
		for _, queued_classes := range [][]value_objects.EventClass{nil, {}} {
			resolution := full_queue.ResolveFullQueue(test_case.policy, test_case.new_class, queued_classes)
			if resolution.Outcome != test_case.outcome || resolution.Reason != test_case.reason || resolution.Index != -1 {
				t.Errorf("%s: %+v", test_case.policy, resolution)
			}
		}
	}
}

func Test_an_unknown_policy_is_drop_newest(t *testing.T) {
	resolution := full_queue.ResolveFullQueue("lazy", value_objects.EventClassNormal, []value_objects.EventClass{value_objects.EventClassBulk})
	if resolution.Outcome != full_queue.OutcomeDropNew || resolution.Reason != full_queue.ReasonQueueFullNewest {
		t.Errorf("%+v", resolution)
	}
}

func Test_an_unknown_class_ranks_as_normal(t *testing.T) {
	resolution := full_queue.ResolveFullQueue(value_objects.QueuePolicyDropOldest, "odd", []value_objects.EventClass{value_objects.EventClassCritical, value_objects.EventClassNormal})
	if resolution.Outcome != full_queue.OutcomeDisplace || resolution.Index != 1 {
		t.Errorf("%+v", resolution)
	}
}
