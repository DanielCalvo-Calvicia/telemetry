package fakes

import (
	"context"
	"fmt"
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// FakeIDGenerator is a fake of outbound.IDGeneratorPort. It returns the queued event ids first and then valid
// UUIDv7 values derived from a counter, so a test gets the same ids on every run.
type FakeIDGenerator struct {
	mutex          sync.Mutex
	queued_ids     []value_objects.EventID
	event_counter  int
	instance_count int
}

// NewFakeIDGenerator returns a FakeIDGenerator that returns the event ids first, in order.
func NewFakeIDGenerator(queued_ids ...value_objects.EventID) *FakeIDGenerator {
	return &FakeIDGenerator{queued_ids: append([]value_objects.EventID(nil), queued_ids...)}
}

// NewEventID returns the next queued id or, when the queue is empty, a deterministic UUIDv7 from the counter.
func (fake_id_generator *FakeIDGenerator) NewEventID(_ context.Context) (value_objects.EventID, error) {
	fake_id_generator.mutex.Lock()
	defer fake_id_generator.mutex.Unlock()
	if len(fake_id_generator.queued_ids) > 0 {
		next_id := fake_id_generator.queued_ids[0]
		fake_id_generator.queued_ids = fake_id_generator.queued_ids[1:]
		return next_id, nil
	}
	fake_id_generator.event_counter++
	return value_objects.NewEventID(fmt.Sprintf("01900000-0000-7000-8000-%012x", fake_id_generator.event_counter))
}

// NewInstanceID returns a deterministic instance id: instance-1, instance-2 and so on.
func (fake_id_generator *FakeIDGenerator) NewInstanceID(_ context.Context) (base_value_objects.Identifier, error) {
	fake_id_generator.mutex.Lock()
	defer fake_id_generator.mutex.Unlock()
	fake_id_generator.instance_count++
	return base_value_objects.NewIdentifier(fmt.Sprintf("instance-%d", fake_id_generator.instance_count))
}
