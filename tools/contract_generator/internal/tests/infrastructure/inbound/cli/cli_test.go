package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/inbound/cli"
)

type fake_executor struct{ execute_error error }

func (executor fake_executor) Execute() error { return executor.execute_error }

func factory_of(execute_error error, received *[]string) cli.Factory {
	return func(contracts_dir string, go_out_dir string, python_out_dir string) cli.Executor {
		*received = []string{contracts_dir, go_out_dir, python_out_dir}
		return fake_executor{execute_error: execute_error}
	}
}

func TestTheThreeFoldersReachTheUseCase(test *testing.T) {
	var received []string
	var messages bytes.Buffer
	code := cli.Run([]string{"--contracts", "c", "--go-out", "g", "--python-out", "p"}, &messages, factory_of(nil, &received))
	if code != cli.ExitOK || messages.Len() != 0 {
		test.Errorf("code %d, messages %q", code, messages.String())
	}
	if strings.Join(received, ",") != "c,g,p" {
		test.Errorf("received %v", received)
	}
}

func TestAFailureGivesANonZeroCodeAndTheMessage(test *testing.T) {
	var received []string
	var messages bytes.Buffer
	code := cli.Run([]string{"--contracts", "c", "--go-out", "g", "--python-out", "p"}, &messages,
		factory_of(errors.New("contracts/errors/catalog.yaml: duplicated error code"), &received))
	if code != cli.ExitFailure || !strings.Contains(messages.String(), "contracts/errors/catalog.yaml") {
		test.Errorf("code %d, messages %q", code, messages.String())
	}
}

func TestMissingOrUnknownArgumentsAreAUsageError(test *testing.T) {
	cases := [][]string{
		{},
		{"--contracts", "c", "--go-out", "g"},
		{"--contracts", "c", "--python-out", "p"},
		{"--go-out", "g", "--python-out", "p"},
		{"--unknown"},
	}
	for _, arguments := range cases {
		var received []string
		var messages bytes.Buffer
		if code := cli.Run(arguments, &messages, factory_of(nil, &received)); code != cli.ExitUsage {
			test.Errorf("%v: code %d", arguments, code)
		}
		if received != nil || messages.Len() == 0 {
			test.Errorf("%v: the use case must not run and a message is expected", arguments)
		}
	}
}
