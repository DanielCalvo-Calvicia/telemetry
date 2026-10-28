# T-10 — Metrics and traces

| | |
|---|---|
| Status | todo (T-10.4 and T-10.5 deferred) |
| Kind | epic |
| Parent | project |
| Depends on | T-03, T-05 |
| Blocks | — |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 (sections 6.2, 6.3 and 7.2) and the answers of [Q08](../../setup/questionnaires/Q08_last-points-before-rewriting-the-proposal.md) (Q08-8: no metrics or tracing backend today), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (Q07-16: metrics and traces come fifth in the build order), [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (Q09-2 and Q09-3: adapters are modules of their own, chosen in code) and [Q02](../../setup/questionnaires/Q02_contract-format-identifiers-and-generation.md) (Q02-5: one capability at a time).

## Goal

Give both languages the in-library API of metrics (section 6.2) and traces with W3C Trace Context (section 6.3). No metrics or tracing backend exists today (Q08-8), so the API, its limits and the cross tests are built in their turn, and the OpenTelemetry and Prometheus adapters wait for a project that has a backend.

## What it keeps and what it changes

- Keeps: everything the earlier epics build (core, delivery, diagnostics). The old Go logging library described in [README.md](../../../README.md) has no metrics and no tracing, so nothing existing is replaced.
- Adds: the metrics API (counters, up/down counters, gauges, histograms, duration) with a cardinality limit; the traces API (spans, parent and child, error state); W3C Trace Context over HTTP; the outbound ports `MetricsSinkPort` and `TraceSinkPort` in both languages; the cross test of a trace that crosses from Go to Python.
- Later, when a backend exists: the OpenTelemetry and Prometheus adapters, each a module of its own in Go and an extra in Python.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-10.1 Metrics API](T-10.1_metrics-api.md) | todo |
| 2 | [T-10.2 Traces API](T-10.2_traces-api.md) | todo |
| 3 | [T-10.3 W3C Trace Context](T-10.3_w3c-trace-context.md) | todo |
| 4 | [T-10.6 Cross tests](T-10.6_cross-tests.md) | todo |
| 5 | [T-10.4 OpenTelemetry adapter](T-10.4_opentelemetry-adapter.md) | deferred |
| 6 | [T-10.5 Prometheus adapter](T-10.5_prometheus-adapter.md) | deferred |

Order: 1 and 2 can run in parallel (they share only the core of T-03.9); 3 needs 2; 4 needs 3. The two adapters wait for a project with a backend and are not part of the first pass: T-10.4 needs 1 and 2, T-10.5 needs only 1. The IDs keep their numbers; the table lists the deferred ones last.

## Left out on purpose (no over-engineering)

A metrics store or query API, sampling of spans, a tracing UI, exemplars, baggage propagation, an adapter for a backend nobody runs, and raw high-frequency readings as metrics (REQ-MET-004: device measurements are events).

## Acceptance criteria (draft)

Metrics and spans work in both languages without a backend, with the limits of REQ-MET-003; a trace that starts in Go and continues in Python keeps one trace id; an incoming trace context is never authorization; the two adapters are built only when a backend exists.
