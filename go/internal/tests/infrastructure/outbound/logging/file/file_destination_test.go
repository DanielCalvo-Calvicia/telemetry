package file_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file/error"
)

// The destination implements both ports.
var (
	_ outbound.LogSinkPort   = (*file.FileDestination)(nil)
	_ outbound.EventSinkPort = (*file.FileDestination)(nil)
)

// log_event builds a log event of the given severity and message.
func log_event(severity value_objects.Severity, message string) value_objects.Envelope {
	return value_objects.Envelope{
		SchemaVersion: "0.1",
		EventId:       "01900000-0000-7000-8000-000000000001",
		EventType:     value_objects.EventTypeLogEntryRecorded,
		OccurredAt:    "2026-10-28T10:00:00.123456Z",
		RecordedAt:    "2026-10-28T10:00:00.123456Z",
		Severity:      severity,
		Data:          map[string]any{"category": "c", "message": message},
	}
}

// settings_in builds settings over a directory, with big limits unless the caller changes them.
func settings_in(directory string) file.Settings {
	settings := file.DefaultSettings()
	settings.Directory = directory
	return settings
}

// read_text returns the content of a file of the directory.
func read_text(t *testing.T, path string) string {
	t.Helper()
	content, read_error := os.ReadFile(path)
	if read_error != nil {
		t.Fatal(read_error)
	}
	return string(content)
}

func Test_default_settings_have_no_directory_and_are_refused_until_one_is_given(t *testing.T) {
	settings := file.DefaultSettings()
	if settings.Directory != "" || settings.FileName != "telemetry.jsonl" || settings.MinimumSeverity != "INFO" ||
		settings.MaxFileBytes != 10485760 || settings.MaxFiles != 5 || settings.QuotaBytes != 52428800 {
		t.Fatalf("the defaults are %+v", settings)
	}
	if file.ValidateSettings(settings) == nil {
		t.Fatal("the defaults have no directory and must be refused")
	}
	settings.Directory = "logs"
	if validation_error := file.ValidateSettings(settings); validation_error != nil {
		t.Fatal(validation_error)
	}
}

func Test_validation_refuses_control_characters_and_never_shows_the_value(t *testing.T) {
	settings := settings_in("lo\x00gs")
	settings.FileName = "secret-name\x1f"
	validation_error := file.ValidateSettings(settings)
	var typed_error *config.ValidationError
	if !errors.As(validation_error, &typed_error) || len(typed_error.Problems) != 2 ||
		typed_error.Problems[0].Field != "directory" || typed_error.Problems[1].Field != "file_name" {
		t.Fatalf("expected the problems directory and file_name, got %v", validation_error)
	}
	if strings.Contains(validation_error.Error(), "secret-name") || strings.Contains(validation_error.Error(), "lo\x00gs") {
		t.Fatalf("a configured value is in the error: %q", validation_error.Error())
	}
	// A dot and a delete character are refused too.
	for _, bad_name := range []string{".", "a\x7fb"} {
		bad_settings := settings_in("logs")
		bad_settings.FileName = bad_name
		if file.ValidateSettings(bad_settings) == nil {
			t.Errorf("the file name %q must be refused", bad_name)
		}
	}
	// A quota below the file size is not reported when the file size itself is invalid.
	only_size := settings_in("logs")
	only_size.MaxFileBytes = 0
	only_size.QuotaBytes = 1
	if !errors.As(file.ValidateSettings(only_size), &typed_error) || len(typed_error.Problems) != 1 || typed_error.Problems[0].Field != "max_file_bytes" {
		t.Fatalf("expected one problem on max_file_bytes, got %v", file.ValidateSettings(only_size))
	}
}

