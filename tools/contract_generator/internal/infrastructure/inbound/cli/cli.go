// Package cli is the command line of the generator: it reads the three folders from the arguments and runs the use
// case.
package cli

import (
	"flag"
	"fmt"
	"io"
)

// Executor is the use case the command line runs.
type Executor interface {
	// Execute generates the code and returns an error that names the file that stopped it.
	Execute() error
}

// Factory builds the use case for the three folders of the command line.
type Factory func(contracts_dir string, go_out_dir string, python_out_dir string) Executor

// Exit codes of the command.
const (
	// ExitOK is returned when the code was generated.
	ExitOK = 0
	// ExitFailure is returned when the generator stopped with an error.
	ExitFailure = 1
	// ExitUsage is returned when the arguments are wrong.
	ExitUsage = 2
)

// Run parses the arguments (without the program name), runs the use case and returns the exit code. Every message
// goes to the error writer.
func Run(arguments []string, error_writer io.Writer, build Factory) int {
	flags := flag.NewFlagSet("contract_generator", flag.ContinueOnError)
	flags.SetOutput(error_writer)
	contracts_dir := flags.String("contracts", "", "the contracts folder to read (required)")
	go_out_dir := flags.String("go-out", "", "the folder where the Go files are written (required)")
	python_out_dir := flags.String("python-out", "", "the folder where the Python files are written (required)")
	if parse_error := flags.Parse(arguments); parse_error != nil {
		return ExitUsage
	}
	if *contracts_dir == "" || *go_out_dir == "" || *python_out_dir == "" {
		fmt.Fprintln(error_writer, "contract_generator: --contracts, --go-out and --python-out are all required")
		flags.Usage()
		return ExitUsage
	}
	if execute_error := build(*contracts_dir, *go_out_dir, *python_out_dir).Execute(); execute_error != nil {
		fmt.Fprintf(error_writer, "contract_generator: %v\n", execute_error)
		return ExitFailure
	}
	return ExitOK
}
