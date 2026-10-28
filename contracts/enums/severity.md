# Severity

The six values of `severity` (`severity.json`, section 5.3 of PR-001). They are uppercase in every event; a lowercase value is rejected. An implementation maps them to the native level of its logging system and to OpenTelemetry (section 5.3 of PR-001).

| Value | Meaning | Go `log/slog` | Python `logging` | OpenTelemetry severity number |
|---|---|---|---|---|
| `TRACE` | the most detailed flow, normally off; `bulk` priority in delivery | `slog.LevelDebug - 4` (`-8`, a custom level) | `5` (a custom level `TRACE` below `DEBUG`) | `TRACE` (1) |
| `DEBUG` | detail useful while developing or diagnosing; `bulk` priority | `slog.LevelDebug` (`-4`) | `logging.DEBUG` (`10`) | `DEBUG` (5) |
| `INFO` | normal operation: something that happened and is worth recording | `slog.LevelInfo` (`0`) | `logging.INFO` (`20`) | `INFO` (9) |
| `WARN` | something unexpected that the program handled; it may need attention | `slog.LevelWarn` (`4`) | `logging.WARNING` (`30`) | `WARN` (13) |
| `ERROR` | an operation failed; the program continues | `slog.LevelError` (`8`) | `logging.ERROR` (`40`) | `ERROR` (17) |
| `CRITICAL` | a failure that endangers the program or its data; `critical` priority in delivery | `slog.LevelError + 4` (`12`, a custom level) | `logging.CRITICAL` (`50`) | `FATAL` (21) |

Notes:

- The native levels of Go and Python have no `TRACE` (Go) or `CRITICAL` above `ERROR` (Go), so the table gives a custom level for them. The numbers of OpenTelemetry are the first of each range (`TRACE` 1 to 4, `DEBUG` 5 to 8, and so on); `CRITICAL` maps to `FATAL` (21 to 24).
- The delivery priority (`critical`, `normal`, `bulk`) of an event is decided by its type and, for `log.entry.recorded`, by its severity (section 10.3 of PR-001, T-05.2).
- The severity of an event never changes how it is masked or validated.
