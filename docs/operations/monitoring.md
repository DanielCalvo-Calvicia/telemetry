# Monitoring and operations

The library is not a service, so it has no probes and no metrics. What a consumer can observe is the behaviour of the logger itself.

## Logs

| What | Where | Format |
|---|---|---|
| The entries of the consumer | every sink that is switched on | the flat JSON payload: `ProducerID`, `ApplicationID`, `ProcessID`, `Application`, `Category`, `Function`, `Indentation`, `Depth`, `Level`, `Message`, `Fields`, `Timestamp` |
| The report of a failing sink | the console (the fallback sink) | an ERROR entry with category `logger`, message `log sink failed: the entry was dropped`, and the fields `dropped_level` and `error` (the cause, joined with `log sink: the entry could not be delivered`) |

The report keeps the identifiers of the dropped entry (application, instance, process) so it can be matched with the request, and never repeats the message or the fields of the dropped entry.

## What to watch in a consumer

| Symptom | Likely cause | Where to look |
|---|---|---|
| ERROR lines `log sink failed: the entry was dropped` on the console | the receiver is down, slow (over 3 s) or answers 400 or above | the `error` field: a connection error, a timeout or `log receiver answered with status N` |
| `AWS log receiver answered with status 400` | the body does not fit the `add_log` contract (a missing group, stream, message or timestamp) | the configuration `LOGGER_AWS_HTTP_*`, the `add_log` contract in [api.md](../api/api.md) |
| No DEBUG lines in production | by design: production lets INFO and above through | [ADR-007](../decisions/ADR-007-minimum-level-follows-the-environment.md) |
| `*****MASKED*****` in a value | the masker found sensitive data or a sensitive field name | [ADR-006](../decisions/ADR-006-masking-before-every-sink.md) |
| The service does not start with a logger error | a sink that is switched on lacks a value, `app_env` has no level, or a mask pattern is not a regular expression | the joined error names the sentinel |

## Timeouts, retries and shutdown

- The HTTP sinks time out after **3 seconds** and **never retry**: a failed entry is dropped for that sink and reported ([ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md)).
- The sinks write synchronously: the call returns after every sink has answered or failed.
- `shutdown` (the second value of `New` and `NewWithSinks`) is called on exit. It flushes what the sinks buffer; today the console and HTTP sinks buffer nothing, so it does nothing, but a consumer should still call it.

## Manual end-to-end check of a deployment

1. `examples/aws_http_smoke`: a sample service that logs in production to the `add_log` endpoint (it needs `LOGGER_AWS_HTTP_*` in its environment).
2. `internal/tests/e2e/deployed_aws_microservice_test.go`: sends one line to a deployed aws_microservice through the `aws_http` adapter and fails when the endpoint does not accept it. Set the constants at the top and the switch `run_against_deployed_service`; the test prints a unique text to search for in the CloudWatch log stream. Keep the switch `false` and the address a placeholder in anything that is committed.

## Not present

Metrics, tracing, a retry queue and a disk buffer. Delivery is best effort ([ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md)).

> **Planned (T-02.6):** the new standard counts the events rejected by the validation at the source (a counter by event type and error code) in its diagnostics (REQ-DGN-001). The validator exists in the domain; the counter is wired by T-03.5 and T-05.5. Nothing counts rejections today.

> **Planned (T-03.2):** the result of a sink call (`SendResult`: accepted, dropped, failed and a reason) is the source of the drop counters per destination and reason (REQ-DLV-008, REQ-DGN-001). The ports and the result exist; the counters are wired in T-05.5. Nothing is emitted yet.

> **Planned (T-03.5):** the counters of the limits (attribute keys dropped, attribute keys truncated, attribute values truncated, events rejected) come with the result of every call and become the drop and truncation diagnostics in T-05.5 (REQ-DGN-001). They count items, never text. Nothing is emitted yet.

> **Planned (T-04.4):** what the shared HTTP client classifies and how, for the counters of T-05.5. **Outcome classes:** `success`, `transient`, `permanent`. **Reasons** (fixed texts, never a URL or a value): `status_<code>`, `network_error`, `timeout`, `tls_verification_failed`, `response_unreadable`, `batch_too_large`, `serialization_failed`, `client_failed`. **Timeouts:** 3000 ms by default (`TELEMETRY_HTTP_TIMEOUT_MS`); a server that never answers ends at it. **Retries:** at most `max_attempts` (5), waiting the equal-jitter backoff from 200 ms up to 30 s or the `Retry-After` if larger (cut to 24 h); the delivery mode decides who retries (T-05.3). Nothing is emitted yet.

> **Planned (T-04.2):** what the console destination shows and reports. **On the standard output:** one line per event written, as the canonical JSON of the envelope or as a readable line (`time severity [category] function: message key=value ...`); an event below the minimum severity (`INFO` by default) is not written and is counted as accepted. **In the result of a send** (for the counters of T-05.5): the reasons `write_failed` (the output could not be written), `serialization_failed` and `destination_failed`. A control character in a text line is shown as an escape, so a line is always one line.

> **Planned (T-04.3):** where the file is and how it rotates. The files are in the directory the project configured: `telemetry.jsonl` (the current one) and `telemetry.jsonl.1` ... `telemetry.jsonl.<max_files - 1>` (rotated, the newest first). The file rotates when the next line would pass `max_file_bytes` (10 MiB by default); the total of all the files is kept under `quota_bytes` (50 MiB by default) by deleting the oldest rotated file first. **Counters** (`FilesRotated`, `FilesDeletedForQuota`, `WriteErrors`, `RotationErrors`, `QuotaErrors`) are read with `Counters()` and become the diagnostics of T-05.5. **Reasons** in the result of a send: `write_failed`, `rotation_failed`, `serialization_failed`, `destination_failed`. A log shipper reads the current file and the rotated ones.

> **Planned (T-04.5):** what the HTTP destination reports in the result of a send (for the counters of T-05.5). **Accepted:** the events the receiver took (2xx) and the ones below the minimum severity (not sent). **Failed (transient, the worker may retry):** the reasons `status_408`, `status_429`, `status_5xx`, `timeout`, `network_error`, `tls_verification_failed` and `response_unreadable`. **Dropped (permanent, not retried):** `status_4xx` (other than 408 and 429), `status_3xx` (a redirect is never followed), `batch_too_large`, `serialization_failed`. **`destination_failed`:** an unexpected failure inside the destination. The full outcome (class, status, `Retry-After`) is available to the worker through `Deliver`. Nothing is emitted yet.

> **Planned (T-04.6):** what the AWS HTTP adapter reports in the result of a send. **Accepted:** the events whose request got a 2xx and the events that are not `log.*` (skipped: the `add_log` contract has no place for them; they are also in `Delivery.Skipped`). **Failed (transient):** the reasons of the shared client (`status_408`, `status_429`, `status_5xx`, `timeout`, `network_error`...). **Dropped (permanent):** `status_4xx`, `status_3xx`, `batch_too_large` and `serialization_failed` (an event that cannot be written; no request is made). `destination_failed` is the reason when the adapter itself fails. There is one request per `log.*` event, so a batch of N events is N requests.
