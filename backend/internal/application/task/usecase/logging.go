package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

func logUnexpectedTaskFailure(logger telemetry.Logger, ctx context.Context, operation string, err error) {
	if err == nil || telemetry.IsRoutineError(err) || isExpectedTaskError(err) {
		return
	}
	fields := append([]any{"operation", operation}, telemetry.ErrorFields(err)...)
	logger.Error(ctx, "task operation failed", fields...)
}

func logTaskStateChange(logger telemetry.Logger, ctx context.Context, operation string, userID domain.UserID, taskID domain.TaskID, resourceID string) {
	fields := []any{"operation", operation, "user_id", string(userID), "task_id", string(taskID)}
	if resourceID != "" {
		fields = append(fields, "resource_id", resourceID)
	}
	logger.Info(ctx, "task operation completed", fields...)
}

func logTaskRestoreFailure(logger telemetry.Logger, ctx context.Context, operation string, err error) {
	if err == nil || telemetry.IsRoutineError(err) || !isExpectedTaskError(err) {
		return
	}
	fields := append([]any{"operation", operation}, telemetry.ErrorFields(err)...)
	logger.Error(ctx, "task data restoration failed", fields...)
}

func isExpectedTaskError(err error) bool {
	for _, expected := range []error{
		ErrTaskNotFound, ErrTaskProjectNotFound, ErrActionItemTaskNotFound, ErrActionItemNotFound,
		ErrActionItemCompleted,
		ErrTaskPatchRequiredFieldNull,
		ErrInvalidTaskPage, ErrActionItemPatchRequiredFieldNull, ErrActionItemPositionOutOfRange,
		domain.ErrTaskIDEmpty,
		domain.ErrTaskUserIDEmpty, domain.ErrTaskTitleEmpty, domain.ErrTaskEstimatedMinutesInvalid,
		domain.ErrTaskActualMinutesInvalid, domain.ErrTaskProgressInvalid, domain.ErrTaskStatusEmpty, domain.ErrTaskStatusInvalid,
		domain.ErrTaskPriorityEmpty, domain.ErrTaskPriorityInvalid, domain.ErrTodoListIDEmpty,
		domain.ErrTodoListUserIDEmpty, domain.ErrTodoListListDateEmpty, domain.ErrActionItemIDEmpty,
		domain.ErrActionItemTaskIDEmpty, domain.ErrActionItemTitleEmpty, domain.ErrActionItemPositionLess,
	} {
		if errors.Is(err, expected) {
			return true
		}
	}
	return false
}
