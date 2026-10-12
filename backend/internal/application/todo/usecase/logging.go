package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

func logUnexpectedTodoFailure(logger telemetry.Logger, ctx context.Context, operation string, err error) {
	if err == nil || telemetry.IsRoutineError(err) || isExpectedTodoError(err) {
		return
	}
	fields := append([]any{"operation", operation}, telemetry.ErrorFields(err)...)
	logger.Error(ctx, "todo operation failed", fields...)
}

func logTodoStateChange(logger telemetry.Logger, ctx context.Context, operation, userID string, todoID string, itemID string) {
	fields := []any{"operation", operation, "user_id", userID, "todo_id", todoID}
	if itemID != "" {
		fields = append(fields, "todo_item_id", itemID)
	}
	logger.Info(ctx, "todo operation completed", fields...)
}

func isExpectedTodoError(err error) bool {
	for _, expected := range []error{
		ErrNotFound, ErrRevisionConflict, ErrPermissionDenied, ErrActionItemNotFound, ErrTaskNotFound,
		ErrFrameNotFound, ErrFrameCompleted, ErrActionItemStarted, ErrTaskStarted, ErrScheduleCompleted, ErrAllDayBlocksAction,
		domain.ErrInvalidInput, domain.ErrInvalidInterval, domain.ErrInvalidGap,
		domain.ErrInvalidCandidate, domain.ErrInvalidReflow, domain.ErrDuplicateID,
		domain.ErrInvalidTodo, domain.ErrInvalidTodoItem,
	} {
		if errors.Is(err, expected) {
			return true
		}
	}
	return false
}
