# ADR-006 — Masking happens before every sink

**Status:** Accepted (carried over from the source service, Q17-1 to Q17-3)
**Related:** [ADR-005](ADR-005-a-failing-sink-never-fails-the-caller.md)

## Context

Log lines leave the process (console, HTTP receiver, CloudWatch). A secret that reaches a sink is a leak, and a sink that has to remember to mask is a sink that will forget.

## Decision

`SensitiveDataMasker` (`domain/operations`) masks the whole entry **once**, in `LoggerService`, before it reaches any sink; the sinks never see sensitive data. Everything sensitive is replaced by `*****MASKED*****` wherever it appears: as the value of a field with a sensitive name, as a whole value, or as a segment of a longer text. The detectors are data in `domain/rules.go`: AWS access key, bearer credential, JWT, private key block, password of a connection string, and credential pairs (`name=value`, `name: value`, `"name": "value"`). A consumer adds regular expressions with `LOGGER_MASK_EXTRA_PATTERNS`.

## Consequences

- Positive: one place decides what is sensitive; adding a sink cannot reopen the leak; the report of a failing sink never repeats the message.
- Negative: detection is by pattern and by field name, so it has limits: free text with an unusual secret is not recognized until the consumer adds a pattern. The names `body`, `content` and `payload` are always hidden, which makes some useful lines less informative.

> **Update 2026-10-28:** [PR-001](../proposals/PR-001-telemetry-standard-go-python.md) (REQ-SEC-002) proposes adding the default redaction list (`password`, `token`, `authorization`, `api_key`, `secret`) to the current detectors; nothing is removed. Not built yet.
