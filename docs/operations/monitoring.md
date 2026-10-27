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
