package logging_test

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/pkg/logging"
)

func ExampleNew() {
	ctx := context.Background()
	logger, shutdown, err := logging.New(ctx, "development", logging.Config{ConsoleEnabled: true})
	if err != nil {
		panic(err)
	}
	defer shutdown(ctx)

	_ = logger.Info(ctx, logging.Entry{Application: "example", Function: "ExampleNew", Message: "hello"})
}
