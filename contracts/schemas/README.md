# schemas

The JSON Schema (D-02) of the event envelope and of the `data` of each event type (REQ-CTR-001 to REQ-CTR-003). The schema is the canonical definition; the Go and Python types are generated from it or validated against it.

- `device/` holds the `device.*` schemas and `ai/` the `ai.*` schemas; they are separate domains and are never reused for each other (REQ-DEV-002, REQ-AI-001).
- One file per event type, named like the type (`log.entry.recorded.schema.json`); the envelope is one more file. The first schemas arrive with T-02.2.
- Empty for now.
