// Command registry_check checks the registry of capabilities (contracts/capabilities/registry.yaml): every requirement
// of the specification is listed by a capability, every listed test file exists, every pending task exists and no
// capability is marked supported without the evidence (REQ-CNF-007). It prints what is not covered yet and the
// maturity of the library (REQ-CNF-008), and exits 1 when a rule is broken.
//
//	go run . --spec <file> --registry <file> --root <dir> --tasks <dir>
package main

import (
	"os"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/composition_root"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/inbound/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, composition_root.NewChecker))
}
