# Prompts to start, pause and resume the build

Three prompts for a Claude Code session in `D:\Snowfactory\Programas\GO\telemetry`. Paste one as your message. They work with the protocol in [README.md](README.md) and keep the progress in `CHECKPOINT.md` (written by the PAUSE prompt). Replace "T-01" by the epic you start.

---

## 1. START

```text
You are going to build the telemetry standard of this repository (PR-001, docs/proposals), starting with epic T-01 in docs/tasks, step by step, following the task files exactly. Work in Spanish or English as I write to you; code, comments and docs are in English.

## Read first (in this order, before touching anything)
1. docs/tasks/README.md: structure, execution protocol (section 2), baseline definition of done (section 3), facts (section 4), status board (section 5).
2. docs/tasks/T-01_repository-foundation/T-01_repository-foundation.md and the task you are about to do.
3. docs/proposals/PR-001-telemetry-standard-go-python.md, docs/decisions/ADR-011, ADR-012 and ADR-013, and CLAUDE.md. They are the source of truth. When two questionnaires in docs/setup/questionnaires disagree, the higher number wins.
4. Your memory (MEMORY.md) and, if it exists, docs/tasks/CHECKPOINT.md (if it exists, stop and tell me: I should have used the RESUME prompt).

## How to work
- Start with the first task whose status is `todo` and whose dependencies are `done`. T-01.1 is a decision for me: if PR-001 is not accepted yet, ask me and wait.
- Before Go code, apply the go-architecture-standard skill. Before Python code, follow the rules of the architecture document.
- Create or modify ONLY the files the task lists. If you need another file, dependency or folder, update the task first and tell me.
- After each task: run its Verification and the baseline of README section 3 in every language the task names. Only then set the task to `done` in its file, in the board of docs/tasks/README.md and in ORDER.md, and write deviations or new facts in the task's Notes.
- Update the documents the task lists and the ones CLAUDE.md requires, in the same piece of work. Check that the links resolve.

## Hard rules
- Do not commit, push, tag, publish or open a pull request unless I ask. A tag is permanent.
- The repository is public: no secret, no internal host name or IP, no customer name. The switch run_against_deployed_service of the e2e test stays false in anything committed.
- Do not delete or rewrite files outside the current task. Never edit generated files by hand.
- If something is unclear, a decision is missing, or a task contradicts another, STOP and ask me before editing tasks or docs. Edit task files only for decisions I made.
- Do not invent data or decisions. No insults. Keep answers short; report at the end of each task: what changed, the verification result (exact failures if any), the next task.

## Stop points
Stop and report after each finished epic, or immediately when: a verification fails twice, a decision is needed, a task cannot be completed as written, or you have used about 70% of the context. When you stop for context or because I ask, follow the PAUSE procedure (write docs/tasks/CHECKPOINT.md) before ending.

Begin now: state which task you are starting and why, then do it.
```

---

## 2. PAUSE

```text
PAUSE the build now. Do not start anything new. Finish or safely park the current step, then leave the repository and the plan in a state that another session can resume without asking me anything.

1. Finish only what is already half-written so every module builds (go build ./... in each Go module, and the Python package imports). If a file cannot be finished, revert your own uncommitted change to that file only and note it. Never revert anything you did not write in this build.
2. Run and record the baseline of docs/tasks/README.md section 3 for the languages you touched. Write the exact output of any failure.
3. Set every task's Status honestly (`done`, `in-progress`, `todo`, `blocked`, `awaiting-user`, `deferred`) in its own file, in the board of docs/tasks/README.md and in ORDER.md. A task is `done` only if all its acceptance criteria passed.
4. Write docs/tasks/CHECKPOINT.md (replace it if it exists) with exactly these sections:
   - Date and time, git branch, output of `git status --short` (grouped: created, modified, deleted by this build).
   - Current task ID and step you were on, and why you stopped.
   - Done since the start: task IDs with a one-line result each.
   - In progress: files touched, what is missing, the exact next command or edit.
   - Verification state: the baseline commands and their results.
   - Decisions taken and deviations from the task files (each with the file where it is recorded).
   - Open questions for me (numbered, each with your recommended answer).
   - New dependencies added (name, version, module) and the go.mod, go.work and pyproject state.
   - Warnings for the next session (things that look done but are not, flaky tests, files not to touch).
   - Next three tasks in order.
5. Save durable decisions or discoveries with Engram mem_save if they are not already in the repository. Do not save what the repo already records.
6. Do not commit or push. Reply with a short summary (max 15 lines): where you stopped, what is verified, what is pending, and the one line I must paste to resume.
```

---

## 3. RESUME

```text
RESUME the build of the telemetry standard from where the last session paused. Do not redo finished work and do not ask me anything that the checkpoint already answers.

1. Read docs/tasks/CHECKPOINT.md completely, then docs/tasks/README.md (protocol, baseline, board), the file of the current task and its parents, PR-001 and ADR-011 to ADR-013. Read MEMORY.md too.
2. Verify the world matches the checkpoint before continuing:
   - `git status --short` and `git branch --show-current` against the checkpoint. If files changed that the checkpoint does not mention, list them and ask me before touching them (they may be my edits).
   - Run the baseline of README section 3 for the languages involved and compare with the recorded results. Fix regressions first, in the task that owns them.
   - Check the board and ORDER.md against the task files; correct mismatches and tell me.
3. Answer nothing on my behalf: the "Open questions" of the checkpoint are mine. Ask them now, in one message, each with your recommendation, unless I already answered them in this message. Continue with everything that does not depend on an unanswered question.
4. Continue with the task and step named in the checkpoint, following the same protocol as at the start.
5. Same hard rules: no commit, push or tag unless I ask; stop and ask when a decision is missing; edit task files only for decisions I made.
6. When you finish this session's work or reach about 70% of the context, follow the PAUSE procedure (rewrite docs/tasks/CHECKPOINT.md, do not append to the old one).

Begin now: tell me in five lines what the checkpoint says (current task, last result, open questions, what you will do first), then continue.
```
