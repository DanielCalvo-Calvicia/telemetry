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


## Core configuration of the new standard (built, T-03.6)

The new core (`go/`, `python/`) has **one logical model of the configuration** ([contracts/config/config.schema.json](../../contracts/config/config.schema.json)) and it **never reads the environment or a file** (ADR-004, REQ-ARC-007): the consumer loads these values with its own tool and passes them in. The names below are the `x-env` names of the schema, the `env` tags of the Go `Config` and the keys of the Python `ENVIRONMENT_VARIABLES`; a test keeps the schema, the tags, this table and `.env.example` equal.

- **Precedence, lowest to highest** (REQ-CFG-003): the defaults, the **profile** (`microservice`, `cronjob`, `worker`, `robotics`, `offline_device`: functions that return a `Config`; they only change the delivery mode, REQ-CFG-004), then the options given in code. **The tags carry no `default=`** on purpose: a default in a tag would override the profile when the consumer loads the environment after choosing it. The defaults are the ones of this table and live in one place, the code. Start from a profile, then load the environment over it.
- **Validation at start** (REQ-CFG-002): an invalid configuration fails with every problem together, each with the field path and a reason, and with the code of the catalog: `config_field_invalid`, or `delivery_mode_unknown` for the delivery mode. The codes are the same in Go and Python (REQ-CNF-004).
- **An empty text means not set.** An empty `TELEMETRY_INSTANCE_ID` makes the core generate one at start. No field is a secret and none has a credential as default (REQ-CFG-005); the secrets of an adapter belong to the adapter.
- **`durable`** is accepted by the validation, which also requires `spool.directory`; the composition root refuses to start a durable destination without a spool port (T-03.9).

| Variable | Default | Field | Notes |
|---|---|---|---|
| `TELEMETRY_SERVICE_NAME` | none (required) | `resource.service_name` | logical name of the service |
| `TELEMETRY_INSTANCE_ID` | empty | `resource.instance_id` | empty: the core generates one at start |
| `TELEMETRY_ENVIRONMENT` | empty | `resource.environment` | prod, staging, ... |
| `TELEMETRY_SERVICE_VERSION` | empty | `resource.service_version` | version of the service |
| `TELEMETRY_HOST` | empty | `resource.host` | host name |
| `TELEMETRY_TIME_ZONE` | empty | `resource.time_zone` | IANA name, display only (ADR-012) |
| `TELEMETRY_DELIVERY_MODE` | the profile: `best_effort` (microservice), `retry` (cronjob, worker), `durable` (robotics, offline_device) | `delivery.mode` | `best_effort`, `retry` or `durable` |
| `TELEMETRY_QUEUE_SIZE` | `10000` | `delivery.queue_size` | events in the bounded queue |
| `TELEMETRY_BATCH_SIZE` | `100` | `delivery.batch_size` | not greater than the queue size |
| `TELEMETRY_FLUSH_INTERVAL_MS` | `2000` | `delivery.flush_interval_ms` | milliseconds |
| `TELEMETRY_TIMEOUT_MS` | `3000` | `delivery.timeout_ms` | milliseconds; no network operation waits for ever (REQ-DLV-004) |
| `TELEMETRY_MAX_ATTRIBUTE_KEYS` | `32` | `limits.max_attribute_keys` |  |
| `TELEMETRY_MAX_ATTRIBUTE_KEY_CHARS` | `64` | `limits.max_attribute_key_chars` | characters |
| `TELEMETRY_MAX_ATTRIBUTE_VALUE_CHARS` | `256` | `limits.max_attribute_value_chars` | characters |
| `TELEMETRY_MAX_EVENT_BYTES` | `65536` | `limits.max_event_bytes` | bytes of the canonical JSON |
| `TELEMETRY_MAX_BATCH_BYTES` | `1048576` | `limits.max_batch_bytes` | not less than the event limit |
| `TELEMETRY_SPOOL_DIRECTORY` | none | `spool.directory` | required when the delivery mode is `durable`; the consumer gives it |
| `TELEMETRY_SPOOL_QUOTA_BYTES` | `268435456` | `spool.quota_bytes` | 256 MiB per destination |
| `TELEMETRY_MASK_EXTRA_PATTERNS` | empty | `masking.extra_patterns` | regular expressions of the subset common to Go and Python, separated by `;` |
| `TELEMETRY_MASK_FIELD_NAMES` | not set | `masking.field_names` | when set (even empty) REPLACES the default list of sensitive names; send it only when it is set |
| `TELEMETRY_MASK_EXTRA_FIELD_NAMES` | empty | `masking.extra_field_names` | names added to the active list, separated by `,` |
| `TELEMETRY_MASK_FIELD_CLASSES` | empty | `masking.field_classes` | `field:class` pairs separated by `,` |
| `TELEMETRY_MASK_CLASS_RULES` | empty | `masking.class_rules` | `class:rule` pairs separated by `,`; the rule is `drop`, `redact` or `allow` |

