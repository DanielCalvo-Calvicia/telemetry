# Q01 — Scope, repository layout and rollout across the projects

**Status: answered by the maintainer on 2026-10-27; the points left open are in [Q07](Q07_open-points-after-the-answers.md).** Proposal: [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) (draft v0.1, nothing built). This questionnaire decides what the standard covers and how the projects adopt it.

Write your answer after `A:`; `UNKNOWN`, `NOT APPLICABLE` and `LATER` are valid; never write secrets, host names or account ids (the repository is public). Each question explains the point and has a **default**: an empty answer means the default is accepted.

---

1. **Which projects will adopt the standard, and in which order?** The standard is meant for every project. The first adopter drives the details (a hardware project needs the device module, an agent project needs the AI module, a plain service only needs logs).
   Options: (a) one pilot project per kind (a service, a device project, an AI agent project), then the rest (my default); (b) all projects at the same time; (c) logs first for all, the device and AI modules later.
   *Default: (a).*
   A:A, but now nobody uses this standaritzatuin until its done

2. **Where do `contracts/` and the Python implementation live?** REQ-ARC-009 asks for the same layers in Go and Python, and `contracts/` is the single source of truth (REQ-CNF-001).
   Options: (a) one repository with `contracts/`, `go/` and `python/` (my default: one pull request can change the contract and both languages, and the cross tests run together); (b) one repository per language and a third one for `contracts/`; (c) `contracts/` and Go here, Python in another repository.
   *Default: (a).*
   A:Those contracts will live here, then the different languages will import this project to use. This way it all is allocated in the same project and its easier.

3. **What happens to the current Go module and `pkg/logging`?** Today the module is `github.com/DanielCalvo-Calvicia/telemetry`, the public surface is `pkg/logging`, and the tags are `v0.1.0` and `v0.2.0` (ADR-002, ADR-003). The proposal says a core module plus optional submodules (REQ-DST-001).
   Options: (a) keep `pkg/logging` working as it is and add the new API beside it, then deprecate it later (my default); (b) a clean cut in a new major version (`v1.0.0`), with the old surface removed; (c) a new module path for the new standard, and this one frozen.
   *Default: (a).*
   A:The pkg/logging will disapear. Its already on a package so it doesnt matter much. 

4. **Who accepts changes to the contract?** A contract change touches every project at once.
   Options: (a) the maintainer alone decides; every change goes through a pull request and a dated note in an ADR (my default); (b) the maintainer plus one reviewer per language; (c) a written proposal (PR-nnn) is needed for every MINOR change too.
   *Default: (a).*
   A:Default

5. **How long does the old API stay supported after the new one exists?**
   Options: (a) until the last project has migrated, with a `Deprecated:` comment, and a `retract` only if a release is broken (my default); (b) a fixed time (write how long); (c) removed in the next major version.
   *Default: (a).*
   A:The old api will disapear without a problem. The only project that uses it it gets it from a package already build on github.

6. **Is the proposal split into several proposals?** It is large (core, devices, AI, delivery, security, parity).
   Options: (a) keep PR-001 as the umbrella and write one proposal per area when it is accepted (my default); (b) split now into PR-001 (contract and core), PR-002 (devices) and PR-003 (AI); (c) keep it as one document.
   *Default: (a).*
   A:Default
