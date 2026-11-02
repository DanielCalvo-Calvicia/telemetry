// Package file is the adapter that appends events to a local file as JSON Lines, with rotation by size and a quota on the
// total space (section 7.2 of PR-001). It follows the tree of the adapters of REQ-ARC-014: the file system I/O lives in
// external/, the error translation in error/ and the pure helpers (the line, the rotation plan, the quota plan) in
// helpers/.
package file

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file/error"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file/external"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file/helpers"
)

// =============================================================================
// FILE DESTINATION
// =============================================================================

// Counters are what the file destination counts about its files and its failures. They only grow.
type Counters struct {
	// FilesRotated is how many times the current file was rotated.
	FilesRotated int64
	// FilesDeletedForQuota is how many rotated files were deleted so that the total fit the quota.
	FilesDeletedForQuota int64
	// WriteErrors is how many lines the file did not take.
	WriteErrors int64
	// RotationErrors is how many rotations failed.
	RotationErrors int64
	// QuotaErrors is how many deletions for the quota failed.
	QuotaErrors int64
}

// FileDestination implements outbound.LogSinkPort and outbound.EventSinkPort: it appends each event as one canonical JSON
// line to the current file of a directory. The file is opened for each line and closed at once: no handle is kept, so a
// rotation can rename the file on every platform and there is nothing to close. A mutex held for the whole batch makes
// the lines of concurrent callers never interleave. It never returns an error and never panics to the caller of Send
// (ADR-005). The files hold only the masked event line and are readable by the owner only.
type FileDestination struct {
	directory        string
	file_name        string
	minimum_severity value_objects.Severity
	max_file_bytes   int64
	max_files        int
	quota_bytes      int64
	write_mutex      sync.Mutex
	counters_mutex   sync.Mutex
	counters         Counters
}

