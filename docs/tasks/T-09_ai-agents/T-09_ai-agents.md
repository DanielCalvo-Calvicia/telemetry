# T-09 — AI agents

| | |
|---|---|
| Status | todo |
| Kind | epic |
| Parent | project |
| Depends on | T-02, T-03, T-05, T-07 |
| Blocks | — |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 (section 9) and the answers of [Q05](../../setup/questionnaires/Q05_ai-agents-and-langfuse.md) (the existing agent, the frameworks, Langfuse, what may be recorded, usage, approvals), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (questions 14 and 15: the frameworks are Python only, the Go check of Langfuse becomes an ADR) and [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (questions 2 and 3: adapters are modules of their own, chosen in code).

## Goal

Section 9 of the specification: the generic `ai.*` contracts, the runtime callbacks, the Langfuse destination and the framework adapters (D-06), in Go and in Python. The first user is the existing Python agent that already uses Langfuse (Q05-1).

## What it keeps and what it changes

- Keeps: the execution model of T-07 (an agent run is an execution), the `AITelemetrySinkPort` and the delivery of T-03 and T-05, the masking of ADR-006 and the call context of the old library, migrated in T-03.8.
- Changes: the old Go library in `pkg/logging` (README section 1) has no AI concept. This epic adds the nine `ai.*` event types in `contracts/schemas/ai/`, the enums in `contracts/enums/ai/`, `outbound/ai_runtime/` (generic callbacks and hosted providers), `outbound/ai_telemetry/langfuse/` and the Python-only adapters for LangChain and LlamaIndex.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-09.1 AI schemas and enums](T-09.1_ai-schemas-and-enums.md) | todo |
| 2 | [T-09.2 Generic callbacks](T-09.2_generic-callbacks.md) | todo |
| 3 | [T-09.3 Privacy of prompts](T-09.3_privacy-of-prompts.md) | todo |
| 4 | [T-09.4 Hosted model providers](T-09.4_hosted-model-providers.md) | todo |
| 5 | [T-09.5 Check Langfuse support in Go](T-09.5_check-langfuse-support-in-go.md) | todo |
| 6 | [T-09.6 Langfuse adapter](T-09.6_langfuse-adapter.md) | todo |
| 7 | [T-09.7 LangChain adapter](T-09.7_langchain-adapter.md) | todo |
| 8 | [T-09.8 LlamaIndex adapter](T-09.8_llamaindex-adapter.md) | todo |
| 9 | [T-09.9 Agent pilot](T-09.9_agent-pilot.md) | todo |

Order: 1; then 2 and 5 in parallel (5 only needs 1); then 3, 4, 6 (needs 5 too), 7 and 8 in parallel, all after 2; 9 last, after 6 and T-06.5. The pilot is the only task that needs the maintainer.

## Left out on purpose (no over-engineering)

A cost calculation (the library reports token counts only, REQ-AI-012), one schema per framework, provider or model (REQ-AI-003), permission or policy enforcement (telemetry never grants permissions, REQ-AI-008), recording of full prompts by default (REQ-AI-006), and Go adapters for LangChain or LlamaIndex (Q07-14: "not applicable").

## Acceptance criteria (draft)

The existing agent reports model calls, tool calls, hand-offs, approvals and denials through the library, with only metadata unless the maintainer switches more on. Langfuse behaves the same from Go and from Python, the two frameworks are marked "not applicable" in Go in the registry, and every document that describes the AI module is true again.
