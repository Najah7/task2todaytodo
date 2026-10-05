package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/logging"
)

func logUnexpectedFailure(logger logging.Logger, ctx context.Context, operation string, err error) {
	if err == nil || logging.IsRoutineError(err) {
		return
	}
	fields := append([]any{"operation", operation}, logging.ErrorFields(err)...)
	logger.Error(ctx, "auth operation failed", fields...)
}

func logAuthenticationRejected(logger logging.Logger, ctx context.Context, reason string) {
	logger.Warn(ctx, "authentication rejected", "reason", reason)
}