func Test_a_missing_directory_is_created_and_the_files_are_for_the_owner_only(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "a", "b")
	destination, build_error := file.NewFileDestination(settings_in(directory))
	if build_error != nil {
		t.Fatal(build_error)
	}
	result := destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "one")})
	if result.Accepted != 1 || result.Failed != 0 {
		t.Fatalf("the result is %+v", result)
	}
	base_path := filepath.Join(directory, "telemetry.jsonl")
	if strings.Count(read_text(t, base_path), "\n") != 1 {
		t.Fatal("expected one line")
	}
	if runtime.GOOS == "windows" {
		return
	}
	for path, expected_mode := range map[string]os.FileMode{base_path: 0o600, directory: 0o700} {
		info, stat_error := os.Stat(path)
		if stat_error != nil {
			t.Fatal(stat_error)
		}
		if info.Mode().Perm()&0o077 != 0 || info.Mode().Perm()&expected_mode != expected_mode {
			t.Errorf("%s has the mode %v, expected %v at most for the group and others none", path, info.Mode().Perm(), expected_mode)
		}
	}
}

func Test_a_directory_that_is_a_file_is_a_problem_on_the_directory_with_no_path_in_it(t *testing.T) {
	occupied := filepath.Join(t.TempDir(), "occupied-by-a-file")
	if write_error := os.WriteFile(occupied, []byte("x"), 0o600); write_error != nil {
		t.Fatal(write_error)
	}
	destination, build_error := file.NewFileDestination(settings_in(occupied))
	if destination != nil {
		t.Fatal("no destination expected")
	}
	var typed_error *config.ValidationError
	if !errors.As(build_error, &typed_error) || len(typed_error.Problems) != 1 || typed_error.Problems[0].Field != "directory" ||
		typed_error.Problems[0].Code != "config_field_invalid" || typed_error.Problems[0].Reason != "the directory cannot be written" {
		t.Fatalf("expected one problem on the directory, got %v", build_error)
	}
	if strings.Contains(build_error.Error(), "occupied-by-a-file") || strings.Contains(build_error.Error(), os.TempDir()) {
		t.Fatalf("the error holds a path: %q", build_error.Error())
	}
}

func Test_a_base_name_that_is_a_directory_makes_the_probe_fail(t *testing.T) {
	directory := t.TempDir()
	if mkdir_error := os.Mkdir(filepath.Join(directory, "telemetry.jsonl"), 0o700); mkdir_error != nil {
		t.Fatal(mkdir_error)
	}
	_, build_error := file.NewFileDestination(settings_in(directory))
	var typed_error *config.ValidationError
	if !errors.As(build_error, &typed_error) || len(typed_error.Problems) != 1 || typed_error.Problems[0].Field != "directory" {
		t.Fatalf("expected one problem on the directory, got %v", build_error)
	}
}

func Test_a_write_that_fails_is_counted_and_the_next_events_are_still_tried(t *testing.T) {
	directory := t.TempDir()
	destination, build_error := file.NewFileDestination(settings_in(directory))
	if build_error != nil {
		t.Fatal(build_error)
	}
	// The current file becomes a directory after the destination was built: every append fails.
	base_path := filepath.Join(directory, "telemetry.jsonl")
	if remove_error := os.Remove(base_path); remove_error != nil {
		t.Fatal(remove_error)
	}
	if mkdir_error := os.Mkdir(base_path, 0o700); mkdir_error != nil {
		t.Fatal(mkdir_error)
	}
	events := []value_objects.Envelope{
		log_event(value_objects.SeverityInfo, "one"),
		log_event(value_objects.SeverityDebug, "below the minimum"),
		log_event(value_objects.SeverityError, "two"),
	}
	result := destination.Send(context.Background(), events)
	if result.Accepted != 1 || result.Failed != 2 || result.Dropped != 0 || result.Reason != "write_failed" {
		t.Fatalf("the result is %+v", result)
	}
	if counters := destination.Counters(); counters.WriteErrors != 2 {
		t.Fatalf("the counters are %+v", counters)
	}
}

func Test_a_directory_that_disappears_makes_the_write_fail_without_a_panic(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "gone")
	destination, build_error := file.NewFileDestination(settings_in(directory))
	if build_error != nil {
		t.Fatal(build_error)
	}
	if remove_error := os.RemoveAll(directory); remove_error != nil {
		t.Fatal(remove_error)
	}
	result := destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "one")})
	if result.Failed != 1 || result.Reason != "write_failed" {
		t.Fatalf("the result is %+v", result)
	}
}

