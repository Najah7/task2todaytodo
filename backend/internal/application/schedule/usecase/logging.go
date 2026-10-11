package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

func logUnexpectedScheduleFailure(logger logging.Logger, ctx context.Context, operation string, err error) {
	if err == nil || logging.IsRoutineError(err) || isExpectedScheduleError(err) {
		return
	}
	fields := append([]any{"operation", operation}, logging.ErrorFields(err)...)
	logger.Error(ctx, "schedule operation failed", fields...)
}

func logScheduleStateChange(logger logging.Logger, ctx context.Context, operation string, actorID domain.UserID, scheduleID domain.ScheduleID) {
	logger.Info(ctx, "schedule operation completed", "operation", operation, "user_id", string(actorID), "schedule_id", string(scheduleID))
}

func isExpectedScheduleError(err error) bool {
	for _, expected := range []error{
		ErrScheduleProjectNotFound, ErrScheduleProjectChanged, ErrScheduleAssigneeNotEligible, ErrScheduleScopeInvalid,
		ErrOccurrenceDateRequired, ErrOccurrenceNotFound, ErrOccurrenceInactive, ErrOccurrenceCompleted,
		ErrOccurrenceRevisionMismatch, ErrOccurrenceTimeInvalid, ErrOccurrenceTitleEmpty,
		ErrOccurrenceRuleMismatch, ErrRescheduleDateMismatch, ErrInvalidSchedulePage, ErrPermissionDenied,
		domain.ErrBlueprintIDEmpty, domain.ErrBlueprintTitleEmpty, domain.ErrBlueprintTimezoneEmpty,
		domain.ErrBlueprintFrequency, domain.ErrBlueprintInterval, domain.ErrBlueprintWeekdays,
		domain.ErrBlueprintDateRange, domain.ErrBlueprintTimeRange,
		recurrence.ErrRecurrenceTimezoneInvalid, recurrence.ErrBlueprintFrequencyInvalid,
		recurrence.ErrBlueprintIntervalInvalid, recurrence.ErrBlueprintRangeInvalid,
	} {
		if errors.Is(err, expected) {
			return true
		}
	}
	return false
}
