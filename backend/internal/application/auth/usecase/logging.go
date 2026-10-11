package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

func logUnexpectedFailure(logger telemetry.Logger, ctx context.Context, operation string, err error) {
	if err == nil || telemetry.IsRoutineError(err) {
		return
	}
	fields := append([]any{"operation", operation}, telemetry.ErrorFields(err)...)
	logger.Error(ctx, "auth operation failed", fields...)
}

func logAuthenticationRejected(logger telemetry.Logger, ctx context.Context, reason string) {
	logger.Warn(ctx, "authentication rejected", "reason", reason)
}
