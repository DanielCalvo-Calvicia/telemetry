# Q06 — Security, configuration and release

**Status: answered by the maintainer on 2026-10-27; the points left open are in [Q07](Q07_open-points-after-the-answers.md).** Proposal: [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) (draft v0.1, nothing built). This questionnaire settles D-07 and D-08, the configuration model and how the standard is released.

Write your answer after `A:`; `UNKNOWN`, `NOT APPLICABLE` and `LATER` are valid; never write secrets, host names or account ids (the repository is public). Each question explains the point and has a **default**: an empty answer means the default is accepted.

---

1. **D-07: language of the documentation.** `CLAUDE.md` already says code, comments and documents are in English.
   Options: (a) English for everything in this repository (my default; D-07 is then closed); (b) English for the code and Spanish for the specification.
   *Default: (a).*
   A:Default

2. **D-08: the minimum Python version.**
   Options: (a) 3.10 (my default, as the proposal says); (b) 3.11 or 3.12 (newer typing and `tomllib`; check what the projects run).
   *Default: (a), after you tell me the lowest version a project runs.*
   A:The python will be 3.1X.X, that is because some hardware may use old versions.

3. **The library and the environment.** ADR-004 says the library never reads the environment, and the consumer loads `Config`. The proposal (REQ-ARC-007, REQ-CFG-003) allows reading it in the configuration layer, and adds a file and profiles.
   Options: (a) keep ADR-004: the library never reads the environment or a file, the consumer loads the configuration and passes it; the shared model is a schema and the profiles are functions (my default); (b) change ADR-004: an optional loader in the configuration layer reads the file and the environment.
   *Default: (a).*
   A:Default

4. **Configuration file format.** Only if the answer to question 3 is (b).
   Options: (a) YAML; (b) TOML; (c) JSON.
   *Default: (a).*
   A:

5. **The default redaction list.** Today the masking of ADR-006 detects keys, tokens, JWTs, private keys, passwords and credential pairs.
   Options: (a) the new list (`password`, `token`, `authorization`, `api_key`, `secret`) is added to the current detectors and nothing is removed (my default); (b) the list replaces the current detectors.
   *Default: (a).*
   A:Default

6. **Audit events.** REQ-SEC-006 separates `audit.*` from logs and says HTTP or a file do not give legal retention. Does any project need audit events in v1?
   Options: (a) no; the contract is written and the adapter comes later (my default); (b) yes (write which project and what it must prove).
   *Default: (a).*
   A:Default

7. **Tests and CI for Python.** The Go coverage gate is 95% without the `external/` folders.
   Options: (a) the same 95% for Python with the same exclusion, plus the cross tests of REQ-CNF-006 in CI (my default); (b) write another number.
   *Default: (a).*
   A:Default

8. **Releases.** Today a release is a tag on `main` (ADR-010).
   Options: (a) one tag per language (`go/vX.Y.Z` for the Go module and `python-vX.Y.Z` for the package), and the contract versioned by `schema_version` (my default, as REQ-VER-001 says); (b) one tag for both.
   *Default: (a).*
   A:Default