func Test_a_rotation_that_fails_fails_the_event_and_is_counted(t *testing.T) {
	directory := t.TempDir()
	settings := settings_in(directory)
	settings.MaxFileBytes = 100
	settings.QuotaBytes = 100000
	settings.MaxFiles = 2
	destination, build_error := file.NewFileDestination(settings)
	if build_error != nil {
		t.Fatal(build_error)
	}
	// A line is bigger than 100 bytes, so the first one is written to the empty file and the second one needs a rotation.
	if result := destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "first")}); result.Accepted != 1 {
		t.Fatalf("the first result is %+v", result)
	}
	// The oldest rotated name is a directory that is not empty: it cannot be removed.
	blocker := filepath.Join(directory, "telemetry.jsonl.1")
	if mkdir_error := os.MkdirAll(filepath.Join(blocker, "inside"), 0o700); mkdir_error != nil {
		t.Fatal(mkdir_error)
	}
	result := destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "second")})
	if result.Failed != 1 || result.Reason != "rotation_failed" {
		t.Fatalf("the result is %+v", result)
	}
	counters := destination.Counters()
	if counters.RotationErrors != 1 || counters.FilesRotated != 0 || counters.WriteErrors != 0 {
		t.Fatalf("the counters are %+v", counters)
	}
	if strings.Contains(read_text(t, filepath.Join(directory, "telemetry.jsonl")), "second") {
		t.Fatal("an event whose rotation failed must not be written")
	}
}

func Test_a_deletion_for_the_quota_that_fails_is_counted_and_does_not_fail_the_event(t *testing.T) {
	directory := t.TempDir()
	settings := settings_in(directory)
	settings.MaxFileBytes = 100
	settings.QuotaBytes = 100
	settings.MaxFiles = 3
	destination, build_error := file.NewFileDestination(settings)
	if build_error != nil {
		t.Fatal(build_error)
	}
	// A rotated name that is a directory that is not empty cannot be deleted; the current file alone is over the quota.
	if mkdir_error := os.MkdirAll(filepath.Join(directory, "telemetry.jsonl.1", "inside"), 0o700); mkdir_error != nil {
		t.Fatal(mkdir_error)
	}
	result := destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "one")})
	if result.Accepted != 1 || result.Failed != 0 {
		t.Fatalf("the result is %+v", result)
	}
	counters := destination.Counters()
	if counters.QuotaErrors != 1 || counters.FilesDeletedForQuota != 0 {
		t.Fatalf("the counters are %+v", counters)
	}
}

func Test_events_below_the_minimum_severity_are_accepted_and_not_written(t *testing.T) {
	directory := t.TempDir()
	settings := settings_in(directory)
	settings.MinimumSeverity = "ERROR"
	destination, build_error := file.NewFileDestination(settings)
	if build_error != nil {
		t.Fatal(build_error)
	}
	result := destination.Send(context.Background(), []value_objects.Envelope{
		log_event(value_objects.SeverityInfo, "quiet"), log_event(value_objects.SeverityCritical, "loud"),
	})
	if result.Accepted != 2 || result.Failed != 0 || result.Reason != "" {
		t.Fatalf("the result is %+v", result)
	}
	content := read_text(t, filepath.Join(directory, "telemetry.jsonl"))
	if strings.Contains(content, "quiet") || !strings.Contains(content, "loud") || strings.Count(content, "\n") != 1 {
		t.Fatalf("the file holds %q", content)
	}
}