The adapters of the sinks (HTTP, file, spool, ...) add their own variables in their own modules (T-04 and later).


## Shared HTTP client (built, T-04.4)

The settings that every HTTP adapter shares ([contracts/config/http_client.schema.json](../../contracts/config/http_client.schema.json)). Like the core configuration, **the library never reads the environment**: the consumer loads these values with its own tool and passes them in. The names are for **one** HTTP destination; an adapter with several destinations gives each its own prefix. The defaults live in the code (a tag has no `default=`, see the core configuration above); **the two secrets (`TELEMETRY_HTTP_BEARER_TOKEN`, `TELEMETRY_HTTP_API_KEY_VALUE`) have no default, are never written in a versioned file and never appear in an error, a log or a diagnostic** (REQ-SEC-007, REQ-CFG-005). TLS verification is **on**; only an explicit `TELEMETRY_HTTP_SKIP_TLS_VERIFICATION=true` turns it off (REQ-SEC-005). A test in each language keeps the schema, the code, this table and `.env.example` equal.

| Variable | Default | Field | Notes |
|---|---|---|---|
| `TELEMETRY_HTTP_URL` | none (required) | `url` | the destination; http or https; authentication over plain http only to localhost |
| `TELEMETRY_HTTP_METHOD` | `POST` | `method` | `POST`, `PUT` or `PATCH` |
| `TELEMETRY_HTTP_TIMEOUT_MS` | `3000` | `timeout_ms` | milliseconds |
| `TELEMETRY_HTTP_COMPRESS` | `false` | `compress` | gzip with `Content-Encoding: gzip` |
| `TELEMETRY_HTTP_SKIP_TLS_VERIFICATION` | `false` | `skip_tls_verification` | turns the TLS verification OFF: leave it unset |
| `TELEMETRY_HTTP_BEARER_TOKEN` | none | `bearer_token` | a secret: the consumer gives it; never in a file of a repository |
| `TELEMETRY_HTTP_API_KEY_HEADER` | none | `api_key_header` | the header name; needs the value |
| `TELEMETRY_HTTP_API_KEY_VALUE` | none | `api_key_value` | a secret; needs the header name; not together with a bearer token |
| `TELEMETRY_HTTP_MAX_ATTEMPTS` | `5` | `max_attempts` | attempts at most, the first included |
| `TELEMETRY_HTTP_RETRY_BASE_DELAY_MS` | `200` | `retry_base_delay_ms` | milliseconds |
| `TELEMETRY_HTTP_RETRY_MAX_DELAY_MS` | `30000` | `retry_max_delay_ms` | milliseconds; not below the base |


## Console destination (built, T-04.2)

The settings of the console destination ([contracts/config/console.schema.json](../../contracts/config/console.schema.json)). The library never reads the environment: the consumer loads these values and passes them in; the defaults live in the code. **The level does not follow `APP_ENV` any more** (ADR-007 is for the old library): the consumer chooses the minimum severity of each destination.

| Variable | Default | Field | Notes |
|---|---|---|---|
| `TELEMETRY_CONSOLE_FORMAT` | `json` | `format` | `json` (the canonical JSON of the envelope, one per line) or `text` (a readable line) |
| `TELEMETRY_CONSOLE_MINIMUM_SEVERITY` | `INFO` | `minimum_severity` | `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR` or `CRITICAL`, in upper case; an event below it is not written |
