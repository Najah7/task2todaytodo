package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type GetOccurrenceUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewGetOccurrenceUseCase(uow UOW, loggers ...telemetry.Logger) *GetOccurrenceUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &GetOccurrenceUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *GetOccurrenceUseCase) Execute(ctx context.Context, actorID, ruleID, occurrenceDate string) (output dao.ScheduleOccurrence, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "GetOccurrenceUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		value, err := occurrenceForDate(ctx, repos.Rules(), actorID, ruleID, occurrenceDate)
		if err != nil {
			return err
		}
		capabilities, err := repos.Rules().ListOccurrenceCapabilities(ctx, actorID, []dao.ScheduleOccurrence{value})
		if err != nil {
			return err
		}
		permissions := capabilities[value.RuleID+"/"+value.OccurrenceDate]
		value.CanUpdate, value.CanDelete = permissions.CanUpdate, permissions.CanDelete
		value.CanSkip, value.CanAssign = permissions.CanSkip, permissions.CanAssign
		revision, err := repos.Rules().GetEffectiveRevisionForDate(ctx, actorID, ruleID, occurrenceDate, shared.ScheduleRead())
		if err != nil {
			if !errors.Is(err, ErrOccurrenceNotFound) {
				return err
			}
		} else if followingAllowed(activeOccurrenceStart([]dao.RuleRevision{revision}, value), time.Now().UTC()) {
			value.CanUpdateFollowing, value.CanDeleteFollowing = permissions.CanUpdateFollowing, permissions.CanDeleteFollowing
		}
		if err := restrictScheduleWriteCapabilities(ctx, repos, actorID, &value); err != nil {
			return err
		}
		output = value
		return nil
	})
	return output, err
}