var (
	_ outbound.LogSinkPort   = (*FileDestination)(nil)
	_ outbound.EventSinkPort = (*FileDestination)(nil)
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewFileDestination validates the settings, creates the directory (readable by the owner only) when it does not exist
// and checks that the current file can be appended. An invalid setting returns the *config.ValidationError of
// ValidateSettings as it is. A directory that cannot be written returns a *config.ValidationError with one problem on the
// field directory and a fixed reason: neither the path nor the text of the system error is in it.
func NewFileDestination(settings Settings) (*FileDestination, error) {
	if validation_error := ValidateSettings(settings); validation_error != nil {
		return nil, validation_error
	}
	base_path := filepath.Join(settings.Directory, settings.FileName)
	if directory_error := external.EnsureDirectory(settings.Directory); directory_error != nil {
		return nil, directory_not_writable()
	}
	if touch_error := external.TouchAppend(base_path); touch_error != nil {
		return nil, directory_not_writable()
	}
	return &FileDestination{
		directory:        settings.Directory,
		file_name:        settings.FileName,
		minimum_severity: value_objects.Severity(settings.MinimumSeverity),
		max_file_bytes:   settings.MaxFileBytes,
		max_files:        settings.MaxFiles,
		quota_bytes:      settings.QuotaBytes,
	}, nil
}

// ----- SEND -----------------------------------------------------------------

// Send appends the events, one line each, and counts each event once. An event below the minimum severity is not written
// and counts as accepted: the destination took it and its policy decided not to write it. An event that cannot be encoded
// counts as failed with the reason serialization_failed; a rotation that fails makes the event failed with the reason
// rotation_failed and it is not written; a line that the file does not take counts as failed with the reason
// write_failed. The next events are still tried. The reason of the result is the first one. A panic is recovered: the
// events not counted yet are failed with the reason destination_failed. A nil destination counts every event as failed
// with that reason. The files and the failures are counted in Counters.
func (destination *FileDestination) Send(_ context.Context, events []value_objects.Envelope) (result dtos.SendResult) {
	if destination == nil {
		return dtos.SendResult{Failed: len(events), Reason: adapter_error.ReasonDestinationFailed}
	}
	destination.write_mutex.Lock()
	defer destination.write_mutex.Unlock()
	defer func() {
		if recover() != nil {
			not_counted := len(events) - result.Accepted - result.Dropped - result.Failed
			if not_counted > 0 {
				result.Failed += not_counted
				if result.Reason == "" {
					result.Reason = adapter_error.ReasonDestinationFailed
				}
			}
		}
	}()

	for _, event := range events {
		reason, deliver_error := destination.deliver(event)
		if deliver_error != nil {
			result.Failed++
			if result.Reason == "" {
				result.Reason = reason
			}
			continue
		}
		result.Accepted++
	}
	return result
}

// Counters returns a copy of the counters. It is safe to call while events are being sent.
func (destination *FileDestination) Counters() Counters {
	if destination == nil {
		return Counters{}
	}
	destination.counters_mutex.Lock()
	defer destination.counters_mutex.Unlock()
	return destination.counters
}

// ----- HELPERS --------------------------------------------------------------

// directory_not_writable is the error of a directory that cannot be created or appended to. Its reason is fixed.
func directory_not_writable() error {
	return config.NewValidationError([]config.Problem{{
		Field: "directory", Code: value_objects.ErrorCodeConfigFieldInvalid, Reason: "the directory cannot be written",
	}})
}

// count changes the counters under their own mutex, so Counters never waits for a batch.
func (destination *FileDestination) count(change func(counters *Counters)) {
	destination.counters_mutex.Lock()
	defer destination.counters_mutex.Unlock()
	change(&destination.counters)
}

// deliver writes one event when it reaches the minimum severity: it rotates when the line would pass the size, appends
// the line and enforces the quota. It returns the reason and the classified error of a failure, and nil when the event
// was written or filtered. The caller holds the mutex.
func (destination *FileDestination) deliver(event value_objects.Envelope) (string, error) {
	if !value_objects.SeverityAtLeast(event.Severity, destination.minimum_severity) {
		return "", nil
	}
	line, encode_error := helpers.EncodeJSONLine(event)
	if encode_error != nil {
		return adapter_error.ReasonSerializationFailed, adapter_error.MapSerializationError(encode_error)
	}

	base_path := filepath.Join(destination.directory, destination.file_name)
	size, exists, size_error := external.FileSize(base_path)
	if size_error != nil {
		destination.count(func(counters *Counters) { counters.WriteErrors++ })
		return adapter_error.ReasonWriteFailed, adapter_error.MapWriteError(size_error)
	}
	if exists && size > 0 && size+int64(len(line)) > destination.max_file_bytes {
		if rotation_error := destination.rotate(); rotation_error != nil {
			destination.count(func(counters *Counters) { counters.RotationErrors++ })
			return adapter_error.ReasonRotationFailed, adapter_error.MapRotationError(rotation_error)
		}
		destination.count(func(counters *Counters) { counters.FilesRotated++ })
	}

	if append_error := external.AppendLine(base_path, line); append_error != nil {
		destination.count(func(counters *Counters) { counters.WriteErrors++ })
		return adapter_error.ReasonWriteFailed, adapter_error.MapWriteError(append_error)
	}
	destination.enforce_quota()
	return "", nil
}

// rotate runs the steps of the rotation plan and stops at the first one that fails.
func (destination *FileDestination) rotate() error {
	names, list_error := external.ListNames(destination.directory)
	if list_error != nil {
		return list_error
	}
	for _, step := range helpers.PlanRotation(destination.file_name, destination.max_files, names) {
		from_path := filepath.Join(destination.directory, step.From)
		var step_error error
		if step.Op == helpers.StepRemove {
			step_error = external.Remove(from_path)
		} else {
			step_error = external.Rename(from_path, filepath.Join(destination.directory, step.To))
		}
		if step_error != nil {
			return step_error
		}
	}
	return nil
}

// enforce_quota deletes the oldest rotated files while the total size is over the quota. A deletion that fails is
// counted and does not fail the event that was written.
func (destination *FileDestination) enforce_quota() {
	sizes := map[string]int64{}
	candidate_names := []string{destination.file_name}
	for index := 1; index < destination.max_files; index++ {
		candidate_names = append(candidate_names, helpers.RotatedName(destination.file_name, index))
	}
	for _, name := range candidate_names {
		size, exists, size_error := external.FileSize(filepath.Join(destination.directory, name))
		if size_error != nil {
			destination.count(func(counters *Counters) { counters.QuotaErrors++ })
			continue
		}
		if exists {
			sizes[name] = size
		}
	}
	for _, name := range helpers.PlanQuota(destination.file_name, destination.max_files, sizes, destination.quota_bytes) {
		if remove_error := external.Remove(filepath.Join(destination.directory, name)); remove_error != nil {
			destination.count(func(counters *Counters) { counters.QuotaErrors++ })
			continue
		}
		destination.count(func(counters *Counters) { counters.FilesDeletedForQuota++ })
	}
}
