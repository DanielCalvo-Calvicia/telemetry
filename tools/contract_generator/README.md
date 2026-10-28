# contract_generator

The generator of [T-02.4](../../docs/tasks/T-02_contract-and-generator/T-02.4_the-generator.md): it reads `contracts/` and writes the code of both languages (REQ-CNF-002). It is a Go module of its own, so its YAML and JSON Schema libraries never reach the core module (REQ-ARC-005). It is not part of the library: a consumer never imports it.

## Regenerate

```bash
cd tools/contract_generator && go run . --contracts ../../contracts --go-out ../../go/internal/domain/value_objects --python-out ../../python/src/telemetry/internal/domain/value_objects
```

The output is **committed** (REQ-DST-005): Go in `go/internal/domain/value_objects/*_generated.go` and Python in `python/src/telemetry/internal/domain/value_objects/*_generated.py`, so a Python install never needs Go. Never edit a generated file by hand: change `contracts/` and run the command again. The CI job `generated-code` fails if the committed files differ from what the command writes.

## What it does

1. **Checks the contract first** and stops with an error that names the file: every enum and schema is valid JSON and a valid JSON Schema (draft 2020-12) whose `$ref`s resolve; every registered event type has its schema at `schemas/<domain>/<entity>_<action>.schema.json` and every such schema is registered; the error codes are unique, lowercase `snake_case`, with a known category and a message; the enum values are unique. If the check fails nothing is written.
2. **Generates**, the same in both languages (names by [contracts/conventions/naming.md](../../contracts/conventions/naming.md)): the enums (`enums_generated`), the registered event types (`event_types_generated`), the error codes with their table (`error_codes_generated`) and the envelope types, including the data type of each event type (`envelope_generated`).
3. Is **deterministic**: sorted input, no timestamp and no machine path; running it twice gives the same bytes. Every file starts with a "generated, do not edit" header that names its source files.

## The schema subset it understands

Properties of type `string`, `number`, `integer`, `boolean` and an object without properties; `$ref` to an enum, to a schema of `schemas/common/`, to `attributes.schema.json` or to `#/$defs/utc_time`. Anything else (arrays, nested objects, other local `$defs`) stops the generator with "unsupported schema construct" and the file and property: extend the generator when a schema needs it.

The configuration schema (REQ-CNF-002) is not generated yet: it does not exist until T-03.6.

## Tests

```bash
cd tools/contract_generator && go test ./... -count=1
```

They are black-box and mirror the code under `internal/tests/`: a small fixture contract whose generated output is compared byte by byte with golden files (`*.golden`), the second run, and the broken contracts.
