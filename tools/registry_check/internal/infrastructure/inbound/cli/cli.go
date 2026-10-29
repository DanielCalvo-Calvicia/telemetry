// Package cli is the command line of the check: it reads the four paths from the arguments, runs the use case and
// prints the report.
package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"
)

// Executor is the use case the command line runs.
type Executor interface {
	// Execute checks the registry. The error is for what stops the check from running; a broken rule is in the report.
	Execute() (domain.Report, error)
}

// Factory builds the use case for the four paths of the command line: the specification, the registry, the root of
// the repository and the tasks folder.
type Factory func(spec_path string, registry_path string, root_dir string, tasks_dir string) Executor

// Exit codes of the command.
const (
	// ExitOK is returned when no rule is broken.
	ExitOK = 0
	// ExitFailure is returned when a rule is broken or the check could not run.
	ExitFailure = 1
	// ExitUsage is returned when the arguments are wrong.
	ExitUsage = 2
)

// Defaults of the flags, relative to the root of the repository: CI passes every path explicitly.
const (
	default_spec     = "docs/proposals/PR-001-telemetry-standard-go-python.md"
	default_registry = "contracts/capabilities/registry.yaml"
	default_root     = "."
	default_tasks    = "docs/tasks"
)

// Run parses the arguments (without the program name), runs the use case and returns the exit code. The report goes
// to the output writer and every broken rule and every failure to the error writer.
func Run(arguments []string, output_writer io.Writer, error_writer io.Writer, build Factory) int {
	flags := flag.NewFlagSet("registry_check", flag.ContinueOnError)
	flags.SetOutput(error_writer)
	spec_path := flags.String("spec", default_spec, "the specification that defines the REQ-* identifiers")
	registry_path := flags.String("registry", default_registry, "the registry of capabilities to check")
	root_dir := flags.String("root", default_root, "the root of the repository, against which the test paths are resolved")
	tasks_dir := flags.String("tasks", default_tasks, "the folder of the tasks, to check the pending task identifiers")
	if parse_error := flags.Parse(arguments); parse_error != nil {
		return ExitUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(error_writer, "registry_check: unexpected argument %q\n", flags.Arg(0))
		flags.Usage()
		return ExitUsage
	}
	report, execute_error := build(*spec_path, *registry_path, *root_dir, *tasks_dir).Execute()
	if execute_error != nil {
		fmt.Fprintf(error_writer, "registry_check: %v\n", execute_error)
		return ExitFailure
	}
	Print(output_writer, report)
	for _, finding := range report.Findings {
		fmt.Fprintf(error_writer, "registry_check: error: %v\n", finding)
	}
	if !report.Passed() {
		fmt.Fprintf(error_writer, "registry_check: FAILED with %d error(s)\n", len(report.Findings))
		return ExitFailure
	}
	return ExitOK
}
