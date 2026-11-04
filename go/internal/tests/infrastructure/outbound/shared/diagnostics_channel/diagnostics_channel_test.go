package diagnostics_channel_test

import (
	"errors"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/diagnostics_channel"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
)

// collector is a callback that keeps what it receives.
type collector struct {
	mutex  sync.Mutex
	events []dtos.DiagnosticEvent
}

func (collected *collector) callback(event dtos.DiagnosticEvent) {
	collected.mutex.Lock()
	defer collected.mutex.Unlock()
	collected.events = append(collected.events, event)
}

func (collected *collector) snapshot() []dtos.DiagnosticEvent {
	collected.mutex.Lock()
	defer collected.mutex.Unlock()
	return append([]dtos.DiagnosticEvent(nil), collected.events...)
}

func (collected *collector) count() int { return len(collected.snapshot()) }

// lines_writer is an io.Writer that keeps what it receives.
type lines_writer struct {
	mutex sync.Mutex
	text  strings.Builder
}

func (writer *lines_writer) Write(content []byte) (int, error) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.text.Write(content)
}

func (writer *lines_writer) String() string {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.text.String()
}

// failing_writer answers an error, or panics.
type failing_writer struct{ should_panic bool }

func (writer failing_writer) Write(_ []byte) (int, error) {
	if writer.should_panic {
		panic("writer panic with a secret value")
	}
	return 0, errors.New("the writer failed")
}

func diagnostic(code value_objects.DiagnosticCode, reason string, count int) dtos.DiagnosticEvent {
	return dtos.DiagnosticEvent{Code: code, Kind: "log", Index: 0, Reason: reason, Count: count}
}

func started_channel(test *testing.T, clock outbound.ClockPort, callbacks []func(dtos.DiagnosticEvent), writer *lines_writer) outbound.DiagnosticsChannelPort {
	test.Helper()
	var channel outbound.DiagnosticsChannelPort
	if writer == nil {
		channel = diagnostics_channel.NewDiagnosticsChannel(clock, callbacks, nil)
	} else {
		channel = diagnostics_channel.NewDiagnosticsChannel(clock, callbacks, writer)
	}
	channel.Start()
	test.Cleanup(channel.Stop)
	return channel
}

func Test_a_storm_of_a_thousand_diagnostics_in_one_window_lets_ten_through_and_the_next_window_reports_the_suppressed(t *testing.T) {
	manual_clock := fakes.NewManualClock()
	received := &collector{}
	channel := started_channel(t, manual_clock, []func(dtos.DiagnosticEvent){received.callback}, nil)

	for storm := 0; storm < 1000; storm++ {
		channel.Publish(diagnostic(value_objects.DiagnosticCodeExportFailed, "timeout", 1))
	}

	waiting.Eventually(t, waiting.DefaultTimeout, "ten diagnostics reached the callback", func() bool { return received.count() == 10 })
	counters := channel.Counters()
	if counters.Published != 10 || counters.Suppressed != 990 || counters.Dropped != 0 || counters.CallbackErrors != 0 {
		t.Fatalf("%+v", counters)
	}
	for _, event := range received.snapshot() {
		if event.Suppressed != 0 {
			t.Fatalf("a diagnostic of the first window carries suppressed %d", event.Suppressed)
		}
	}

	manual_clock.AdvanceMilliseconds(1000)
	channel.Publish(diagnostic(value_objects.DiagnosticCodeExportFailed, "timeout", 1))
	waiting.Eventually(t, waiting.DefaultTimeout, "the first diagnostic of the next window arrived", func() bool { return received.count() == 11 })
	if last := received.snapshot()[10]; last.Suppressed != 990 {
		t.Fatalf("the next diagnostic carries suppressed %d, expected 990", last.Suppressed)
	}
	if counters = channel.Counters(); counters.Published != 11 || counters.Suppressed != 990 {
		t.Fatalf("%+v", counters)
	}
}

func Test_the_callbacks_are_called_in_the_order_given_and_a_nil_callback_is_ignored(t *testing.T) {
	var order []string
	var order_mutex sync.Mutex
	record := func(name string) func(dtos.DiagnosticEvent) {
		return func(event dtos.DiagnosticEvent) {
			order_mutex.Lock()
			defer order_mutex.Unlock()
			order = append(order, name+":"+event.Reason)
		}
	}
	channel := started_channel(t, fakes.NewManualClock(), []func(dtos.DiagnosticEvent){record("first"), nil, record("second")}, nil)

	channel.Publish(diagnostic(value_objects.DiagnosticCodeEventsDropped, "one", 1))
	channel.Publish(diagnostic(value_objects.DiagnosticCodeEventsDropped, "two", 1))

	waiting.Eventually(t, waiting.DefaultTimeout, "four calls", func() bool {
		order_mutex.Lock()
		defer order_mutex.Unlock()
		return len(order) == 4
	})
	order_mutex.Lock()
	defer order_mutex.Unlock()
	if strings.Join(order, ",") != "first:one,second:one,first:two,second:two" {
		t.Fatalf("%v", order)
	}
}

