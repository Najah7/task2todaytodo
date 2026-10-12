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
		ErrTodoRegenerationRequired,
		domain.ErrRuleIDEmpty, domain.ErrRuleTitleEmpty, domain.ErrRuleTimezoneEmpty,
		domain.ErrRuleFrequency, domain.ErrRuleInterval, domain.ErrRuleWeekdays,
		domain.ErrRuleDateRange, domain.ErrRuleTimeRange,
		domain.ErrRecurrenceTimezoneInvalid, domain.ErrRuleFrequencyInvalid,
		domain.ErrRuleIntervalInvalid, domain.ErrRuleRangeInvalid,
	} {
		if errors.Is(err, expected) {
			return true
		}
	}
	return false
}
