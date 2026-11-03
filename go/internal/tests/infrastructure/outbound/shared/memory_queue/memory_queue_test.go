package memory_queue_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/full_queue"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/memory_queue"
)

const (
	normal   = value_objects.EventClassNormal
	bulk     = value_objects.EventClassBulk
	critical = value_objects.EventClassCritical

	drop_newest = value_objects.QueuePolicyDropNewest
	drop_oldest = value_objects.QueuePolicyDropOldest
	block       = value_objects.QueuePolicyBlockWithTimeout
)

func envelope_of(id int) value_objects.Envelope {
	return value_objects.Envelope{EventId: strconv.Itoa(id)}
}

// ids_of takes everything out of the queue and returns the ids in order.
func ids_of(queue *memory_queue.MemoryQueue) []string {
	ids := []string{}
	for _, envelope := range queue.Take(queue.Len()) {
		ids = append(ids, envelope.EventId)
	}
	return ids
}

func require_ids(t *testing.T, queue *memory_queue.MemoryQueue, expected ...string) {
	t.Helper()
	got := ids_of(queue)
	if len(got) != len(expected) {
		t.Fatalf("the queue holds %v, want %v", got, expected)
	}
	for index := range expected {
		if got[index] != expected[index] {
			t.Fatalf("the queue holds %v, want %v", got, expected)
		}
	}
}

// filled_queue returns a queue of the capacity of the classes, full, with the ids 1, 2, 3... and those classes.
func filled_queue(t *testing.T, classes ...value_objects.EventClass) *memory_queue.MemoryQueue {
	t.Helper()
	queue := memory_queue.NewMemoryQueue(len(classes))
	for index, event_class := range classes {
		if result := queue.Offer(envelope_of(index+1), event_class, int64((index+1)*10), drop_newest); !result.Admitted {
			t.Fatalf("offer %d refused", index+1)
		}
	}
	return queue
}

func Test_the_queue_keeps_the_order_and_takes_the_oldest_first(t *testing.T) {
	queue := memory_queue.NewMemoryQueue(10)
	for id := 1; id <= 5; id++ {
		if result := queue.Offer(envelope_of(id), normal, int64(id*10), drop_newest); !result.Admitted || result.Displaced != nil || result.MustWait || result.Reason != "" {
			t.Fatalf("offer %d gave %+v", id, result)
		}
	}
	if oldest, has_oldest := queue.OldestArrivalMS(); !has_oldest || oldest != 10 || queue.Len() != 5 {
		t.Fatalf("oldest %d %v, len %d", oldest, has_oldest, queue.Len())
	}
	taken := queue.Take(2)
	if len(taken) != 2 || taken[0].EventId != "1" || taken[1].EventId != "2" {
		t.Fatalf("taken %+v", taken)
	}
	if oldest, _ := queue.OldestArrivalMS(); oldest != 30 || queue.Len() != 3 {
		t.Fatalf("oldest %d, len %d", oldest, queue.Len())
	}
}

func Test_a_full_queue_with_drop_newest_refuses_without_blocking_and_changes_nothing(t *testing.T) {
	queue := filled_queue(t, normal, normal, normal)

	result := queue.Offer(envelope_of(4), normal, 0, drop_newest)

	if result.Admitted || result.Displaced != nil || result.MustWait || result.Reason != full_queue.ReasonQueueFullNewest {
		t.Fatalf("result %+v", result)
	}
	if queue.Len() != 3 {
		t.Fatalf("len %d", queue.Len())
	}
	queue.Take(1)
	if !queue.Offer(envelope_of(5), normal, 0, drop_newest).Admitted {
		t.Fatal("a queue with room refused")
	}
	require_ids(t, queue, "2", "3", "5")
}

func Test_drop_oldest_removes_the_oldest_event_of_a_class_not_higher_and_keeps_the_order(t *testing.T) {
	queue := filled_queue(t, critical, normal, bulk, normal)

	result := queue.Offer(envelope_of(100), normal, 99, drop_oldest)

	if !result.Admitted || result.MustWait || result.Reason != full_queue.ReasonQueueFullOldest {
		t.Fatalf("result %+v", result)
	}
	if result.Displaced == nil || result.Displaced.EventId != "2" {
		t.Fatalf("displaced %+v", result.Displaced)
	}
	if oldest, _ := queue.OldestArrivalMS(); oldest != 10 {
		t.Fatalf("the oldest arrival is %d", oldest)
	}
	require_ids(t, queue, "1", "3", "4", "100")
}

