# Environment variables

The library **never reads the environment** ([ADR-004](../decisions/ADR-004-library-never-reads-the-environment.md)). The Go `Config` and the `Settings` of each destination carry the `env` tags with the names listed here; the consumer loads them with its own tool (in its single configuration pass) and passes the values in, so it can set them in its `.env`, its Kubernetes Secret or its pipeline. `.env.example` holds the same list with fake values only.

All the names start with `TELEMETRY_`. The sections run from the core configuration to the settings of each adapter (the shared HTTP client, the console, the file, the HTTP destination and the AWS HTTP adapter). The `LOGGER_*` variables and `APP_ENV` of the old library no longer exist: that library was removed in T-04.7 (2026-11-03).

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
| `TELEMETRY_QUEUE_POLICY` | `drop_newest` | `delivery.queue_policy` | what a full queue does: `drop_newest`, `drop_oldest` or `block_with_timeout` (REQ-DLV-007); one policy for every destination of the core |
| `TELEMETRY_BLOCK_TIMEOUT_MS` | `0` | `delivery.block_timeout_ms` | milliseconds a log call may wait for room in a full queue; required and greater than zero with `block_with_timeout`, the one case where a call can wait |
| `TELEMETRY_EVENT_CLASSES` | empty | `delivery.event_classes` | `event_type:class` pairs separated by `,`, the class being `critical`, `normal` or `bulk` (REQ-DLV-009); a class is a priority for a full queue, never a delivery guarantee |
| `TELEMETRY_RETRY_MAX_ATTEMPTS` | `5` | `delivery.retry_max_attempts` | attempts at most for one batch with the `retry` mode, the first included; `best_effort` sends once |
| `TELEMETRY_RETRY_BASE_DELAY_MS` | `200` | `delivery.retry_base_delay_ms` | base of the exponential backoff with jitter between attempts |
| `TELEMETRY_RETRY_MAX_DELAY_MS` | `30000` | `delivery.retry_max_delay_ms` | ceiling of the backoff; not below the base |
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

Each adapter has its own section below (T-04); the adapters that come later (the spool and the others) add theirs.


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

The settings of the console destination ([contracts/config/console.schema.json](../../contracts/config/console.schema.json)). The library never reads the environment: the consumer loads these values and passes them in; the defaults live in the code. **The level does not follow the environment**: the consumer chooses the minimum severity of each destination.

| Variable | Default | Field | Notes |
|---|---|---|---|
| `TELEMETRY_CONSOLE_FORMAT` | `json` | `format` | `json` (the canonical JSON of the envelope, one per line) or `text` (a readable line) |
| `TELEMETRY_CONSOLE_MINIMUM_SEVERITY` | `INFO` | `minimum_severity` | `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR` or `CRITICAL`, in upper case; an event below it is not written |


## File destination (built, T-04.3)

The settings of the file destination ([contracts/config/file.schema.json](../../contracts/config/file.schema.json)): events appended as JSON Lines with rotation by size and a quota. The library never picks a path and never reads the environment: the consumer passes the directory and the values in. The files are created **readable by the owner only** (0600; the directory 0700) and hold masked events only.

| Variable | Default | Field | Notes |
|---|---|---|---|
| `TELEMETRY_FILE_DIRECTORY` | none (required) | `directory` | created if missing; must be writable |
| `TELEMETRY_FILE_NAME` | `telemetry.jsonl` | `file_name` | no path separator |
| `TELEMETRY_FILE_MINIMUM_SEVERITY` | `INFO` | `minimum_severity` | `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR` or `CRITICAL`, in upper case |
| `TELEMETRY_FILE_MAX_FILE_BYTES` | `10485760` | `max_file_bytes` | rotate when the next line would pass it |
| `TELEMETRY_FILE_MAX_FILES` | `5` | `max_files` | the current file included |
| `TELEMETRY_FILE_QUOTA_BYTES` | `52428800` | `quota_bytes` | all the files together; not below the file size; the oldest rotated files are deleted first |


## HTTP destination (built, T-04.5)

The HTTP destination is **the shared client** (the section "Shared HTTP client" above: `TELEMETRY_HTTP_URL` and the others) plus one setting of its own ([contracts/config/http_destination.schema.json](../../contracts/config/http_destination.schema.json)). The library never reads the environment: the consumer loads the values and passes them in; the URL is always required and no URL is fixed in code.

| Variable | Default | Field | Notes |
|---|---|---|---|
| `TELEMETRY_HTTP_MINIMUM_SEVERITY` | `INFO` | `minimum_severity` | `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR` or `CRITICAL`, in upper case; an event below it is not sent |

## AWS HTTP adapter (built, T-04.6)

The adapter that sends `log.*` events to the `add_log` endpoint of aws_microservice ([contracts/config/aws_http.schema.json](../../contracts/config/aws_http.schema.json)). The endpoint is `host` + `:` + `port` + `path`; the host includes its scheme. It always POSTs and never compresses. The library never reads the environment: the consumer loads the values and passes them in. Host, port, path, log group and log stream are required and have no default; this repository holds placeholders only.

| Variable | Default | Field | Notes |
|---|---|---|---|
| `TELEMETRY_AWS_HTTP_HOST` | none (required) | `host` | with the scheme, `http` or `https`; authentication over plain http is refused except to localhost |
| `TELEMETRY_AWS_HTTP_PORT` | none (required) | `port` | 1 to 65535 |
| `TELEMETRY_AWS_HTTP_PATH` | none (required) | `path` | starts with `/` |
| `TELEMETRY_AWS_HTTP_LOG_GROUP_NAME` | none (required) | `log_group_name` | the CloudWatch log group |
| `TELEMETRY_AWS_HTTP_LOG_STREAM_NAME` | none (required) | `log_stream_name` | the CloudWatch log stream |
| `TELEMETRY_AWS_HTTP_TIMEOUT_MS` | `3000` | `timeout_ms` | milliseconds before a request gives up |
| `TELEMETRY_AWS_HTTP_SKIP_TLS_VERIFICATION` | `false` | `skip_tls_verification` | turns the certificate check off; on by default |
| `TELEMETRY_AWS_HTTP_BEARER_TOKEN` | none | `bearer_token` | a secret; not together with an API key |
| `TELEMETRY_AWS_HTTP_API_KEY_HEADER` | none | `api_key_header` | needs the value |
| `TELEMETRY_AWS_HTTP_API_KEY_VALUE` | none | `api_key_value` | a secret; needs the header |
| `TELEMETRY_AWS_HTTP_MAX_ATTEMPTS` | `5` | `max_attempts` | the first attempt included |
| `TELEMETRY_AWS_HTTP_RETRY_BASE_DELAY_MS` | `200` | `retry_base_delay_ms` | base of the backoff |
| `TELEMETRY_AWS_HTTP_RETRY_MAX_DELAY_MS` | `30000` | `retry_max_delay_ms` | ceiling of the backoff, not below the base |
