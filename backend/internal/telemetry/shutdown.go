package telemetry

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/logging"
	"go.opentelemetry.io/otel/sdk/trace"
)

// Shutdown flushes and shuts down the trace provider using independent bounded
// contexts. A flush failure does not prevent provider shutdown.
func Shutdown(provider *trace.TracerProvider, logger logging.Logger) {
	flushCtx, cancelFlush := context.WithTimeout(context.Background(), 5*time.Second)
	if err := provider.ForceFlush(flushCtx); err != nil {
		logger.Error(context.Background(), "flush telemetry provider", logging.ErrorFields(err)...)
	}
	cancelFlush()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := provider.Shutdown(ctx); err != nil {
		logger.Error(context.Background(), "shutdown telemetry provider", logging.ErrorFields(err)...)
	}
}