func Test_the_writer_receives_one_line_per_diagnostic_in_the_exact_format(t *testing.T) {
	writer := &lines_writer{}
	channel := started_channel(t, fakes.NewManualClock(), nil, writer)

	channel.Publish(dtos.DiagnosticEvent{Code: value_objects.DiagnosticCodeExportFailed, Kind: "metrics", Index: 2, Reason: "status_503", Count: 7})

	waiting.Eventually(t, waiting.DefaultTimeout, "the line was written", func() bool { return writer.String() != "" })
	expected := "telemetry diagnostics: code=export_failed destination=metrics/2 reason=status_503 count=7 suppressed=0\n"
	if writer.String() != expected {
		t.Fatalf("%q", writer.String())
	}
}

func Test_a_writer_that_fails_or_panics_and_a_callback_that_panics_are_contained_and_counted_and_create_nothing(t *testing.T) {
	for name, writer := range map[string]failing_writer{"error": {should_panic: false}, "panic": {should_panic: true}} {
		t.Run("writer_"+name, func(t *testing.T) {
			received := &collector{}
			channel := diagnostics_channel.NewDiagnosticsChannel(fakes.NewManualClock(), []func(dtos.DiagnosticEvent){received.callback}, writer)
			channel.Start()
			t.Cleanup(channel.Stop)

			channel.Publish(diagnostic(value_objects.DiagnosticCodeEventsDropped, "x", 1))

			waiting.Eventually(t, waiting.DefaultTimeout, "the writer failure was counted", func() bool { return channel.Counters().CallbackErrors == 1 })
			if received.count() != 1 || channel.Counters().Published != 1 {
				t.Fatalf("callbacks %d, counters %+v", received.count(), channel.Counters())
			}
		})
	}

	t.Run("callback_panic", func(t *testing.T) {
		received := &collector{}
		panicking := func(dtos.DiagnosticEvent) { panic("callback panic with a secret value") }
		channel := started_channel(t, fakes.NewManualClock(), []func(dtos.DiagnosticEvent){panicking, received.callback}, nil)

		channel.Publish(diagnostic(value_objects.DiagnosticCodeEventsDropped, "x", 1))
		channel.Publish(diagnostic(value_objects.DiagnosticCodeEventsDropped, "y", 1))

		waiting.Eventually(t, waiting.DefaultTimeout, "both diagnostics reached the second callback", func() bool { return received.count() == 2 })
		waiting.Eventually(t, waiting.DefaultTimeout, "two panics counted", func() bool { return channel.Counters().CallbackErrors == 2 })
		if counters := channel.Counters(); counters.Published != 2 || counters.Suppressed != 0 {
			t.Fatalf("a panic of a callback produced another diagnostic: %+v", counters)
		}
	})
}

func Test_a_callback_that_hangs_blocks_only_the_dispatcher_and_the_full_queue_drops_without_waiting(t *testing.T) {
	manual_clock := fakes.NewManualClock()
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	var delivered collector
	hanging := func(event dtos.DiagnosticEvent) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		delivered.callback(event)
	}
	channel := diagnostics_channel.NewDiagnosticsChannel(manual_clock, []func(dtos.DiagnosticEvent){hanging}, nil)
	channel.Start()
	released := false
	release_callbacks := func() {
		if !released {
			released = true
			close(release)
		}
	}
	t.Cleanup(func() { release_callbacks(); channel.Stop() })

	published_within := make(chan struct{})
	go func() {
		defer close(published_within)
		for position := 0; position < 100; position++ {
			manual_clock.AdvanceMilliseconds(1000) // a new window each time: the rate limit never refuses
			channel.Publish(diagnostic(value_objects.DiagnosticCodeEventsDropped, "x", 1))
			if position == 0 {
				<-entered
			}
		}
	}()
	select {
	case <-published_within:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish waited for a callback that hangs")
	}

	counters := channel.Counters()
	if counters.Dropped != 100-1-64 || counters.Published != 65 || counters.Suppressed != 0 {
		t.Fatalf("%+v: one in the callback, 64 in the queue and the rest dropped", counters)
	}

	stopped_within := make(chan struct{})
	go func() {
		channel.Stop()
		close(stopped_within)
	}()
	select {
	case <-stopped_within:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop waited for a callback that hangs")
	}
	channel.Publish(diagnostic(value_objects.DiagnosticCodeEventsDropped, "after stop", 1))
	if after_stop := channel.Counters(); after_stop != counters {
		t.Fatalf("Publish after Stop changed the counters: %+v", after_stop)
	}

	release_callbacks()
	waiting.Eventually(t, waiting.DefaultTimeout, "what was queued is delivered before the dispatcher ends", func() bool { return delivered.count() == 65 })
}

