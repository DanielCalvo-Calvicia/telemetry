# ADR-008 — The AWS HTTP sink adapts the entry to the `add_log` contract

**Status:** Accepted (2026-10-20, requested by the maintainer; released in v0.2.0)
**Related:** [ADR-005](ADR-005-a-failing-sink-never-fails-the-caller.md), [ADR-009](ADR-009-ready-made-sink-constructors.md)

## Context

The generic HTTP sink posts the flat JSON entry to a receiver. The `add_log` endpoint of aws_microservice (`POST /api/v1/cloudwatch/logs/add`) expects a different body: `log_group_name`, `log_stream_name`, `message` and `timestamp` in milliseconds. Posting the flat entry there answers 400.

## Decision

A new outbound adapter, `aws_http` (`internal/logging/infrastructure/outbound/aws_http`), implements `LogSinkPort` and adapts every entry to that contract: the log group and the log stream come from its configuration, the `timestamp` is the entry's time in milliseconds and the `message` is the **whole entry** encoded as the flat JSON payload, so the level, the category and the fields are kept in CloudWatch. It has its own `external/`, `error/` and `helpers/` folders like the other adapters, a 3 s timeout, no retry, and the same failure handling ([ADR-005](ADR-005-a-failing-sink-never-fails-the-caller.md)). It is switched on with `LOGGER_AWS_HTTP_*`.

## Consequences

- Positive: a service logs to CloudWatch without an AWS SDK and without credentials; the endpoint creates the log group and the stream when they do not exist.
- Negative: the sink depends on the contract of another service; a change of `add_log` is a change of this adapter (the tests build the request from the same field names). The `message` of a CloudWatch event is JSON text, so a reader sees a JSON string inside the event.
