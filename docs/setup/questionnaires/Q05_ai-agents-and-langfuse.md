# Q05 — AI agents and Langfuse

**Status: answered by the maintainer on 2026-10-27; the points left open are in [Q07](Q07_open-points-after-the-answers.md).** Proposal: [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) (draft v0.1, nothing built). This questionnaire settles D-06 and D-09 and the scope of section 9 of the specification.

Write your answer after `A:`; `UNKNOWN`, `NOT APPLICABLE` and `LATER` are valid; never write secrets, host names or account ids (the repository is public). Each question explains the point and has a **default**: an empty answer means the default is accepted.

---

1. **Which agent projects exist today, in which language, and with which framework?** The proposal says the existing Python agent already uses Langfuse.
   A:yes, there is already an ai agent that uses langfuse. That project will use this new library.

2. **D-06: the initial list of frameworks.**
   Options: (a) generic callbacks and Langfuse, and LangChain or LlamaIndex only if a pilot project uses it (my default); (b) all three, as the proposal says; (c) write another list.
   *Default: (a).*
   A:B, in this case i prefer to have more and unused than left.

3. **D-09: how to integrate Langfuse.**
   Options: (a) the Langfuse SDK in Python and its HTTP/OTLP API in Go, with the same observable behavior (my default, if Go has no official SDK; this must be checked); (b) the HTTP/OTLP API in both languages (one design, more code); (c) the SDK in both, if it exists.
   *Default: (a).*
   A:Default

4. **What may be recorded about a prompt?** REQ-AI-006 records only metadata by default.
   Options: (a) metadata only; full prompts and tool results only when the consumer switches it on, with a size limit and redaction (my default); (b) the same, and also write which kinds of content are never allowed (for example personal data of customers).
   *Default: (a).*
   A:Default, that may change depending of the requirements.

5. **Cost and usage.** Is the token count enough, or does the library also report a cost per call?
   Options: (a) tokens only; the receiver calculates the cost (my default); (b) the library reports a cost from a price table the consumer gives.
   *Default: (a).*
   A:Default

6. **Approvals and policies.** `ai.approval.*` and `ai.policy.denied` assume agents that ask a human or a policy. Do your agents do that today?
   Options: (a) keep them in the contract, as they cost nothing if unused (my default); (b) leave them for a later version.
   *Default: (a).*
   A:Default. They already exist.
