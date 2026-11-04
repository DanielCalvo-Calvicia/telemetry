package services

import (
	"context"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/batching"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/classification"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/full_queue"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/retry_delay"
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
	// ReasonShutdown is the reason of an event that was dropped because the service was already closed by Shutdown, and of
	// the events that were still queued when the shutdown ended.
	ReasonShutdown = "shutdown"
	// ReasonRetryLimitReached is the reason of the events of a batch that a destination kept failing with a transient failure
	// until the attempts of the retry mode (RetryMaxAttempts) were used up.
	ReasonRetryLimitReached = "retry_limit_reached"
)

// DeliverySettings are the numbers the delivery service works with: the events one queue holds, the events of one batch,
// the milliseconds before a batch that is not full leaves, the milliseconds before a call to a sink gives up, what a full
// queue does (QueuePolicy; empty is drop_newest), the milliseconds a call may wait for room with block_with_timeout
// (BlockTimeoutMS), the class assigned to an event type (EventClasses; nil is none, and then the default class of the
// event applies), the delivery mode (Mode; empty is best_effort: one attempt per batch; retry sends a batch that failed
// with a transient failure again) and the retry settings of that mode: the attempts at most for one batch, the first
// included (RetryMaxAttempts), and the base and the ceiling in milliseconds of the backoff with jitter between two attempts
// (RetryBaseDelayMS, RetryMaxDelayMS). They are already validated (greater than zero, and the batch not greater than the
// queue).
type DeliverySettings struct {
	QueueSize       int
	BatchSize       int
	FlushIntervalMS int
	TimeoutMS       int
	QueuePolicy     value_objects.QueuePolicy
	BlockTimeoutMS  int
	EventClasses    map[string]value_objects.EventClass

	Mode             value_objects.DeliveryMode
	RetryMaxAttempts int
	RetryBaseDelayMS int
	RetryMaxDelayMS  int
}

