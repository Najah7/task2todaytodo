package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

func logUnexpectedScheduleFailure(logger telemetry.Logger, ctx context.Context, operation string, err error) {
	if err == nil || telemetry.IsRoutineError(err) || isExpectedScheduleError(err) {
		return
	}
	fields := append([]any{"operation", operation}, telemetry.ErrorFields(err)...)
	logger.Error(ctx, "schedule operation failed", fields...)
}

func logScheduleStateChange(logger telemetry.Logger, ctx context.Context, operation string, actorID domain.UserID, scheduleID domain.ScheduleID) {
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
		domain.ErrRecurrenceTimezoneInvalid, domain.ErrBlueprintFrequencyInvalid,
		domain.ErrBlueprintIntervalInvalid, domain.ErrBlueprintRangeInvalid,
	} {
		if errors.Is(err, expected) {
			return true
		}
	}
	return false
}