func Test_drop_oldest_drops_the_new_event_when_nothing_queued_is_of_a_class_not_higher(t *testing.T) {
	queue := filled_queue(t, critical, critical)

	result := queue.Offer(envelope_of(100), normal, 0, drop_oldest)

	if result.Admitted || result.Displaced != nil || result.Reason != full_queue.ReasonQueueFullNewest {
		t.Fatalf("result %+v", result)
	}
	require_ids(t, queue, "1", "2")
}

func Test_a_critical_event_displaces_the_oldest_bulk_with_any_policy(t *testing.T) {
	for _, policy := range []value_objects.QueuePolicy{drop_newest, drop_oldest, block} {
		queue := filled_queue(t, normal, bulk, normal, bulk, critical)

		result := queue.Offer(envelope_of(100), critical, 0, policy)

		if !result.Admitted || result.Displaced == nil || result.Displaced.EventId != "2" || result.Reason != full_queue.ReasonDisplacedByCritical {
			t.Fatalf("%s: result %+v", policy, result)
		}
		require_ids(t, queue, "1", "3", "4", "5", "100")
	}
}

func Test_a_critical_event_displaces_the_oldest_normal_when_there_is_no_bulk(t *testing.T) {
	queue := filled_queue(t, critical, normal, critical, normal)

	result := queue.Offer(envelope_of(100), critical, 0, drop_newest)

	if !result.Admitted || result.Displaced == nil || result.Displaced.EventId != "2" {
		t.Fatalf("result %+v", result)
	}
	require_ids(t, queue, "1", "3", "4", "100")
}

func Test_block_with_timeout_answers_must_wait_and_changes_nothing(t *testing.T) {
	queue := filled_queue(t, normal, critical)

	result := queue.Offer(envelope_of(100), normal, 0, block)

	if result.Admitted || result.Displaced != nil || !result.MustWait || result.Reason != full_queue.ReasonBlockTimeout {
		t.Fatalf("result %+v", result)
	}
	require_ids(t, queue, "1", "2")
}

func Test_displacing_in_a_ring_that_has_wrapped_keeps_the_order(t *testing.T) {
	queue := filled_queue(t, normal, bulk, normal, normal)
	queue.Take(2) // the ring head is now in the middle of the storage
	queue.Offer(envelope_of(5), normal, 50, drop_newest)
	queue.Offer(envelope_of(6), bulk, 60, drop_newest) // the queue holds 3, 4, 5, 6 and wraps

	result := queue.Offer(envelope_of(100), critical, 100, drop_newest)

	if !result.Admitted || result.Displaced == nil || result.Displaced.EventId != "6" {
		t.Fatalf("result %+v", result)
	}
	require_ids(t, queue, "3", "4", "5", "100")
}

func Test_displacing_the_oldest_and_the_last_event_works(t *testing.T) {
	queue := filled_queue(t, bulk, normal, normal)
	if result := queue.Offer(envelope_of(100), critical, 0, drop_newest); result.Displaced == nil || result.Displaced.EventId != "1" {
		t.Fatalf("result %+v", result)
	}
	require_ids(t, queue, "2", "3", "100")

	queue = filled_queue(t, normal, normal, bulk)
	if result := queue.Offer(envelope_of(100), critical, 0, drop_newest); result.Displaced == nil || result.Displaced.EventId != "3" {
		t.Fatalf("result %+v", result)
	}
	require_ids(t, queue, "1", "2", "100")
}

func Test_a_queue_of_one_displaces_its_only_event(t *testing.T) {
	queue := filled_queue(t, normal)
	result := queue.Offer(envelope_of(100), normal, 5, drop_oldest)
	if !result.Admitted || result.Displaced == nil || result.Displaced.EventId != "1" || queue.Len() != 1 {
		t.Fatalf("result %+v, len %d", result, queue.Len())
	}
	if oldest, _ := queue.OldestArrivalMS(); oldest != 5 {
		t.Fatalf("the oldest arrival is %d", oldest)
	}
}

