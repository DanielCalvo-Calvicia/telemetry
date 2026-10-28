# contracts

The single source of truth between Go and Python ([PR-001](../docs/proposals/PR-001-telemetry-standard-go-python.md), REQ-CNF-001): the JSON Schemas of the events, the enums, the registries and catalogs, the naming convention and the layouts of the files that readers open. Nobody writes by hand a copy of an enum, an event type, an error code or a configuration schema in Go or Python: they are **generated** from this folder (T-02.4) and the generated code is committed.

## The folders

| Folder | What it holds | Filled by |
|---|---|---|
| [schemas/](schemas/README.md) | the JSON Schema of the envelope and of the `data` of every event type (`device/` and `ai/` for those domains) | T-02.2, T-07 to T-11 |
| [enums/](enums/README.md) | the enums: severity, execution states and transitions, the values of the device and AI domains | T-02.2, T-07 to T-09 |
| [events/](events/README.md) | the registry of event types (type, schema, capability, since which `schema_version`) | T-02.3 |
| [errors/](errors/README.md) | the catalog of error codes, identical in both languages (REQ-CNF-004) | T-02.3 |
| [config/](config/README.md) | the schema of the configuration: option names, defaults, profiles | T-03.6 |
| [capabilities/](capabilities/registry.yaml) | `registry.yaml`: each capability with its API, schemas, ports, adapters, state in each language and its conformance tests | T-02.7 |
| [conventions/](conventions/naming.md) | `naming.md`: the mechanical rule for the names of both languages | this folder |
| [storage/](storage/README.md) | the layouts of the files that a reader opens: the SQLite destination, the spool | T-06.1 |

## How a change is made

- **Only the maintainer approves a change**, by a pull request and a dated note in an ADR (Q01-4).
- **A contract change is implemented in both languages and released in both**, even one that only adds an optional field (see the release rule in [deployment.md](../docs/deployment/deployment.md)).
- A schema, an enum or a registry changes together with the generated code of both languages and its conformance test; CI fails if the generated code differs from the contract (T-02.4).
- The files are documentation or data: no code lives here. Examples use placeholders: no secret, no internal host, no account id.

## State

The layout and the naming rule (T-02.1) and the envelope schema with the severity and actor-kind enums (T-02.2) exist. No event-type schema, registry, catalog or generator exists yet.
