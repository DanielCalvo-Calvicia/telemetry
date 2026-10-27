# Environment variables

The library **never reads the environment** ([ADR-004](../decisions/ADR-004-library-never-reads-the-environment.md)). `logging.Config` carries the env tags (`env:"NAME,default=..."`, the format of `sethvargo/go-envconfig`); the consumer embeds `Config` in its own configuration struct and loads it in its single configuration pass. This document lists the variable names that tags define, so a consumer can set them in its `.env`, its Kubernetes Secret or its pipeline. `.env.example` holds the same list.

`app_env` is not a `Config` field: the consumer passes it to `New` (usually from its own `APP_ENV`). It selects the minimum level: `development` from DEBUG up, `production` from INFO up. **There is no log-level variable** ([ADR-007](../decisions/ADR-007-minimum-level-follows-the-environment.md)).

Every sink that is switched on receives every entry that passes the level. The console is also the fallback that reports a sink that failed.

## Console sink

| Variable | Default | Field | Notes |
|---|---|---|---|
| `LOGGER_CONSOLE_ENABLED` | `true` | `ConsoleEnabled` | one JSON line per entry on the standard output |

## HTTP sink (generic log receiver)

| Variable | Default | Field | Notes |
|---|---|---|---|
| `LOGGER_HTTP_ENABLED` | `false` | `HTTPEnabled` | posts the flat JSON entry |
| `LOGGER_HOST` | `http://localhost` | `Host` | scheme included |
| `LOGGER_PORT` | `3000` | `Port` | |
| `LOGGER_PATH` | `/logs` | `Path` | |

The request goes to `LOGGER_HOST:LOGGER_PORT` + `LOGGER_PATH`. When the sink is enabled, an empty host, port or path stops the construction with `ErrLogSinkEndpointMissing`.

## AWS HTTP sink (the `add_log` endpoint of aws_microservice, CloudWatch Logs)

| Variable | Default | Field | Notes |
|---|---|---|---|
| `LOGGER_AWS_HTTP_ENABLED` | `false` | `AWSHTTPEnabled` | adapts every entry to the `add_log` contract |
| `LOGGER_AWS_HTTP_HOST` | `http://localhost` | `AWSHTTPHost` | scheme included |
| `LOGGER_AWS_HTTP_PORT` | `8080` | `AWSHTTPPort` | |
| `LOGGER_AWS_HTTP_PATH` | `/api/v1/cloudwatch/logs/add` | `AWSHTTPPath` | |
| `LOGGER_AWS_HTTP_LOG_GROUP` | none | `AWSHTTPLogGroup` | required when the sink is enabled; the endpoint creates it when it does not exist |
| `LOGGER_AWS_HTTP_LOG_STREAM` | none | `AWSHTTPLogStream` | required when the sink is enabled; the endpoint creates it when it does not exist |

An empty host, port or path stops the construction with `ErrLogSinkEndpointMissing`; an empty log group or log stream with `ErrLogSinkAWSTargetMissing`.

## Masking

| Variable | Default | Field | Notes |
|---|---|---|---|
| `LOGGER_MASK_EXTRA_PATTERNS` | empty | `MaskExtraPatterns` | regular expressions added to the built-in detectors, separated by `;` (a comma would split a pattern such as `\d{2,5}`); an invalid expression stops the construction |

A consumer that deploys with a list that can be empty should send the variable only when it is set: an empty value may be read as one empty pattern.

## Limits that are not variables

| Value | Where | Why fixed |
|---|---|---|
| 3 s | request timeout of the HTTP and AWS HTTP sinks | a slow receiver must not slow the caller; there is no retry, a failed entry is dropped and reported |
| 1 | `StartingIndentation` | the indentation of the inbound layer |