func Test_a_channel_with_no_callback_and_no_writer_starts_no_goroutine_but_counts(t *testing.T) {
	manual_clock := fakes.NewManualClock()
	channel := diagnostics_channel.NewDiagnosticsChannel(manual_clock, nil, nil)
	before := runtime.NumGoroutine()

	channel.Start()
	for storm := 0; storm < 30; storm++ {
		channel.Publish(diagnostic(value_objects.DiagnosticCodeEventsDropped, "x", 1))
	}

	waiting.StaysTrue(t, 30*time.Millisecond, "no goroutine was started", func() bool { return runtime.NumGoroutine() <= before })
	if counters := channel.Counters(); counters.Published != 10 || counters.Suppressed != 20 || counters.Dropped != 0 {
		t.Fatalf("%+v", counters)
	}
	channel.Stop()
}

func Test_nothing_starts_in_the_constructor_and_the_dispatcher_ends_on_stop(t *testing.T) {
	before := runtime.NumGoroutine()
	channel := diagnostics_channel.NewDiagnosticsChannel(fakes.NewManualClock(), []func(dtos.DiagnosticEvent){func(dtos.DiagnosticEvent) {}}, nil)
	waiting.StaysTrue(t, 30*time.Millisecond, "the constructor started no goroutine", func() bool { return runtime.NumGoroutine() <= before })

	channel.Start()
	channel.Start()
	waiting.Eventually(t, waiting.DefaultTimeout, "one dispatcher runs", func() bool { return runtime.NumGoroutine() == before+1 })

	channel.Stop()
	channel.Stop()
	waiting.Eventually(t, waiting.DefaultTimeout, "the dispatcher ended", func() bool { return runtime.NumGoroutine() <= before })

	channel.Start() // after Stop it does nothing
	waiting.StaysTrue(t, 30*time.Millisecond, "no dispatcher after Stop", func() bool { return runtime.NumGoroutine() <= before })
}

func Test_a_nil_channel_does_nothing_and_a_nil_clock_reads_zero(t *testing.T) {
	var nil_channel *diagnostics_channel.DiagnosticsChannel
	nil_channel.Publish(dtos.DiagnosticEvent{})
	nil_channel.Start()
	nil_channel.Stop()
	if counters := nil_channel.Counters(); counters != (dtos.ChannelDiagnostics{}) {
		t.Fatalf("%+v", counters)
	}

	received := &collector{}
	channel := started_channel(t, nil, []func(dtos.DiagnosticEvent){received.callback}, nil)
	channel.Publish(diagnostic(value_objects.DiagnosticCodeEventsDropped, "x", 1))
	waiting.Eventually(t, waiting.DefaultTimeout, "the diagnostic arrived", func() bool { return received.count() == 1 })
}

func Test_the_reason_is_cleaned_before_it_reaches_a_callback_or_the_writer(t *testing.T) {
	received := &collector{}
	writer := &lines_writer{}
	channel := started_channel(t, fakes.NewManualClock(), []func(dtos.DiagnosticEvent){received.callback}, writer)

	channel.Publish(diagnostic(value_objects.DiagnosticCodeExportFailed, "Bad reason: https://host.example/path?token=abc", 1))
	channel.Publish(diagnostic(value_objects.DiagnosticCodeExportFailed, "", 1))

	waiting.Eventually(t, waiting.DefaultTimeout, "two diagnostics", func() bool { return received.count() == 2 })
	events := received.snapshot()
	if events[0].Reason != "Bad_reason:_https:__host.example_path_token_abc" || events[1].Reason != "unspecified" {
		t.Fatalf("%q and %q", events[0].Reason, events[1].Reason)
	}
	waiting.Eventually(t, waiting.DefaultTimeout, "two lines", func() bool { return strings.Count(writer.String(), "\n") == 2 })
	if strings.Contains(writer.String(), "//") || strings.Contains(writer.String(), "?") || strings.Contains(writer.String(), "=abc") {
		t.Fatalf("%q", writer.String())
	}
}