func Test_an_event_that_cannot_be_encoded_fails_with_serialization_failed(t *testing.T) {
	directory := t.TempDir()
	destination, build_error := file.NewFileDestination(settings_in(directory))
	if build_error != nil {
		t.Fatal(build_error)
	}
	bad_event := log_event(value_objects.SeverityInfo, "bad")
	bad_event.Data = map[string]any{"value": math.NaN()}
	result := destination.Send(context.Background(), []value_objects.Envelope{bad_event, log_event(value_objects.SeverityInfo, "good")})
	if result.Accepted != 1 || result.Failed != 1 || result.Reason != "serialization_failed" {
		t.Fatalf("the result is %+v", result)
	}
	if strings.Count(read_text(t, filepath.Join(directory, "telemetry.jsonl")), "\n") != 1 {
		t.Fatal("only the good event must be written")
	}
}

// panicking_value panics when it is turned into JSON.
type panicking_value struct{}

func (panicking_value) MarshalJSON() ([]byte, error) { panic("the value panicked") }

func Test_a_panic_inside_the_destination_is_recovered_and_the_lock_is_released(t *testing.T) {
	directory := t.TempDir()
	destination, build_error := file.NewFileDestination(settings_in(directory))
	if build_error != nil {
		t.Fatal(build_error)
	}
	panicking_event := log_event(value_objects.SeverityInfo, "boom")
	panicking_event.Data = map[string]any{"value": panicking_value{}}
	result := destination.Send(context.Background(), []value_objects.Envelope{
		log_event(value_objects.SeverityInfo, "before"), panicking_event, log_event(value_objects.SeverityInfo, "after"),
	})
	if result.Accepted != 1 || result.Failed != 2 || result.Reason != "destination_failed" {
		t.Fatalf("the result is %+v", result)
	}
	// The mutex was released: the next call works.
	if next := destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "next")}); next.Accepted != 1 {
		t.Fatalf("the next result is %+v", next)
	}
}

func Test_a_panic_keeps_the_reason_that_was_already_set(t *testing.T) {
	directory := t.TempDir()
	destination, build_error := file.NewFileDestination(settings_in(directory))
	if build_error != nil {
		t.Fatal(build_error)
	}
	bad_event := log_event(value_objects.SeverityInfo, "bad")
	bad_event.Data = map[string]any{"value": math.NaN()}
	panicking_event := log_event(value_objects.SeverityInfo, "boom")
	panicking_event.Data = map[string]any{"value": panicking_value{}}
	result := destination.Send(context.Background(), []value_objects.Envelope{bad_event, panicking_event})
	if result.Failed != 2 || result.Reason != "serialization_failed" {
		t.Fatalf("the result is %+v", result)
	}
}

func Test_a_nil_destination_counts_every_event_as_failed(t *testing.T) {
	var destination *file.FileDestination
	result := destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "a"), log_event(value_objects.SeverityInfo, "b")})
	if result.Failed != 2 || result.Accepted != 0 || result.Reason != "destination_failed" {
		t.Fatalf("the result is %+v", result)
	}
	if counters := destination.Counters(); counters != (file.Counters{}) {
		t.Fatalf("the counters of a nil destination are %+v", counters)
	}
}

func Test_no_events_give_an_empty_result(t *testing.T) {
	destination, build_error := file.NewFileDestination(settings_in(t.TempDir()))
	if build_error != nil {
		t.Fatal(build_error)
	}
	for _, events := range [][]value_objects.Envelope{nil, {}} {
		if result := destination.Send(context.Background(), events); result.Accepted != 0 || result.Failed != 0 || result.Dropped != 0 || result.Reason != "" {
			t.Fatalf("the result is %+v", result)
		}
	}
}

