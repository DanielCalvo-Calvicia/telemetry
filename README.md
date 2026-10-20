# telemetry

Go logging library with sensitive-data masking, a console sink and an HTTP sink.
Public API: `github.com/DanielCalvo-Calvicia/telemetry/pkg/logging`.

```go
logger, shutdown, err := logging.New(ctx, "development", logging.Config{ConsoleEnabled: true})
if err != nil { /* handle */ }
defer shutdown(ctx)

_ = logger.Info(ctx, logging.Entry{Application: "my-app", Function: "main", Message: "hello"})
```

The library never reads environment variables: embed `logging.Config` (env tags included) in your own
config and load it yourself. See `.env.example`. `app_env` is `development` (DEBUG and above) or
`production` (INFO and above).

Status: v0.x, the API may change. Licensed under MIT.

## Sinks

- Console: one JSON line per entry on the standard output (`LOGGER_CONSOLE_ENABLED`).
- HTTP: posts the flat JSON entry to a generic receiver (`LOGGER_HTTP_*`).
- AWS HTTP: adapts the entry to the `add_log` endpoint of aws_microservice, so it lands in a CloudWatch Logs
  stream (`LOGGER_AWS_HTTP_*`, with `LOGGER_AWS_HTTP_LOG_GROUP` and `LOGGER_AWS_HTTP_LOG_STREAM` required).

`examples/aws_http_smoke` sends one entry through the AWS HTTP sink to check a deployment.

## Building the sinks yourself

Instead of switching sinks on through `Config`, a consumer can build the ones it needs and pass them to the logger:

```go
aws_sink, err := logging.NewAWSHTTPSink("http://aws-microservice", "8080", "/api/v1/cloudwatch/logs/add", "my-group", "my-stream")
if err != nil { /* handle */ }

logger, shutdown, err := logging.NewWithSinks(ctx, "production", nil, logging.NewConsoleSink(), aws_sink)
```

`NewConsoleSink`, `NewHTTPSink` and `NewAWSHTTPSink` are the ready-made sinks; a consumer can also implement `logging.Sink`
for a destination of its own.
