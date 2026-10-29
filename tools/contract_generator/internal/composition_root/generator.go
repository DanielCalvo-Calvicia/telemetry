// Package composition_root wires the use case of the generator with its concrete adapters. It is the only package
// that imports them.
package composition_root

import (
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/inbound/cli"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/outbound/contract_folder"
	folder_external "github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/outbound/contract_folder/external"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/outbound/file_writer"
	writer_external "github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/outbound/file_writer/external"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/outbound/go_renderer"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/outbound/python_renderer"
)

// NewGenerator builds the use case that reads the contracts folder and writes the Go and the Python types and the Go
// and the Python validators of the data of the event types.
func NewGenerator(
	contracts_dir string,
	go_out_dir string,
	python_out_dir string,
	go_validation_out_dir string,
	python_validation_out_dir string,
) cli.Executor {
	return application.NewGenerateCode(
		contract_folder.NewReader(folder_external.NewFolder(contracts_dir)),
		application.Target{Renderer: go_renderer.NewRenderer(), Writer: file_writer.NewWriter(writer_external.Disk{}, go_out_dir)},
		application.Target{Renderer: python_renderer.NewRenderer(), Writer: file_writer.NewWriter(writer_external.Disk{}, python_out_dir)},
		application.Target{Renderer: go_renderer.NewValidationRenderer(), Writer: file_writer.NewWriter(writer_external.Disk{}, go_validation_out_dir)},
		application.Target{Renderer: python_renderer.NewValidationRenderer(), Writer: file_writer.NewWriter(writer_external.Disk{}, python_validation_out_dir)},
	)
}
