# T-08 — Devices and hardware

| | |
|---|---|
| Status | todo |
| Kind | epic |
| Parent | project |
| Depends on | T-02, T-03, T-05, T-07 |
| Blocks | — |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 (section 8 and Annex B) and the answers of [Q04](../../setup/questionnaires/Q04_devices-and-hardware.md) (hardware, sampling, offline devices, examples to be edited with the real projects), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (questions 5, 6 and 13: no Go pilot, Python 3.11.4 on the Raspberry Pi, the clock of a device) and [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (question 1: the two times, question 5: the SQLite destination).

## Goal

Section 8 of the specification: the generic `device.*` contracts and the typed source helpers, in Go and in Python. The library has no hardware drivers (D-05, Q04-3): the project that owns the hardware keeps its drivers and reports through the helpers. The first user is a Raspberry Pi project in Python (Q04-1).

## What it keeps and what it changes

- Keeps: the delivery of T-05 and the destinations of T-04 and T-06 (HTTP, file, SQLite, the spool) that the device events travel through, and the consumer's own drivers and objects.
- Changes: the old Go library in `pkg/logging` (README section 1) has no device concept. This epic adds the seven `device.*` event types in `contracts/schemas/device/`, the enums in `contracts/enums/device/`, one typed helper per source, sampling and batching for fast sensors and command correlation. It does not add `outbound/device_control/`: Q04-3 removed it.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-08.1 Device schemas and enums](T-08.1_device-schemas-and-enums.md) | todo |
| 2 | [T-08.2 Source helpers](T-08.2_source-helpers.md) | todo |
| 3 | [T-08.3 Sampling and batching](T-08.3_sampling-and-batching.md) | todo |
| 4 | [T-08.4 Command correlation](T-08.4_command-correlation.md) | todo |
| 5 | [T-08.5 Fixtures from Annex B](T-08.5_fixtures-from-annex-b.md) | todo |
| 6 | [T-08.6 Raspberry Pi pilot](T-08.6_raspberry-pi-pilot.md) | todo |
| 7 | [T-08.7 Edit the examples with the real projects](T-08.7_edit-the-examples-with-the-real-projects.md) | todo |

Order: 1; then 2 and 5 (5 needs T-02.8 too); 3 and 4 after 2, in parallel; 6 after 2 and T-06.5; 7 last. The pilot is the only task that needs the maintainer and the hardware.

## Left out on purpose (no over-engineering)

Hardware drivers and any dependency on a hardware library (D-05), safety restrictions (REQ-DEV-001: telemetry is not a safety mechanism), a command executor or a deadline enforcer (the consumer executes and honors the deadline), one schema per sensor model or manufacturer (REQ-DEV-004), and device metrics for raw readings (REQ-MET-004).

## Acceptance criteria (draft)

The real Raspberry Pi project reports through the library, and one UI can show every source with the same structure. Every example of Annex B validates against the schemas and gives the same bytes in both languages, and the documents describe what is built and mark what is planned.