// DestinationStats is a snapshot of one destination, the minimum needed to observe its worker. Kind and Index say which
// sink it is (Index counts the sinks of the kind, from zero, in the order given). Queued is the events waiting now.
// Batches is the batches taken from the queue and finished (delivered, dropped or failed). Attempts is the calls to the
// sink and Retries the calls beyond the first one of a batch (with best_effort Attempts is Batches and Retries is zero).
// Accepted is the events that were queued; Dropped is the events that were not queued,
// the ones that were removed from the queue to make room and the ones the sink dropped; Failed is the events the sink
// failed to take; Delivered is the events the sink accepted. LastReason is the last reason that was not empty.
// DroppedByReason counts every dropped event per reason (REQ-DLV-008): the ones the queue dropped or displaced
// (queue_full_newest, queue_full_oldest, displaced_by_critical, block_timeout) and the ones a sink reported as dropped,
// under the reason of its result or ReasonUnspecified when it gave none, the ones of a batch that ran out of attempts
// with the retry mode (ReasonRetryLimitReached) and the ones of a batch that was abandoned while it waited to be sent again
// because the service stopped (ReasonShutdown). Delivered counts only the accepted events of the answer that ended a batch.
// Its sum is equal to Dropped. The map is a copy:
// the caller may change it.
type DestinationStats struct {
	Kind            DestinationKind
	Index           int
	Queued          int
	Batches         int
	Attempts        int
	Retries         int
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
	random       outbound.RandomPort
	destinations []*delivery_destination
	by_kind      map[DestinationKind][]*delivery_destination
	stop_signal  chan struct{}
	stop_once    sync.Once

	lifecycle_mutex sync.Mutex
	started         bool
	stopped         bool
	cancel_workers  context.CancelFunc
	workers         sync.WaitGroup

	// admission_mutex guards closed: an offer holds it for reading, Shutdown for writing, so no event is admitted to a
	// queue once Shutdown has closed the service. close_signal is closed at that moment, to wake the callers that wait
	// for room.
	admission_mutex sync.RWMutex
	closed          bool
	close_signal    chan struct{}

	// flush_pending is the number of flushes in progress: while it is above zero the workers send partial batches at
	// once. progress is closed (and set to nil) each time a destination resolves events, to wake the flushes that wait.
	flush_pending  atomic.Int32
	progress_mutex sync.Mutex
	progress       chan struct{}

	// shutdown_mutex is held for the whole Shutdown, so a second call waits for the first and returns its result.
	shutdown_mutex  sync.Mutex
	shutdown_done   bool
	shutdown_result dtos.ShutdownResult
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

	// flow_mutex makes the movements of the events atomic for a flush: an offer (and the removal of an event it displaces),
	// a take and the end of a send. in_flight is the events taken and not yet counted; resolved is the events that left
	// the queue for good (sent, failed or dropped by the sink, or displaced). It is taken before stats_mutex.
	flow_mutex sync.Mutex
	in_flight  int
	resolved   int

	stats_mutex sync.Mutex
	stats       DestinationStats
}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewDeliveryService returns a DeliveryService with one queue, made by new_queue with the queue size, for every sink of
// every kind in the destinations. random is the source of the jitter of the retry mode (a nil one gives 0.5). It starts
// nothing: no goroutine runs until Start is called.
func NewDeliveryService(
	destinations Destinations,
	settings DeliverySettings,
	clock outbound.ClockPort,
	random outbound.RandomPort,
	new_queue func(capacity int) outbound.EventQueuePort,
) *DeliveryService {
	service := &DeliveryService{
		settings:     settings,
		clock:        clock,
		random:       random,
		by_kind:      map[DestinationKind][]*delivery_destination{},
		stop_signal:  make(chan struct{}),
		close_signal: make(chan struct{}),
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
// context cancelled; a sink that ignores its context holds Stop up (Shutdown does not wait for it for ever). A caller of
// Enqueue that waits for room (block_with_timeout) is woken, and its event is dropped with ReasonBlockTimeout. It is
// idempotent and it does not drain: what is still queued is left (Shutdown flushes first).
func (service *DeliveryService) Stop() {
	service.signal_stop()
	service.workers.Wait()
}

// signal_stop tells the workers to stop and the callers that wait for room to give up, without waiting for anything.
func (service *DeliveryService) signal_stop() {
	service.stop_once.Do(func() { close(service.stop_signal) })
	service.lifecycle_mutex.Lock()
	service.stopped = true
	cancel_workers := service.cancel_workers
	service.lifecycle_mutex.Unlock()
	if cancel_workers != nil {
		cancel_workers()
	}
}

// Flush sends what is queued and keeps running. Because there is no synchronous send, the events that are still queued
// when a program exits are lost: a program MUST call Shutdown (not only Flush) before it exits.
//
// For each destination the target is the number of events queued plus the ones being sent at the moment of the call. The
// workers send partial batches at once while a flush is pending, and the call waits on a signal of the workers (never a
// loop that spins) until every destination has finished its target or the time is over. The time is the deadline of ctx;
// without a deadline it is domain.DefaultShutdownTimeoutMilliseconds. Per destination, Sent is the events that the sink
// accepted since the call began (at most the target) and Dropped is the rest of the target: events the sink dropped or
// failed (with the retry mode, after the attempts of the batch), events displaced from the queue and events still queued or in flight at the end of the
// time. Spooled is zero until the spool exists (T-06). A ctx that is already done returns at once with the whole target
// dropped. A flush after Shutdown returns the zero result. It never panics and never returns an error.
func (service *DeliveryService) Flush(ctx context.Context) (result dtos.ShutdownResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = dtos.ShutdownResult{}
		}
	}()
	if service.is_closed() {
		return dtos.ShutdownResult{}
	}
	budget_context, cancel_budget := shutdown_budget(ctx)
	defer cancel_budget()
	return service.flush(budget_context)
}

