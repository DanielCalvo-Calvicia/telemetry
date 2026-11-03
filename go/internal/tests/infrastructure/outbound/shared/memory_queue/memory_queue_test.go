package memory_queue_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/memory_queue"
)

func envelope_of(id int) value_objects.Envelope {
	return value_objects.Envelope{EventId: strconv.Itoa(id)}
}

func Test_the_queue_keeps_the_order_and_takes_the_oldest_first(t *testing.T) {
	queue := memory_queue.NewMemoryQueue(10)
	for id := 1; id <= 5; id++ {
		if !queue.Push(envelope_of(id), int64(id*10)) {
			t.Fatalf("push %d refused", id)
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

func Test_the_queue_is_bounded_and_a_full_queue_refuses_without_blocking(t *testing.T) {
	queue := memory_queue.NewMemoryQueue(3)
	for id := 1; id <= 3; id++ {
		queue.Push(envelope_of(id), 0)
	}
	if queue.Push(envelope_of(4), 0) {
		t.Fatal("a full queue took an event")
	}
	if queue.Len() != 3 {
		t.Fatalf("len %d", queue.Len())
	}
	queue.Take(1)
	if !queue.Push(envelope_of(5), 0) {
		t.Fatal("a queue with room refused")
	}
	ids := []string{}
	for _, envelope := range queue.Take(10) {
		ids = append(ids, envelope.EventId)
	}
	if len(ids) != 3 || ids[0] != "2" || ids[1] != "3" || ids[2] != "5" {
		t.Fatalf("the order after wrapping is %v", ids)
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
	queue.Push(envelope_of(1), 7)
	queue.Push(envelope_of(2), 8)
	if len(queue.Take(0)) != 0 || len(queue.Take(-5)) != 0 || queue.Len() != 2 {
		t.Fatal("a take of zero or less must take nothing")
	}
	if taken := queue.Take(100); len(taken) != 2 || queue.Len() != 0 {
		t.Fatalf("taken %d, len %d", len(taken), queue.Len())
	}
}

func Test_a_capacity_below_one_is_one(t *testing.T) {
	queue := memory_queue.NewMemoryQueue(0)
	if !queue.Push(envelope_of(1), 0) || queue.Push(envelope_of(2), 0) {
		t.Fatal("a queue of capacity zero must hold exactly one event")
	}
}

func Test_many_goroutines_push_and_take_and_the_counts_add_up(t *testing.T) {
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
				if queue.Push(envelope_of(index), 0) {
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
