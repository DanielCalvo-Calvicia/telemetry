# Security

## What the library protects

- **Sensitive data in log lines.** Every entry is masked once, before any sink ([ADR-006](../decisions/ADR-006-masking-before-every-sink.md)): AWS access keys, bearer credentials, JWTs, private key blocks, passwords in connection strings, credential pairs, and the value of fields whose name says it is sensitive (`password`, `secret`, `token`, `authorization`, `cookie`, `body`, `payload`, and others in `SensitiveFieldNames`). The report of a failing sink never repeats the message or the fields of the dropped entry.
- **Limits of the masking.** Detection is by pattern and by field name. A secret of an unusual shape inside free text is not recognized: add a regular expression with `LOGGER_MASK_EXTRA_PATTERNS`, and do not log secrets on purpose.

## The library itself

- It holds **no credential**. The receivers are reached by address only; CloudWatch Logs is reached through aws_microservice, which owns the AWS access.
- It never reads the environment or files ([ADR-004](../decisions/ADR-004-library-never-reads-the-environment.md)).
- The HTTP sinks send plain HTTP when the host says `http://`: a consumer that logs across a network it does not trust should use `https://` hosts.
- A request is abandoned after 3 s; the answer body is never interpreted.

## Public repository: what must never be pushed

Everything pushed is permanent (the Go proxy caches versions forever). Before a push or a tag:

- [ ] Company approval to publish was obtained (confirmed by the maintainer before the first push)
- [ ] No secret in code, tests, fixtures, documents, `.env.example` or commit messages (run `gitleaks detect --source . --log-opts="--all"`; it was not installed on the maintainer's machine, so the check made so far was a search of the tree and of the history for addresses and key-shaped values)
- [ ] No internal host names, IP addresses, account ids, customer names or internal links; examples use placeholders (`http://localhost`, `my-log-group`)
- [ ] Test values are fake (the AWS documentation key `AKIAIOSFODNN7EXAMPLE` is the only key-shaped value)
- [ ] No binaries (a debugger build `__debug_bin*` is git-ignored)
- [ ] The inputs of `internal/tests/e2e/deployed_aws_microservice_test.go` are placeholders and its switch is `false`

## Supply chain

- One third-party dependency: `github.com/google/uuid`.
- The CI workflow pins every action by commit SHA (checkout, setup-go and setup-python) and gives the `GITHUB_TOKEN` read-only permissions. The tools it installs are pinned by version (`goimports`, `ruff`, `coverage`); the workflow holds no secret, host or account id.
- Consumers pin a tag, commit `go.sum` and use `GOFLAGS=-mod=readonly` in CI; a bad release is retracted, never moved ([ADR-010](../decisions/ADR-010-no-deployment-release-by-tags.md)).
- `LICENSE` is MIT.

## Validation errors never carry the value (planned, T-02.6)

The validator of the new standard (built, not yet called from a logging path) puts only the event type, the field path and a fixed reason in its error, never the value that failed: a value may be a secret. A hostile field name is cut to 64 characters and its control characters are replaced, so it cannot forge a log line. A test feeds a fake secret-looking value and checks it never appears in a message.

## Reporting a vulnerability

Privately, through GitHub's private vulnerability reporting of the repository (see `SECURITY.md`). Not through a public issue.

## Masking in the new core (built, T-03.4)

- **What the masker protects:** `data`, `attributes` and `context` of every envelope, at any depth, by the six detectors of ADR-006 (AWS access keys, bearer credentials, JWTs, private key blocks, passwords in connection strings, credential pairs), by the default list of field names and by what the consumer adds. The resource and the actor are identities and are not masked.
- **Limits that remain:** detection is by pattern and by name; a secret of an unusual shape inside free text is not recognized until the consumer adds a pattern. `body`, `content` and `payload` are always hidden by name (an open question). A consumer that replaces the default list of names removes that protection for its own names: the detectors stay on.
- **A failing mask drops the event**, never sends it unmasked. The error never holds the value.
- **Checklist before a push:** the masking fixtures hold only fake credentials (AWS documentation examples and obvious placeholders); no real key, token or host.
- **Planned:** the guarantee that no sink sees an unmasked event is complete when the chain is wired (T-03.9); the tests build the chain by hand until then.

## Limits as a protection against oversized payloads (built, T-03.5)

- An event is rejected above 64 KiB of canonical JSON and the attributes are bounded (32 keys, 64 and 256 characters), so a caller cannot make the library build an unbounded event. A batch for HTTP is bounded at 1 MiB by the batcher using `fits_in_batch`.
- The limits are applied after masking, so a secret is never cut in half and left half visible. The counters and the error messages hold sizes and counts, never the cut text.

## Configuration (built, T-03.6)

- No field of the core configuration is a secret and none has a credential as default; the secrets of an adapter (a token, a password) are fields of that adapter, given by the consumer in code (REQ-CFG-005, REQ-SEC-007). `.env.example` holds fake values only.
- The validation errors name the field and a fixed reason and **never the configured value**.
- The library reads no environment and no file for its configuration; a test fails if a source mentions the environment.

## Errors carry no value (built, T-03.7)

- A message names a field path or an event type (cut to 64 characters, control characters replaced), never a value that the caller gave: a value may be a secret. A test in each language makes 29 real failures with a secret-looking value and fails if it appears in a message.
- The cause of an error that a consumer's own adapter built is kept in the chain and may hold whatever that adapter put there: an adapter must not put a value in its errors.

## The shared HTTP client (built, T-04.4)

- **TLS verification is on by default.** A server with an untrusted certificate is refused; only the explicit setting `skip_tls_verification` (`TELEMETRY_HTTP_SKIP_TLS_VERIFICATION`) turns it off, and it is documented as leaving it unset.
- **Tokens and keys.** The bearer token and the API key value come from the settings the consumer passes in, have no default, are never written in a versioned file (`.env.example` holds them empty) and never appear in an outcome, an error, a log or a diagnostic, even when the URL carries a secret in its query: transport errors are translated to fixed reasons. A value with a line break or a space is refused (header injection). A token is never sent over plain `http` except to `localhost`.
- **A redirect is never followed**, so a response cannot send the token to another host. The client uses no proxy from the environment.
- **A body over the batch limit is never sent** (1 MiB by default).
- **The test certificate** of the Python TLS tests is generated at test time in a temporary directory with `openssl` and deleted; no key or certificate is committed.

## The console destination (built, T-04.2)

- **No log injection.** In the readable format a line break, a carriage return, a tab, the escape character and any other control character of the category, the function, the message or an attribute key are written as escapes, and the attribute values in their JSON form: one event cannot split a line, forge another or colour the terminal.
- **It writes only what the masked event holds.** The destination sits after the masking of the chain (ADR-006) and prints no resource or id that is not in the event line.

## The file destination (built, T-04.3)

- **Permissions.** The files are created readable and writable by the owner only (`0600`) and the directory it creates `0700`. The mode is set when the file is created: an existing directory or file keeps its own mode (the project owns it).
- **What the file holds.** Masked events only, as canonical JSON: the destination sits after the masking of the chain (ADR-006). A project that ships the file must treat it as it treats its logs.
- **No path from the library.** The directory comes from the consumer; the library fixes no path. The file name may not hold a path separator, so a setting cannot leave the directory.
- **A start error never shows a path.** A directory that cannot be written is reported as one problem on `directory` with a fixed text, no path and no operating system message.
- **The quota bounds the disk use** (50 MiB by default), so a flood of events cannot fill the disk through this destination.