// Shutdown closes the service and flushes it. Because there is no synchronous send, a program MUST call Shutdown before
// it exits, or the events that are still queued are lost.
//
// It first closes the service (from then on Enqueue drops every event with ReasonShutdown and counts it), then it does a
// Flush with the same time (the deadline of ctx, or domain.DefaultShutdownTimeoutMilliseconds), then it stops the workers
// and waits for them only for the time that is left. A sink that ignores its context cannot hold Shutdown past that time:
// its goroutine is abandoned and it leaks until its Send returns. What is still queued at the end is removed and counted
// as dropped under ReasonShutdown in the stats. A second call returns the first result (it waits for the first if it is
// still running). It never panics and never returns an error.
func (service *DeliveryService) Shutdown(ctx context.Context) (result dtos.ShutdownResult) {
	service.shutdown_mutex.Lock()
	defer service.shutdown_mutex.Unlock()
	if service.shutdown_done {
		return service.shutdown_result
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			result = dtos.ShutdownResult{}
		}
		service.shutdown_done = true
		service.shutdown_result = result
	}()

	budget_context, cancel_budget := shutdown_budget(ctx)
	defer cancel_budget()

	service.admission_mutex.Lock()
	service.closed = true
	close(service.close_signal)
	service.admission_mutex.Unlock()

	result = service.flush(budget_context)

	service.signal_stop()
	workers_done := make(chan struct{})
	go func() {
		service.workers.Wait()
		close(workers_done)
	}()
	select {
	case <-workers_done:
	case <-budget_context.Done():
	}

	for _, destination := range service.destinations {
		destination.drain_for_shutdown()
	}
	return result
}

