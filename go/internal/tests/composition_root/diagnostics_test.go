package composition_root_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/containers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
)

type core_diagnostics_writer struct {
	mutex sync.Mutex
	text  strings.Builder
}

func (writer *core_diagnostics_writer) Write(content []byte) (int, error) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.text.Write(content)
}

func (writer *core_diagnostics_writer) String() string {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.text.String()
}

func Test_a_nil_core_and_a_core_that_was_not_built_answer_the_zero_snapshot(t *testing.T) {
	var nil_core *containers.Core
	if snapshot := nil_core.Diagnostics(); len(snapshot.Destinations) != 0 || snapshot.Channel != (dtos.ChannelDiagnostics{}) {
		t.Fatalf("%+v", snapshot)
	}
	if snapshot := (&containers.Core{}).Diagnostics(); len(snapshot.Destinations) != 0 {
		t.Fatalf("%+v", snapshot)
	}
}

func Test_the_diagnostics_options_reach_the_callbacks_and_the_writer_and_the_snapshot_answers_after_shutdown(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	sink.SetResult(dtos.SendResult{Failed: 1, Reason: "timeout"})
	var mutex sync.Mutex
	var first, second []dtos.DiagnosticEvent
	writer := &core_diagnostics_writer{}
	cfg := service_config("service")
	cfg.Delivery.FlushIntervalMS = 600000
	core := new_core(t, cfg, config.WithLogSink(sink),
		config.WithDiagnosticsCallback(nil),
		config.WithDiagnosticsCallback(func(event dtos.DiagnosticEvent) {
			mutex.Lock()
			defer mutex.Unlock()
			first = append(first, event)
		}),
		config.WithDiagnosticsCallback(func(event dtos.DiagnosticEvent) {
			mutex.Lock()
			defer mutex.Unlock()
			second = append(second, event)
		}),
		config.WithDiagnosticsWriter(nil),
		config.WithDiagnosticsWriter(writer))
	core.Emit(context.Background(), log_input())

	core.Shutdown(context.Background())

	waiting.Eventually(t, waiting.DefaultTimeout, "both callbacks and the writer got the diagnostic", func() bool {
		mutex.Lock()
		defer mutex.Unlock()
		return len(first) == 1 && len(second) == 1 && writer.String() != ""
	})
	mutex.Lock()
	defer mutex.Unlock()
	if first[0].Code != value_objects.DiagnosticCodeExportFailed || first[0].Reason != "timeout" || first[0].Count != 1 || first[0] != second[0] {
		t.Fatalf("%+v and %+v", first[0], second[0])
	}
	if !strings.HasPrefix(writer.String(), "telemetry diagnostics: code=export_failed destination=log/0 reason=timeout count=1") {
		t.Fatalf("%q", writer.String())
	}
	snapshot := core.Diagnostics()
	if len(snapshot.Destinations) != 1 || snapshot.Destinations[0].ExportFailures != 1 || !snapshot.Destinations[0].LastFlush.Done || snapshot.Channel.Published != 1 {
		t.Fatalf("%+v", snapshot)
	}
}
