// Package file is the public file destination of the telemetry library: it appends the events of a core to a local file
// as JSON Lines, with rotation by size and a quota on the total space. A project builds it with NewDestination and passes
// it to the core: telemetry.New(config, telemetry.WithLogSink(destination)). The package reads no environment variable and
// no file of settings: the project passes the Settings in, and the directory is always the project's choice.
package file

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/dependencies"
	internal_file "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file"
)

// Settings are the settings of the file destination: the Directory (required), the FileName, the MinimumSeverity
// (TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL, in upper case), the MaxFileBytes at which the file rotates, the MaxFiles
// that are kept (the current one included) and the QuotaBytes that all the files together may use.
type Settings = internal_file.Settings

// Counters are what the file destination counts: the rotations, the files deleted for the quota and the failures of
// writes, rotations and deletions.
type Counters = internal_file.Counters

// Destination is the file destination. It satisfies the log sink port of the core, and its Counters method returns the
// counters of its files and failures.
type Destination = internal_file.FileDestination

// DefaultSettings returns the settings with their defaults: the file telemetry.jsonl, a minimum severity of INFO, files of
// 10 MiB, 5 files and a quota of 50 MiB. The Directory is empty: set it before NewDestination.
func DefaultSettings() Settings { return internal_file.DefaultSettings() }

// NewDestination validates the settings, creates the directory (readable by the owner only) and checks that it can be
// written. An invalid setting or a directory that cannot be written returns a *telemetry.ValidationError with its
// problems. Pass the result to telemetry.WithLogSink.
func NewDestination(settings Settings) (*Destination, error) {
	return dependencies.NewFileDestination(settings)
}