func Test_take_more_than_held_and_take_nothing(t *testing.T) {
	queue := memory_queue.NewMemoryQueue(4)
	if len(queue.Take(3)) != 0 {
		t.Fatal("an empty queue gave events")
	}
	if _, has_oldest := queue.OldestArrivalMS(); has_oldest {
		t.Fatal("an empty queue has an oldest")
	}
	queue.Offer(envelope_of(1), normal, 7, drop_newest)
	queue.Offer(envelope_of(2), normal, 8, drop_newest)
	if len(queue.Take(0)) != 0 || len(queue.Take(-5)) != 0 || queue.Len() != 2 {
		t.Fatal("a take of zero or less must take nothing")
	}
	if taken := queue.Take(100); len(taken) != 2 || queue.Len() != 0 {
		t.Fatalf("taken %d, len %d", len(taken), queue.Len())
	}
}

func Test_a_capacity_below_one_is_one(t *testing.T) {
	queue := memory_queue.NewMemoryQueue(0)
	if !queue.Offer(envelope_of(1), normal, 0, drop_newest).Admitted || queue.Offer(envelope_of(2), normal, 0, drop_newest).Admitted {
		t.Fatal("a queue of capacity zero must hold exactly one event")
	}
}

func Test_many_goroutines_offer_and_take_and_the_counts_add_up(t *testing.T) {
	const pushers, pushes_each = 16, 500
	queue := memory_queue.NewMemoryQueue(pushers * pushes_each)
	var push_group, take_group sync.WaitGroup
	var accepted_mutex sync.Mutex
	accepted := 0
	for pusher := 0; pusher < pushers; pusher++ {
		push_group.Add(1)
		go func() {
			defer push_group.Done()
			for index := 0; index < pushes_each; index++ {
				if queue.Offer(envelope_of(index), normal, 0, drop_newest).Admitted {
					accepted_mutex.Lock()
					accepted++
					accepted_mutex.Unlock()
				}
			}
		}()
	}
	taken_total := 0
	stop_taking := make(chan struct{})
	take_group.Add(1)
	go func() {
		defer take_group.Done()
		for {
			taken_total += len(queue.Take(37))
			select {
			case <-stop_taking:
				return
			default:
			}
		}
	}()
	push_group.Wait()
	close(stop_taking)
	take_group.Wait()
	taken_total += len(queue.Take(pushers * pushes_each))
	if accepted != pushers*pushes_each || taken_total != accepted || queue.Len() != 0 {
		t.Fatalf("accepted %d, taken %d, left %d", accepted, taken_total, queue.Len())
	}
}

func Test_concurrent_offers_to_a_full_queue_keep_the_counts_and_the_capacity(t *testing.T) {
	const capacity, offerers, offers_each = 20, 8, 400
	for _, policy := range []value_objects.QueuePolicy{drop_newest, drop_oldest, block} {
		queue := memory_queue.NewMemoryQueue(capacity)
		classes := []value_objects.EventClass{bulk, normal, critical}
		var group sync.WaitGroup
		var counts_mutex sync.Mutex
		admitted, displaced, refused, must_wait := 0, 0, 0, 0
		for offerer := 0; offerer < offerers; offerer++ {
			group.Add(1)
			go func() {
				defer group.Done()
				for index := 0; index < offers_each; index++ {
					result := queue.Offer(envelope_of(offerer*offers_each+index), classes[(offerer+index)%3], 0, policy)
					if queue.Len() > capacity {
						t.Errorf("the queue holds %d, over the capacity", queue.Len())
					}
					counts_mutex.Lock()
					switch {
					case result.MustWait:
						must_wait++
					case result.Admitted:
						admitted++
						if result.Displaced != nil {
							displaced++
						}
					default:
						refused++
					}
					counts_mutex.Unlock()
				}
			}()
		}
		group.Wait()

		if admitted+refused+must_wait != offerers*offers_each {
			t.Fatalf("%s: admitted %d + refused %d + must wait %d is not %d", policy, admitted, refused, must_wait, offerers*offers_each)
		}
		if queue.Len() != capacity || admitted-displaced != capacity {
			t.Fatalf("%s: the queue holds %d, admitted %d, displaced %d", policy, queue.Len(), admitted, displaced)
		}
		if policy == drop_newest && displaced == 0 {
			t.Fatalf("%s: critical events should have displaced others", policy)
		}
		if policy != block && must_wait != 0 {
			t.Fatalf("%s: only block_with_timeout waits", policy)
		}
	}
}
