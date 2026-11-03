package services

import (
	"context"
	"maps"
	"sync"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/batching"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/classification"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/full_queue"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// DELIVERY SERVICE
// =============================================================================

// The reasons of the events that a queue dropped or displaced (REQ-DLV-008). They are the keys of
// DestinationStats.DroppedByReason.
const (
	// ReasonQueueFullNewest is the reason of a new event that was dropped because the queue of its destination was full.
	ReasonQueueFullNewest = full_queue.ReasonQueueFullNewest
	// ReasonQueueFullOldest is the reason of a queued event that was removed to make room for a new one (drop_oldest).
	ReasonQueueFullOldest = full_queue.ReasonQueueFullOldest
	// ReasonDisplacedByCritical is the reason of a queued event that was removed to make room for a critical one.
	ReasonDisplacedByCritical = full_queue.ReasonDisplacedByCritical
	// ReasonBlockTimeout is the reason of a new event that found no room within the timeout of block_with_timeout.
	ReasonBlockTimeout = full_queue.ReasonBlockTimeout
	// ReasonUnspecified is the reason under which a drop that a sink reported without a reason is counted.
	ReasonUnspecified = "unspecified"
)

// DeliverySettings are the numbers the delivery service works with: the events one queue holds, the events of one batch,
// the milliseconds before a batch that is not full leaves, the milliseconds before a call to a sink gives up, what a full
// queue does (QueuePolicy; empty is drop_newest), the milliseconds a call may wait for room with block_with_timeout
// (BlockTimeoutMS) and the class assigned to an event type (EventClasses; nil is none, and then the default class of the
// event applies). They are already validated (greater than zero, and the batch not greater than the queue).
type DeliverySettings struct {
	QueueSize       int
	BatchSize       int
	FlushIntervalMS int
	TimeoutMS       int
	QueuePolicy     value_objects.QueuePolicy
	BlockTimeoutMS  int
	EventClasses    map[string]value_objects.EventClass
}

// DestinationStats is a snapshot of one destination, the minimum needed to observe its worker. Kind and Index say which
// sink it is (Index counts the sinks of the kind, from zero, in the order given). Queued is the events waiting now.
// Batches is the calls to the sink. Accepted is the events that were queued; Dropped is the events that were not queued,
// the ones that were removed from the queue to make room and the ones the sink dropped; Failed is the events the sink
// failed to take; Delivered is the events the sink accepted. LastReason is the last reason that was not empty.
// DroppedByReason counts every dropped event per reason (REQ-DLV-008): the ones the queue dropped or displaced
// (queue_full_newest, queue_full_oldest, displaced_by_critical, block_timeout) and the ones a sink reported as dropped,
// under the reason of its result or ReasonUnspecified when it gave none. Its sum is equal to Dropped. The map is a copy:
// the caller may change it.
type DestinationStats struct {
	Kind            DestinationKind
	Index           int
	Queued          int
	Batches         int
	Accepted        int
	Dropped         int
	Failed          int
	Delivered       int
	LastReason      string
	DroppedByReason map[string]int
}

// DeliveryService holds one bounded queue and one worker per destination. Enqueue only touches the queue, so a call
// never waits for the network, the disk or a slow destination; the worker of each destination sends the events in
// batches (REQ-DLV-002, REQ-DLV-006). The order of the events of one destination is kept. A slow destination blocks
// only its own worker. It holds no global state and is safe for concurrent use.
//
// Priority is not a guarantee (REQ-DLV-009): the class of an event decides who is dropped when a queue is full, and
// nothing more; any event can still be lost.
type DeliveryService struct {
	settings     DeliverySettings
	clock        outbound.ClockPort
	destinations []*delivery_destination
	by_kind      map[DestinationKind][]*delivery_destination
	stop_signal  chan struct{}
	stop_once    sync.Once

	lifecycle_mutex sync.Mutex
	started         bool
	stopped         bool
	cancel_workers  context.CancelFunc
	workers         sync.WaitGroup
}

// delivery_destination is one sink with its queue, its wake-up signal and its counters.
type delivery_destination struct {
	kind  DestinationKind
	index int
	send  func(ctx context.Context, events []value_objects.Envelope) dtos.SendResult
	queue outbound.EventQueuePort
	wake  chan struct{}

	// room is closed (and set to nil) each time the worker takes events, to wake the callers that wait for room with
	// block_with_timeout. It is nil while nobody waits, so the usual path allocates nothing.
	room_mutex sync.Mutex
	room       chan struct{}

	stats_mutex sync.Mutex
	stats       DestinationStats
}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewDeliveryService returns a DeliveryService with one queue, made by new_queue with the queue size, for every sink of
// every kind in the destinations. It starts nothing: no goroutine runs until Start is called.
func NewDeliveryService(
	destinations Destinations,
	settings DeliverySettings,
	clock outbound.ClockPort,
	new_queue func(capacity int) outbound.EventQueuePort,
) *DeliveryService {
	service := &DeliveryService{
		settings:    settings,
		clock:       clock,
		by_kind:     map[DestinationKind][]*delivery_destination{},
		stop_signal: make(chan struct{}),
	}
	for _, kind := range destination_kinds() {
		for index, send := range send_functions_of(destinations, kind) {
			destination := &delivery_destination{
				kind:  kind,
				index: index,
				send:  send,
				queue: new_queue(settings.QueueSize),
				wake:  make(chan struct{}, 1),
				stats: DestinationStats{Kind: kind, Index: index, DroppedByReason: map[string]int{}},
			}
			service.destinations = append(service.destinations, destination)
			service.by_kind[kind] = append(service.by_kind[kind], destination)
		}
	}
	return service
}

// ----- METHODS --------------------------------------------------------------

// Start starts one worker goroutine per destination. It is idempotent: a second call does nothing, and a call after Stop
// does nothing either.
func (service *DeliveryService) Start() {
	service.lifecycle_mutex.Lock()
	defer service.lifecycle_mutex.Unlock()
	if service.started || service.stopped {
		return
	}
	service.started = true
	workers_context, cancel_workers := context.WithCancel(context.Background())
	service.cancel_workers = cancel_workers
	for _, destination := range service.destinations {
		service.workers.Add(1)
		go service.run_worker(workers_context, destination)
	}
}

// Stop signals every worker to stop and waits until they have returned. A call to a sink that is in flight gets its
// context cancelled; a sink that ignores its context holds Stop up. A caller of Enqueue that waits for room
// (block_with_timeout) is woken, and its event is dropped with ReasonBlockTimeout. It is idempotent and it does not
// drain: what is still queued is left (draining, flush and shutdown with a result are T-05.4).
func (service *DeliveryService) Stop() {
	service.stop_once.Do(func() { close(service.stop_signal) })
	service.lifecycle_mutex.Lock()
	service.stopped = true
	cancel_workers := service.cancel_workers
	service.lifecycle_mutex.Unlock()
	if cancel_workers != nil {
		cancel_workers()
	}
	service.workers.Wait()
}

// Enqueue classifies the envelope (classification.ClassifyEvent, with its event type, its severity and the classes of
// the settings) and offers it to the queue of every destination of the kind. No destination of the kind is Dropped 1
// with ReasonNoDestination. For each destination: an admitted event is Accepted 1 (accepted means queued); when a queued
// event was displaced to make room, the displaced event is counted as dropped in the stats of that destination under its
// reason, and the result of this call does not change because of it; a refused event is Dropped 1 with the reason of the
// drop (ReasonQueueFullNewest, or ReasonBlockTimeout after a wait). The result is the sum over the destinations and
// Reason is the first one that is not empty.
//
// A call returns at once, except in ONE case: with the policy block_with_timeout and a full queue that holds nothing
// the event may displace, the call WAITS for room, at most BlockTimeoutMS milliseconds (a real wait on a signal of the
// worker and a timer, never a loop that spins), and offers again each time the worker takes events. If room appears the
// event is Accepted 1; if the time is over, the service is stopped or ctx is done, it is Dropped 1 with
// ReasonBlockTimeout. With any other policy the call never waits.
func (service *DeliveryService) Enqueue(ctx context.Context, kind DestinationKind, envelope value_objects.Envelope) dtos.EmitResult {
	kind_destinations := service.by_kind[kind]
	if len(kind_destinations) == 0 {
		return dtos.EmitResult{Dropped: 1, Reason: ReasonNoDestination}
	}

	event_class := classification.ClassifyEvent(envelope.EventType, envelope.Severity, service.settings.EventClasses)
	var total dtos.EmitResult
	for _, destination := range kind_destinations {
		offer_result := service.offer_to(ctx, destination, envelope, event_class)
		if offer_result.Admitted {
			total.Accepted++
			destination.count(func(stats *DestinationStats) {
				stats.Accepted++
				if offer_result.Displaced != nil {
					stats.count_drop(offer_result.Reason)
				}
			})
			destination.notify()
			continue
		}
		total.Dropped++
		destination.count(func(stats *DestinationStats) { stats.count_drop(offer_result.Reason) })
		if total.Reason == "" {
			total.Reason = offer_result.Reason
		}
	}
	return total
}

// Stats returns a snapshot of every destination: by kind, in the order of the kinds, and by index inside a kind.
func (service *DeliveryService) Stats() []DestinationStats {
	snapshot := make([]DestinationStats, 0, len(service.destinations))
	for _, destination := range service.destinations {
		destination.stats_mutex.Lock()
		stats := destination.stats
		stats.DroppedByReason = maps.Clone(destination.stats.DroppedByReason)
		destination.stats_mutex.Unlock()
		stats.Queued = destination.queue.Len()
		snapshot = append(snapshot, stats)
	}
	return snapshot
}

// ----- THE OFFER ------------------------------------------------------------

// offer_to offers the envelope to the queue of one destination and, only when the queue says MustWait (the policy is
// block_with_timeout), waits for room and offers again until the event is admitted, the time is over, the service is
// stopped or ctx is done. A wait that ended without room is not admitted, with ReasonBlockTimeout.
func (service *DeliveryService) offer_to(
	ctx context.Context,
	destination *delivery_destination,
	envelope value_objects.Envelope,
	event_class value_objects.EventClass,
) outbound.OfferResult {
	offer := func() outbound.OfferResult {
		arrival_ms := service.clock.MonotonicNanoseconds(ctx) / 1_000_000
		return destination.queue.Offer(envelope, event_class, arrival_ms, service.settings.QueuePolicy)
	}
	offer_result := offer()
	if !offer_result.MustWait {
		return offer_result
	}

	timed_out := outbound.OfferResult{Reason: ReasonBlockTimeout}
	if service.settings.BlockTimeoutMS <= 0 {
		return timed_out
	}
	timer := time.NewTimer(time.Duration(service.settings.BlockTimeoutMS) * time.Millisecond)
	defer timer.Stop()
	for offer_result.MustWait {
		// The channel is taken BEFORE the offer is repeated, so room that appears between the two is not missed: the
		// worker closes the channel after it takes events, and a closed channel ends the wait at once.
		room := destination.room_channel()
		offer_result = offer()
		if !offer_result.MustWait {
			break
		}
		select {
		case <-room:
		case <-timer.C:
			return timed_out
		case <-service.stop_signal:
			return timed_out
		case <-ctx.Done():
			return timed_out
		}
		offer_result = offer()
	}
	return offer_result
}

// ----- THE WORKER -----------------------------------------------------------

// run_worker is the loop of one destination: it asks the planner what to do, sends a batch, waits for the deadline of
// the oldest event or for a push, or waits for a push when the queue is empty. It returns when the context is done. It
// never holds a lock of the queue while it calls the sink, and it never spins: it always waits on a timer or a signal.
func (service *DeliveryService) run_worker(ctx context.Context, destination *delivery_destination) {
	defer service.workers.Done()
	for {
		if ctx.Err() != nil {
			return
		}
		plan := service.plan_for(ctx, destination)
		switch plan.Action {
		case batching.PlanSend:
			service.send_batch(ctx, destination, plan.Take)
		case batching.PlanWait:
			wait_for_signal(ctx, destination.wake, time.Duration(plan.WaitMS)*time.Millisecond)
		default:
			wait_for_signal(ctx, destination.wake, 0)
		}
	}
}

// plan_for asks the planner what the worker does next, with the state of the queue and the time of the clock.
func (service *DeliveryService) plan_for(ctx context.Context, destination *delivery_destination) batching.Plan {
	now_ms := service.clock.MonotonicNanoseconds(ctx) / 1_000_000
	queued := destination.queue.Len()
	oldest_arrival_ms, has_oldest := destination.queue.OldestArrivalMS()
	if !has_oldest {
		queued = 0
	}
	return batching.PlanBatch(now_ms, queued, oldest_arrival_ms, service.settings.BatchSize, int64(service.settings.FlushIntervalMS))
}

// send_batch takes the events from the queue, wakes the callers that wait for room, calls the sink once with the timeout
// of the settings and adds the result to the counters. There is no retry here (T-05.3). A panic of the sink is Failed
// with ReasonSinkPanicked.
func (service *DeliveryService) send_batch(ctx context.Context, destination *delivery_destination, take int) {
	batch := destination.queue.Take(take)
	if len(batch) == 0 {
		return
	}
	destination.signal_room()
	send_context, cancel_send := context.WithTimeout(ctx, time.Duration(service.settings.TimeoutMS)*time.Millisecond)
	defer cancel_send()
	result := send_protected(len(batch), func() dtos.SendResult { return destination.send(send_context, batch) })
	destination.count(func(stats *DestinationStats) {
		stats.Batches++
		stats.Delivered += result.Accepted
		stats.Failed += result.Failed
		if result.Dropped > 0 {
			drop_reason := result.Reason
			if drop_reason == "" {
				drop_reason = ReasonUnspecified
			}
			stats.count_drops(result.Dropped, drop_reason)
		} else if result.Reason != "" {
			stats.LastReason = result.Reason
		}
	})
}

// wait_for_signal blocks until the wake channel has a signal, the context is done or, when the duration is above zero,
// the duration passes. The timer is always stopped.
func wait_for_signal(ctx context.Context, wake <-chan struct{}, duration time.Duration) {
	if duration <= 0 {
		select {
		case <-wake:
		case <-ctx.Done():
		}
		return
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-wake:
	case <-timer.C:
	case <-ctx.Done():
	}
}

// ----- HELPERS --------------------------------------------------------------

// destination_kinds returns the kinds in the order in which the destinations and their stats are listed.
func destination_kinds() []DestinationKind {
	return []DestinationKind{
		DestinationKindLog, DestinationKindEvent, DestinationKindMetrics,
		DestinationKindTrace, DestinationKindDeviceTelemetry, DestinationKindAITelemetry,
	}
}

// notify wakes the worker of the destination. A signal that is already pending is enough, so it never blocks.
func (destination *delivery_destination) notify() {
	select {
	case destination.wake <- struct{}{}:
	default:
	}
}

// room_channel returns the channel that the next signal_room closes, and makes it when nobody waits.
func (destination *delivery_destination) room_channel() <-chan struct{} {
	destination.room_mutex.Lock()
	defer destination.room_mutex.Unlock()
	if destination.room == nil {
		destination.room = make(chan struct{})
	}
	return destination.room
}

// signal_room wakes every caller that waits for room: the queue has just lost events. It does nothing when nobody waits.
func (destination *delivery_destination) signal_room() {
	destination.room_mutex.Lock()
	defer destination.room_mutex.Unlock()
	if destination.room != nil {
		close(destination.room)
		destination.room = nil
	}
}

// count_drop counts one event dropped under its reason, which becomes the last reason too.
func (stats *DestinationStats) count_drop(reason string) {
	stats.count_drops(1, reason)
}

// count_drops counts several dropped events under one reason, which becomes the last reason too.
func (stats *DestinationStats) count_drops(count int, reason string) {
	stats.Dropped += count
	stats.LastReason = reason
	stats.DroppedByReason[reason] += count
}

// count changes the counters under their lock.
func (destination *delivery_destination) count(change func(stats *DestinationStats)) {
	destination.stats_mutex.Lock()
	defer destination.stats_mutex.Unlock()
	change(&destination.stats)
}

// send_functions_of returns the Send method of every sink of the kind, in the order given. A nil sink is ignored, so a
// constructor never panics.
func send_functions_of(destinations Destinations, kind DestinationKind) []func(context.Context, []value_objects.Envelope) dtos.SendResult {
	var send_functions []func(context.Context, []value_objects.Envelope) dtos.SendResult
	switch kind {
	case DestinationKindLog:
		for _, sink := range destinations.Log {
			if sink == nil {
				continue
			}
			send_functions = append(send_functions, sink.Send)
		}
	case DestinationKindMetrics:
		for _, sink := range destinations.Metrics {
			if sink == nil {
				continue
			}
			send_functions = append(send_functions, sink.Send)
		}
	case DestinationKindTrace:
		for _, sink := range destinations.Trace {
			if sink == nil {
				continue
			}
			send_functions = append(send_functions, sink.Send)
		}
	case DestinationKindDeviceTelemetry:
		for _, sink := range destinations.DeviceTelemetry {
			if sink == nil {
				continue
			}
			send_functions = append(send_functions, sink.Send)
		}
	case DestinationKindAITelemetry:
		for _, sink := range destinations.AITelemetry {
			if sink == nil {
				continue
			}
			send_functions = append(send_functions, sink.Send)
		}
	default:
		for _, sink := range destinations.Event {
			if sink == nil {
				continue
			}
			send_functions = append(send_functions, sink.Send)
		}
	}
	return send_functions
}

// send_protected calls the function and turns a panic into a result that fails the events of the call (event_count) with
// the reason ReasonSinkPanicked. The panic value is not kept, because it could hold data of the event.
func send_protected(event_count int, send func() dtos.SendResult) (result dtos.SendResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = dtos.SendResult{Failed: event_count, Reason: ReasonSinkPanicked}
		}
	}()
	return send()
}