// Enqueue classifies the envelope (classification.ClassifyEvent, with its event type, its severity and the classes of
// the settings) and offers it to the queue of every destination of the kind. No destination of the kind is Dropped 1
// with ReasonNoDestination. For each destination: an admitted event is Accepted 1 (accepted means queued); when a queued
// event was displaced to make room, the displaced event is counted as dropped in the stats of that destination under its
// reason, and the result of this call does not change because of it; a refused event is Dropped 1 with the reason of the
// drop (ReasonQueueFullNewest, or ReasonBlockTimeout after a wait). The result is the sum over the destinations and
// Reason is the first one that is not empty. After Shutdown the event is Dropped 1 with ReasonShutdown and the drop is
// counted in the stats of every destination of the kind.
//
// A call returns at once, except in ONE case: with the policy block_with_timeout and a full queue that holds nothing
// the event may displace, the call WAITS for room, at most BlockTimeoutMS milliseconds (a real wait on a signal of the
// worker and a timer, never a loop that spins), and offers again each time the worker takes events. If room appears the
// event is Accepted 1; if the time is over, the service is stopped or ctx is done, it is Dropped 1 with
// ReasonBlockTimeout. With any other policy the call never waits.
func (service *DeliveryService) Enqueue(ctx context.Context, kind DestinationKind, envelope value_objects.Envelope) dtos.EmitResult {
	if closed_result, is_closed := service.RejectIfClosed(kind); is_closed {
		return closed_result
	}
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

// RejectIfClosed says whether Shutdown has closed the service. When it has, the result is Dropped 1 with ReasonShutdown
// for any event (a kind with no destination included) and the drop is counted under ReasonShutdown in the stats of every
// destination of the kind, when there are some. The pipeline calls it before it builds the event.
func (service *DeliveryService) RejectIfClosed(kind DestinationKind) (dtos.EmitResult, bool) {
	if !service.is_closed() {
		return dtos.EmitResult{}, false
	}
	for _, destination := range service.by_kind[kind] {
		destination.count(func(stats *DestinationStats) { stats.count_drop(ReasonShutdown) })
	}
	return dtos.EmitResult{Dropped: 1, Reason: ReasonShutdown}, true
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
		return service.try_offer(destination, envelope, event_class, arrival_ms)
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
		case <-service.close_signal:
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

// try_offer makes one offer to the queue, unless the service is closed: then nothing is admitted and the reason is
// ReasonShutdown. The closed check and the offer are one step for Shutdown (admission_mutex), and the offer and the count
// of a displaced event are one step for a flush (flow_mutex).
func (service *DeliveryService) try_offer(
	destination *delivery_destination,
	envelope value_objects.Envelope,
	event_class value_objects.EventClass,
	arrival_ms int64,
) outbound.OfferResult {
	service.admission_mutex.RLock()
	defer service.admission_mutex.RUnlock()
	if service.closed {
		return outbound.OfferResult{Reason: ReasonShutdown}
	}
	destination.flow_mutex.Lock()
	offer_result := destination.queue.Offer(envelope, event_class, arrival_ms, service.settings.QueuePolicy)
	if offer_result.Displaced != nil {
		destination.resolved++
	}
	destination.flow_mutex.Unlock()
	if offer_result.Displaced != nil {
		service.signal_progress()
	}
	return offer_result
}

// ----- THE FLUSH ------------------------------------------------------------

// flush_target is what a flush knows about one destination when it begins: the events to finish (queued plus being sent),
// and the counters at that moment.
type flush_target struct {
	destination      *delivery_destination
	target           int
	resolved_before  int
	delivered_before int
}

// flush does the work of Flush and of Shutdown with a context that already carries the time. It does not check whether
// the service is closed.
func (service *DeliveryService) flush(budget_context context.Context) dtos.ShutdownResult {
	// The targets are read BEFORE the flush is marked as pending: once it is pending the workers send partial batches, and
	// a batch sent between the mark and the snapshot would be left out of the target.
	targets := make([]flush_target, 0, len(service.destinations))
	for _, destination := range service.destinations {
		targets = append(targets, destination.flush_target())
	}
	service.flush_pending.Add(1)
	defer service.flush_pending.Add(-1)
	for _, destination := range service.destinations {
		destination.notify()
	}

	var result dtos.ShutdownResult
	if budget_context.Err() != nil || !service.is_running() {
		// Nobody can finish the events in time (or at all): all of them are dropped, none is sent.
		for _, target := range targets {
			result.Dropped += target.target
		}
		return result
	}

	for !service.targets_finished(targets) {
		progress := service.progress_channel()
		if service.targets_finished(targets) {
			break
		}
		select {
		case <-progress:
		case <-budget_context.Done():
			return service.tally(targets)
		}
	}
	return service.tally(targets)
}

// tally is the result of a flush: for each target the events the sink accepted since the flush began, at most the target,
// are sent and the rest of the target is dropped.
func (service *DeliveryService) tally(targets []flush_target) dtos.ShutdownResult {
	var result dtos.ShutdownResult
	for _, target := range targets {
		sent := min(target.destination.delivered()-target.delivered_before, target.target)
		sent = max(sent, 0)
		result.Sent += sent
		result.Dropped += target.target - sent
	}
	return result
}

// targets_finished says whether every destination has resolved as many events as its target since the flush began.
func (service *DeliveryService) targets_finished(targets []flush_target) bool {
	for _, target := range targets {
		if target.destination.resolved_count()-target.resolved_before < target.target {
			return false
		}
	}
	return true
}

// is_closed says whether Shutdown has closed the service.
func (service *DeliveryService) is_closed() bool {
	service.admission_mutex.RLock()
	defer service.admission_mutex.RUnlock()
	return service.closed
}

// is_running says whether the workers have been started and not stopped.
func (service *DeliveryService) is_running() bool {
	service.lifecycle_mutex.Lock()
	defer service.lifecycle_mutex.Unlock()
	return service.started && !service.stopped
}

// progress_channel returns the channel that the next signal_progress closes, and makes it when nobody waits.
func (service *DeliveryService) progress_channel() <-chan struct{} {
	service.progress_mutex.Lock()
	defer service.progress_mutex.Unlock()
	if service.progress == nil {
		service.progress = make(chan struct{})
	}
	return service.progress
}

// signal_progress wakes the flushes that wait: a destination has resolved events. It does nothing when nobody waits.
func (service *DeliveryService) signal_progress() {
	service.progress_mutex.Lock()
	defer service.progress_mutex.Unlock()
	if service.progress != nil {
		close(service.progress)
		service.progress = nil
	}
}

// shutdown_budget returns the context a flush or a shutdown works with: ctx itself when it has a deadline, and ctx with
// the default timeout when it has none. A nil ctx is a background context.
func shutdown_budget(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, has_deadline := ctx.Deadline(); has_deadline {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, time.Duration(domain.DefaultShutdownTimeoutMilliseconds)*time.Millisecond)
}

// flush_target reads the target and the counters of the destination in one step with the offers, the takes and the ends
// of the sends.
func (destination *delivery_destination) flush_target() flush_target {
	destination.flow_mutex.Lock()
	defer destination.flow_mutex.Unlock()
	return flush_target{
		destination:      destination,
		target:           destination.queue.Len() + destination.in_flight,
		resolved_before:  destination.resolved,
		delivered_before: destination.delivered(),
	}
}

// resolved_count returns the events that left the queue for good.
func (destination *delivery_destination) resolved_count() int {
	destination.flow_mutex.Lock()
	defer destination.flow_mutex.Unlock()
	return destination.resolved
}

// delivered returns the events the sink has accepted.
func (destination *delivery_destination) delivered() int {
	destination.stats_mutex.Lock()
	defer destination.stats_mutex.Unlock()
	return destination.stats.Delivered
}

// drain_for_shutdown removes what is still queued and counts it as dropped with ReasonShutdown. The events were already
// counted as dropped in the result of the shutdown.
func (destination *delivery_destination) drain_for_shutdown() {
	destination.flow_mutex.Lock()
	defer destination.flow_mutex.Unlock()
	left := len(destination.queue.Take(destination.queue.Len()))
	if left == 0 {
		return
	}
	destination.resolved += left
	destination.count(func(stats *DestinationStats) { stats.count_drops(left, ReasonShutdown) })
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
	flush_interval_ms := int64(service.settings.FlushIntervalMS)
	if service.flush_pending.Load() > 0 {
		// A flush is waiting: a batch that is not full leaves at once. PlanBatch itself does not change.
		flush_interval_ms = 0
	}
	return batching.PlanBatch(now_ms, queued, oldest_arrival_ms, service.settings.BatchSize, flush_interval_ms)
}

// send_batch takes the events from the queue, wakes the callers that wait for room and sends them to the sink with the
// timeout of the settings, as the delivery mode says (REQ-DLV-003). With best_effort (and any mode that is not retry) the
// batch is sent ONCE, whatever the answer. With retry, an answer with failed events and no dropped ones is a transient
// failure: the whole batch is sent again after the delay of retry_delay_for while the attempts are below
// RetryMaxAttempts, and at the limit the failed events are dropped with ReasonRetryLimitReached. Any other answer ends
// the batch. Only the accepted events of the answer that ends the batch count as delivered. The wait between two
// attempts is a real wait that Stop wakes: the batch then counts as dropped with ReasonShutdown. While it waits the
// events stay in flight, so a flush waits for them. A panic of the sink is Failed with ReasonSinkPanicked.
func (service *DeliveryService) send_batch(ctx context.Context, destination *delivery_destination, take int) {
	if ctx.Err() != nil {
		return
	}
	destination.flow_mutex.Lock()
	batch := destination.queue.Take(take)
	destination.in_flight += len(batch)
	destination.flow_mutex.Unlock()
	if len(batch) == 0 {
		return
	}
	destination.signal_room()

	is_retry := service.settings.Mode == value_objects.DeliveryModeRetry
	max_attempts := 1
	if is_retry {
		max_attempts = service.retry_max_attempts()
	}
	for attempt := 0; ; attempt++ {
		result := service.attempt_send(ctx, destination, batch, attempt)
		if is_retry && result.Failed > 0 && result.Dropped == 0 {
			jitter := 0.0
			if attempt+1 < max_attempts {
				jitter = service.random_number() // the source is asked only when there is a wait to compute
			}
			delay_ms, retry := retry_delay.RetryDelay(attempt, max_attempts, true, result.RetryAfterMS, result.RetryAfterMS > 0,
				service.retry_base_delay_ms(), service.retry_max_delay_ms(), jitter)
			if retry {
				if wait_or_stop(ctx, time.Duration(delay_ms)*time.Millisecond) {
					continue
				}
				service.finish_batch(destination, len(batch), dtos.SendResult{Dropped: len(batch), Reason: ReasonShutdown}, false)
				return
			}
		}
		service.finish_batch(destination, len(batch), result, is_retry)
		return
	}
}

// attempt_send makes one call to the sink with the timeout of the settings and counts it (attempt is 0 for the first call
// of a batch).
func (service *DeliveryService) attempt_send(ctx context.Context, destination *delivery_destination, batch []value_objects.Envelope, attempt int) dtos.SendResult {
	send_context, cancel_send := context.WithTimeout(ctx, time.Duration(service.settings.TimeoutMS)*time.Millisecond)
	defer cancel_send()
	destination.count(func(stats *DestinationStats) {
		stats.Attempts++
		if attempt > 0 {
			stats.Retries++
		}
	})
	return send_protected(len(batch), func() dtos.SendResult { return destination.send(send_context, batch) })
}

// finish_batch adds the answer that ends a batch to the counters and releases its events. The counters and the end of the
// send are one step for a flush: it reads both to know what the sink accepted. When with_retry is true the failed events
// of the answer are the ones that ran out of attempts: they are dropped with ReasonRetryLimitReached instead of failed.
func (service *DeliveryService) finish_batch(destination *delivery_destination, batch_size int, result dtos.SendResult, with_retry bool) {
	destination.flow_mutex.Lock()
	destination.count(func(stats *DestinationStats) {
		stats.Batches++
		stats.Delivered += result.Accepted
		if with_retry && result.Failed > 0 && result.Dropped == 0 {
			stats.count_drops(result.Failed, ReasonRetryLimitReached)
		} else {
			stats.Failed += result.Failed
		}
		if result.Dropped > 0 {
			drop_reason := result.Reason
			if drop_reason == "" {
				drop_reason = ReasonUnspecified
			}
			stats.count_drops(result.Dropped, drop_reason)
		} else if result.Reason != "" && !(with_retry && result.Failed > 0) {
			stats.LastReason = result.Reason
		}
	})
	destination.in_flight -= batch_size
	destination.resolved += batch_size
	destination.flow_mutex.Unlock()
	service.signal_progress()
}

// retry_max_attempts is the attempts at most for one batch with the retry mode (the first included); a setting that is
// not positive (the settings are validated, so it is a service that was built by hand) is the default.
func (service *DeliveryService) retry_max_attempts() int {
	if service.settings.RetryMaxAttempts < 1 {
		return domain.DefaultRetryMaxAttempts
	}
	return service.settings.RetryMaxAttempts
}

// retry_base_delay_ms is the base of the backoff in milliseconds, or the default when the setting is not positive.
func (service *DeliveryService) retry_base_delay_ms() int {
	if service.settings.RetryBaseDelayMS < 1 {
		return domain.DefaultRetryBaseDelayMilliseconds
	}
	return service.settings.RetryBaseDelayMS
}

// retry_max_delay_ms is the ceiling of the backoff in milliseconds, or the default when the setting is not positive.
func (service *DeliveryService) retry_max_delay_ms() int {
	if service.settings.RetryMaxDelayMS < 1 {
		return domain.DefaultRetryMaxDelayMilliseconds
	}
	return service.settings.RetryMaxDelayMS
}

// random_number is the next number of the random source, in [0, 1]; 0.5 when the service has no source.
func (service *DeliveryService) random_number() float64 {
	if service.random == nil {
		return 0.5
	}
	return service.random.Float64()
}

// wait_or_stop waits for the duration and returns true, or returns false at once when the context is done first (the
// service is stopped). It waits on a timer and the context, never in a loop; the timer is always stopped. A duration that
// is not positive does not wait, and returns whether the context is still alive.
func wait_or_stop(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ctx.Err() == nil
	case <-ctx.Done():
		return false
	}
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
