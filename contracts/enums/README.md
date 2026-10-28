# enums

The enums of the contract. Each `.json` file is a JSON Schema with an `enum`, so the generator reads it and a schema can `$ref` it.

| File | Values |
|---|---|
| [severity.json](severity.json), [severity.md](severity.md) | `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`, `CRITICAL`, with their meaning and their mapping to `log/slog`, Python `logging` and OpenTelemetry |
| [actor_kind.json](actor_kind.json) | `device`, `agent`, `subsystem` (the first values) |

Still to come: the execution states and their valid transitions (`execution_transitions.yaml`, T-07.1) and, in `device/` and `ai/`, the allowed values of the device and AI fields (REQ-DEV-005, REQ-AI-005). Adding a value is a MINOR change of the contract (REQ-VER-003).
