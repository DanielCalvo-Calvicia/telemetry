# Q02 — The contract: format, identifiers, events and generation

**Status: answered by the maintainer on 2026-10-27; the points left open are in [Q07](Q07_open-points-after-the-answers.md).** Proposal: [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) (draft v0.1, nothing built). This questionnaire settles D-02 and D-04 and the open points of section 5 of the specification.

Write your answer after `A:`; `UNKNOWN`, `NOT APPLICABLE` and `LATER` are valid; never write secrets, host names or account ids (the repository is public). Each question explains the point and has a **default**: an empty answer means the default is accepted.

---

1. **D-02: format of the contract.**
   Options: (a) JSON Schema, with the types generated or validated from it (my default: it is what the proposal says, it is human-readable and both languages have mature tools); (b) protobuf (smaller and faster, but the JSON output of REQ-CNF-003 would need a second mapping); (c) JSON Schema for the events and protobuf only for a future binary transport.
   *Default: (a).*
   A:Default

2. **D-04: event identifier.**
   Options: (a) UUIDv7 (time-ordered, my default); (b) UUIDv4 (random, no ordering); (c) ULID.
   *Default: (a).*
   A:Default

3. **Which tool generates the code from `contracts/`?** REQ-CNF-002 requires enums, event types, error codes and the configuration schema to be generated, and CI to fail when they differ.
   Options: (a) one small generator written in this repository (in Go) that emits both languages (my default: no new tool to learn and one place for the rule); (b) the off-the-shelf generator of each language; (c) decide when the first schema exists.
   *Default: (a).*
   A:Default

4. **How does today's `logging.Entry` map to an event?** Today an entry has `Application`, `Category`, `Function`, `Message` and the call context.
   Options: (a) a `log.entry.recorded` event whose `data` holds category, function and message, with the application going to `resource.service_name` (my default); (b) a different event type per category; (c) logs stay outside the envelope and are only wrapped when sent to HTTP.
   *Default: (a).*
   A:Default

5. **The first list of event types.** The proposal names `log.*`, `metric.*`, `trace.*`, `execution.*`, `fault.*`, `report.*`, `audit.*`, `device.*`, `ai.*` and the health events. Which are in the first release?
   Options: (a) `log.*`, `execution.*`, `fault.*` and `report.*` first, the rest in the order the pilot projects need them (my default); (b) all of them in v1, as the proposal says; (c) write which ones.
   *Default: (a).*
   A:B ,but it can be done one by one, when one is finished comes the next.
   > **Update 2026-10-28:** The build order of the capabilities is in Q07-16.

6. **`schema_version` and its first value.**
   Options: (a) start at `0.1` while the library is `v0.x` and move to `1.0` with the first stable release (my default); (b) start at `1.0` now.
   *Default: (a).*
   A:Default

7. **Field names in the JSON.** The proposal uses `snake_case` (`event_id`, `service_name`). `CLAUDE.md` asks for descriptive `snake_case` for locals and the Go convention for exported identifiers. Is the JSON `snake_case` everywhere, with the Go struct tags doing the mapping?
   Options: (a) yes (my default); (b) `camelCase` in the JSON.
   *Default: (a).*
   A:Default, but the go must follow the standard.
