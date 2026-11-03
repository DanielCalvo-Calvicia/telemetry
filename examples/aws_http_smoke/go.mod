module github.com/DanielCalvo-Calvicia/telemetry/examples/aws_http_smoke

go 1.25.5

require (
	github.com/DanielCalvo-Calvicia/telemetry/go v0.0.0
	github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http v0.0.0
)

// For local development only: they point to the core and to the adapter of this repository. A project outside this
// repository removes them and requires the tagged versions.
replace (
	github.com/DanielCalvo-Calvicia/telemetry/go => ../../go
	github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http => ../../go/pkg/logging/aws_http
)
