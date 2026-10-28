# schemas

The JSON Schema (D-02, dialect draft 2020-12) of the event envelope and of the `data` of each event type (REQ-CTR-001 to REQ-CTR-003). The schema is the canonical definition; the Go and Python types are generated from it or validated against it.

## Files

| File | What it defines |
|---|---|
| [envelope.schema.json](envelope.schema.json) | the envelope every event uses: `schema_version`, `event_id` (UUIDv7), `event_type`, `occurred_at`, `recorded_at`, `severity`, `resource`, `actor`, `context`, `data`, `attributes` |
| [common/resource.schema.json](common/resource.schema.json) | the identity of the emitting process: `service_name`, `instance_id`, and optionally `environment`, `service_version`, `host`, `time_zone` |
| [common/actor.schema.json](common/actor.schema.json) | the subject of the event: `kind` (from [enums/actor_kind.json](../enums/actor_kind.json)), `id`, and optionally `type`, `version` |
| [common/context.schema.json](common/context.schema.json) | the correlation: `trace_id`, `span_id`, `execution_id`, `correlation_id` |
| [common/attributes.schema.json](common/attributes.schema.json) | the consumer's extensions: scalars and lists of scalars, at most 32 keys, a key up to 64 and a string value up to 256 characters |

The schema of each event type narrows the open `data` object of the envelope. **Where it lives:** the schema of the event type `<domain>.<entity>.<action>` is `schemas/<domain>/<entity>_<action>.schema.json`, and the type is registered in [../events/registry.yaml](../events/registry.yaml). `device/` and `ai/` are the domains of `device.*` and `ai.*`; they are separate and never reused for each other (REQ-DEV-002, REQ-AI-001).

| File | Event type or concept |
|---|---|
| [log/entry_recorded.schema.json](log/entry_recorded.schema.json) | the `data` of `log.entry.recorded`: `category`, optional `function`, `message` |
| [common/failure.schema.json](common/failure.schema.json) | the classification of a failure (REQ-FLT-001): `code`, `category`, `severity`, `recoverable` |

## Rules of these files

- **Each concept has one field** (section 5.2): the identity of the process (`resource`), of the subject (`actor`), of the execution and of the correlation (`context`) are separate.
- **The objects are closed** (`additionalProperties: false`) so a producer never emits a field the contract does not name; the extension point is `attributes`. A **consumer is tolerant**: it ignores the unknown fields of later MINOR versions (REQ-VER-004), which is a rule of how it reads, not a schema keyword.
- **The identifiers `$id` and `$ref` use a placeholder address** (`https://telemetry.example/...`): the schemas are never fetched from it; they are resolved from this folder.
- **Times** are UTC with six decimals and the zone `Z` (`2026-10-28T10:00:00.123456Z`); `recorded_at` is set only by the library (ADR-012).
