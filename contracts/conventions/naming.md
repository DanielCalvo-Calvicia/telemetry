# Naming

The mechanical rule that keeps the names of Go and Python equivalent (REQ-ARC-010). A concept is **never renamed** in one language: the same words, in the form each language uses. Layers, directories, packages and adapters have the same name in both languages (REQ-ARC-009, REQ-ARC-011); only what the language forces differs (`__init__.py`, the `_test.go` suffix, `go/pkg/` against `python/src/`, the file extension).

## The three rules

1. **Go** exported identifiers (types, functions, methods, constants, options) are `PascalCase`. Locals, parameters, unexported fields and import aliases are descriptive `snake_case` (`CLAUDE.md`).
2. **Python** functions, methods, variables, modules and arguments are `snake_case`; classes are `PascalCase`.
3. **JSON** fields, event types and enum values are `snake_case` in both languages, and the Go struct tags do the mapping (Q02-7). An event type is `domain.entity.action` (REQ-CTR-001). The words of a name are the same in all four forms.

## How a name is derived

Take the words of the concept and join them in the form of each language.

| Concept | JSON, event type | Go | Python |
|---|---|---|---|
| the event type | `log.entry.recorded` | constant `EventTypeLogEntryRecorded` | constant `EVENT_TYPE_LOG_ENTRY_RECORDED` |
| the type of its data | | `LogEntryRecorded` | class `LogEntryRecorded` |
| its constructor | | `NewLogEntryRecorded` | function `new_log_entry_recorded` |
| a field of the envelope | `occurred_at` | field `OccurredAt` with the tag `json:"occurred_at"` | attribute `occurred_at` |
| a field of its data | `category` | field `Category` with the tag `json:"category"` | attribute `category` |

## Worked example: the event `log.entry.recorded`

The same logical event, seen in the three forms. The values are placeholders.

```json
{
  "event_type": "log.entry.recorded",
  "occurred_at": "2026-10-28T10:00:00.000000Z",
  "data": { "category": "http", "function": "handle_request", "message": "request received" }
}
```

Go:

```go
type LogEntryRecorded struct {
	Category string `json:"category"`
	Function string `json:"function"`
	Message  string `json:"message"`
}

func NewLogEntryRecorded(category string, function_name string, message string) LogEntryRecorded {
	return LogEntryRecorded{Category: category, Function: function_name, Message: message}
}
```

Python:

```python
class LogEntryRecorded:
    category: str
    function: str
    message: str


def new_log_entry_recorded(category: str, function: str, message: str) -> LogEntryRecorded: ...
```

## Public packages

The public packages follow `<functionality>/<adapter>` in both languages (REQ-ARC-012): `go/pkg/<functionality>/<adapter>` in Go and `telemetry.<functionality>.<adapter>` in Python, installed as `telemetry` (decided in T-01.1, 2026-10-28). The public package of the Go core is `go/pkg/telemetry`. Example: the SQLite spool is `go/pkg/spool/sqlite` and `telemetry.spool.sqlite`.

## Checks

T-12.3 compares the layer folders, the package and adapter names and the public names of the two languages by this rule. The Python rules until the Python standard of the organization can be read (Q07-8) are PEP 8 and the layers of the Go core.
