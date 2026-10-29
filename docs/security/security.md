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
