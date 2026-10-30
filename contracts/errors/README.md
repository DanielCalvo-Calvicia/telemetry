# errors

The catalog of error codes ([catalog.yaml](catalog.yaml), REQ-CNF-004): a stable code, its category and a default message. The validation and configuration codes are identical in Go and Python; the text of a message may differ. A code is lowercase `snake_case`, unique, and never renamed or reused.

The failure classification of REQ-FLT-001 (`code`, `category`, `severity`, `recoverable`) is [schemas/common/failure.schema.json](../schemas/common/failure.schema.json) with the categories of [enums/failure_category.json](../enums/failure_category.json).

Each capability adds its own codes as it is built (delivery, devices, AI), as it does for event types.

## Codes

| Code | Category | When |
|---|---|---|
| `event_schema_invalid` | validation | the data of an event does not match the schema of its type |
| `event_type_unknown` | validation | the event type is not registered |
| `limit_exceeded` | validation | a payload, event or attribute limit was exceeded |
| `config_field_invalid` | configuration | a configuration field is missing or invalid |
| `delivery_mode_unknown` | configuration | the delivery mode is not `best_effort`, `retry` or `durable` |
| `instance_id_unavailable` | configuration | the instance id could not be generated at start (T-03.7) |
| `event_build_failed` | internal | a port or an unexpected failure stopped the building of an event (T-03.7) |
| `masking_failed` | internal | the masking of an event failed; the event is dropped (T-03.7) |
| `limits_failed` | internal | applying the limits failed for a reason other than a limit exceeded (T-03.7) |

## Decided by the maintainer (2026-10-28)

Each capability adds its own error codes as it is built, as it does for event types; the catalog does not list the delivery codes in advance. Delivery codes arrive with T-05, device codes with T-08 and AI codes with T-09. A code is permanent, so none is written before the code that raises it exists.
