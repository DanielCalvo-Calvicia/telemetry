// Package composition_root wires the use case of the check with its concrete adapters. It is the only package that
// imports them.
package composition_root

import (
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/inbound/cli"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/registry_file"
	registry_external "github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/registry_file/external"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/spec_file"
	spec_external "github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/spec_file/external"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/task_files"
	tasks_external "github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/task_files/external"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/test_files"
	test_files_external "github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/test_files/external"
)

// NewChecker builds the use case that checks a registry file against a specification, the test files under a root
// folder and the tasks of a tasks folder.
func NewChecker(spec_path string, registry_path string, root_dir string, tasks_dir string) cli.Executor {
	return application.NewCheckRegistry(
		registry_file.NewReader(registry_external.NewFile(registry_path)),
		spec_file.NewReader(spec_external.NewFile(spec_path)),
		test_files.NewFinder(test_files_external.NewDisk(root_dir)),
		task_files.NewCatalog(tasks_external.NewFolder(tasks_dir)),
	)
}