func Test_sixteen_goroutines_never_interleave_their_lines(t *testing.T) {
	directory := t.TempDir()
	destination, build_error := file.NewFileDestination(settings_in(directory))
	if build_error != nil {
		t.Fatal(build_error)
	}
	const goroutines, batches, per_batch = 16, 5, 4
	var group sync.WaitGroup
	for goroutine := 0; goroutine < goroutines; goroutine++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for batch := 0; batch < batches; batch++ {
				events := make([]value_objects.Envelope, 0, per_batch)
				for position := 0; position < per_batch; position++ {
					events = append(events, log_event(value_objects.SeverityInfo, fmt.Sprintf("g%d-b%d-e%d", goroutine, batch, position)))
				}
				if result := destination.Send(context.Background(), events); result.Accepted != per_batch {
					t.Errorf("the result is %+v", result)
				}
			}
		}()
	}
	group.Wait()

	content := read_text(t, filepath.Join(directory, "telemetry.jsonl"))
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if len(lines) != goroutines*batches*per_batch {
		t.Fatalf("the file has %d lines, expected %d", len(lines), goroutines*batches*per_batch)
	}
	seen := map[string]bool{}
	for _, line := range lines {
		var parsed struct {
			Data map[string]any `json:"data"`
		}
		if decode_error := json.Unmarshal([]byte(line), &parsed); decode_error != nil {
			t.Fatalf("a line is not complete JSON: %q", line)
		}
		seen[fmt.Sprint(parsed.Data["message"])] = true
	}
	if len(seen) != len(lines) {
		t.Fatalf("%d different events in %d lines", len(seen), len(lines))
	}
}

func Test_the_counters_can_be_read_while_events_are_sent_and_the_rotations_are_counted(t *testing.T) {
	directory := t.TempDir()
	settings := settings_in(directory)
	settings.MaxFileBytes = 700
	settings.QuotaBytes = 700
	settings.MaxFiles = 3
	destination, build_error := file.NewFileDestination(settings)
	if build_error != nil {
		t.Fatal(build_error)
	}
	var group sync.WaitGroup
	stop_reading := make(chan struct{})
	reader_done := make(chan struct{})
	go func() {
		defer close(reader_done)
		var last file.Counters
		for {
			select {
			case <-stop_reading:
				return
			default:
				counters := destination.Counters()
				if counters.FilesRotated < last.FilesRotated || counters.FilesDeletedForQuota < last.FilesDeletedForQuota {
					t.Errorf("a counter went down: %+v after %+v", counters, last)
				}
				last = counters
			}
		}
	}()
	for goroutine := 0; goroutine < 16; goroutine++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for position := 0; position < 10; position++ {
				destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "x")})
			}
		}()
	}
	group.Wait()
	close(stop_reading)
	<-reader_done
	counters := destination.Counters()
	if counters.FilesRotated == 0 || counters.FilesDeletedForQuota == 0 || counters.WriteErrors != 0 || counters.RotationErrors != 0 || counters.QuotaErrors != 0 {
		t.Fatalf("the counters are %+v", counters)
	}
	// The total never stays over the quota by more than the file that is being written.
	entries, read_error := os.ReadDir(directory)
	if read_error != nil {
		t.Fatal(read_error)
	}
	var total int64
	for _, entry := range entries {
		info, info_error := entry.Info()
		if info_error != nil {
			t.Fatal(info_error)
		}
		total += info.Size()
	}
	if total > settings.QuotaBytes+700 {
		t.Fatalf("the files use %d bytes, over the quota of %d", total, settings.QuotaBytes)
	}
}

func Test_the_errors_of_the_adapter_keep_the_sentinel_and_the_cause(t *testing.T) {
	cause := errors.New("the cause")
	cases := []struct {
		name     string
		got      error
		sentinel error
	}{
		{"write", adapter_error.MapWriteError(cause), adapter_error.ErrWriteFailed},
		{"rotation", adapter_error.MapRotationError(cause), adapter_error.ErrRotationFailed},
		{"serialization", adapter_error.MapSerializationError(cause), adapter_error.ErrSerializationFailed},
		{"destination", adapter_error.MapDestinationError(cause), adapter_error.ErrDestinationUnusable},
		{"destination without a cause", adapter_error.MapDestinationError(nil), adapter_error.ErrDestinationUnusable},
	}
	for _, error_case := range cases {
		t.Run(error_case.name, func(t *testing.T) {
			if !errors.Is(error_case.got, error_case.sentinel) {
				t.Fatalf("%v is not %v", error_case.got, error_case.sentinel)
			}
			if error_case.name != "destination without a cause" && !errors.Is(error_case.got, cause) {
				t.Fatalf("%v lost its cause", error_case.got)
			}
		})
	}
}
