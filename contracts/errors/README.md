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

## Open question for the maintainer

Does this catalog also hold the **delivery** failure codes now, or does each capability add its own as it is built (the plan says the latter for event types)? **Not answered; the default was used:** each capability adds its own. Delivery codes arrive with T-05, device codes with T-08 and AI codes with T-09.
