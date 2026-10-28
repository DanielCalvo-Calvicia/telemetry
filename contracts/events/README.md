# events

The registry of event types ([registry.yaml](registry.yaml), REQ-CTR-001, REQ-CTR-002): for each type in the notation `domain.entity.action`, its schema, its capability and the `schema_version` since which it exists. Every event type has a schema, and the schema of `<domain>.<entity>.<action>` is `schemas/<domain>/<entity>_<action>.schema.json`.

An event type is added with its capability, one at a time (Q02-5).

## Registered event types

| Event type | Schema | Capability |
|---|---|---|
| `log.entry.recorded` | [schemas/log/entry_recorded.schema.json](../schemas/log/entry_recorded.schema.json) | logging |
