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
