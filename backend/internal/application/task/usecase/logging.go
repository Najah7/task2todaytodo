package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

func logUnexpectedTaskFailure(logger logging.Logger, ctx context.Context, operation string, err error) {
	if err == nil || logging.IsRoutineError(err) || isExpectedTaskError(err) {
		return
	}
	fields := append([]any{"operation", operation}, logging.ErrorFields(err)...)
	logger.Error(ctx, "task operation failed", fields...)
}

func logTaskStateChange(logger logging.Logger, ctx context.Context, operation string, userID domain.UserID, taskID domain.TaskID, resourceID string) {
	fields := []any{"operation", operation, "user_id", string(userID), "task_id", string(taskID)}
	if resourceID != "" {
		fields = append(fields, "resource_id", resourceID)
	}
	logger.Info(ctx, "task operation completed", fields...)
}

func logTaskRestoreFailure(logger logging.Logger, ctx context.Context, operation string, err error) {
	if err == nil || logging.IsRoutineError(err) || !isExpectedTaskError(err) {
		return
	}
	fields := append([]any{"operation", operation}, logging.ErrorFields(err)...)
	logger.Error(ctx, "task data restoration failed", fields...)
}

func isExpectedTaskError(err error) bool {
	for _, expected := range []error{
		ErrTaskNotFound, ErrTaskProjectNotFound, ErrTodoItemTaskNotFound, ErrTodoItemNotFound,
		ErrTodoItemPositionConflict, ErrOccurrenceDateRequired, ErrOccurrenceNotFound,
		ErrOccurrenceInactive, ErrOccurrenceCompleted, ErrOccurrenceRuleMismatch,
		ErrTaskTagNotFound, ErrTaskTagNameConflict, ErrTaskTagAssignmentNotOwned,
		ErrTaskPatchRequiredFieldNull, ErrProjectPatchRequiredFieldNull,
		ErrInvalidTaskPage, ErrTodoItemPatchRequiredFieldNull, ErrTodoItemScopeInvalid,
		ErrTodoItemFutureDueDateUnsupported, ErrTodoItemPositionOutOfRange,
		ErrRescheduleDateMismatch, ErrTaskSchedulePatchRequiredFieldNull,
		ErrTaskScheduleScopeInvalid,
		domain.ErrProjectEndDateBeforeStartDate, domain.ErrProjectTypeEmpty, domain.ErrProjectTypeInvalid,
		domain.ErrTaskFrequencyEmpty, domain.ErrTaskFrequencyInvalid, domain.ErrTaskIDEmpty,
		domain.ErrTaskUserIDEmpty, domain.ErrTaskTitleEmpty, domain.ErrTaskEstimatedMinutesInvalid,
		domain.ErrTaskActualMinutesInvalid, domain.ErrTaskProgressInvalid, domain.ErrRecurrenceIntervalWeeksLess,
		domain.ErrRecurrenceTimezoneInvalid, domain.ErrRecurrenceMetadataInvalid, domain.ErrRecurrenceLimitInvalid,
		domain.ErrRecurrenceHorizonInvalid, domain.ErrTaskStatusEmpty, domain.ErrTaskStatusInvalid,
		domain.ErrTaskPriorityEmpty, domain.ErrTaskPriorityInvalid, domain.ErrTaskScheduleIDEmpty,
		domain.ErrTaskScheduleTaskIDEmpty, domain.ErrTaskScheduleTitleEmpty, domain.ErrTaskScheduleIntervalWeeksLess,
		domain.ErrTaskScheduleStartAtEmpty, domain.ErrTaskScheduleEndAtEmpty, domain.ErrTaskScheduleEndAtMustBeAfterStartAt,
		domain.ErrTaskScheduleTaskNotFound, domain.ErrTaskScheduleNotFound, domain.ErrTodoListIDEmpty,
		domain.ErrTodoListUserIDEmpty, domain.ErrTodoListListDateEmpty, domain.ErrTodoItemIDEmpty,
		domain.ErrTodoItemTaskIDEmpty, domain.ErrTodoItemTitleEmpty, domain.ErrTodoItemPositionLess,
		domain.ErrTodoItemIntervalWeeksLess, domain.ErrProjectIDEmpty, domain.ErrProjectUserIDEmpty,
		domain.ErrProjectTitleEmpty, domain.ErrProjectProgressInvalid, domain.ErrProjectNotFound,
		domain.ErrTaskTagIDEmpty, domain.ErrTaskTagUserIDEmpty, domain.ErrTaskTagNameEmpty,
	} {
		if errors.Is(err, expected) {
			return true
		}
	}
	return false
}
