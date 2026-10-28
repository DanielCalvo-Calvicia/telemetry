// Command contract_generator reads the contracts folder and writes the Go and the Python code generated from it
// (REQ-CNF-002). It stops, naming the file, when the contract is not valid.
//
//	go run . --contracts <dir> --go-out <dir> --python-out <dir>
package main

import (
	"os"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/composition_root"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/inbound/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stderr, composition_root.NewGenerator))
}
