package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ReopenFrameUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewReopenFrameUseCase(uow UOW, loggers ...telemetry.Logger) *ReopenFrameUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &ReopenFrameUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (u *ReopenFrameUseCase) Execute(ctx context.Context, userID, frameID string, expectedRevision int32) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "ReopenFrameUseCase.Execute", err) }()
	err = u.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		day, err := repos.Sources().CurrentDay(ctx, userID)
		if err != nil {
			return err
		}
		if err := repos.Todos().LockUser(ctx, userID); err != nil {
			return err
		}
		todo, saved, sources, _, err := loadPlanningState(ctx, userID, day.Date, repos)
		if err != nil {
			return err
		}
		if todo.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		var target *dao.Item
		for index := range saved {
			if saved[index].ID == frameID {
				target = &saved[index]
				break
			}
		}
		if target == nil {
			return ErrFrameNotFound
		}
		if !target.Completed {
			return ErrFrameNotFound
		}
		if target.Kind == string(domain.FrameSchedule) && target.Hidden {
			return ErrPermissionDenied
		}
		model, err := restoredItem(*target)
		if err != nil {
			return err
		}
		if target.Kind == string(domain.FrameSchedule) {
			occurrence, ok := findSchedule(sources.Schedules, scheduleKey(target.ScheduleRuleID, target.OccurrenceDate))
			if !ok || !occurrence.CanReopen {
				return ErrPermissionDenied
			}
			if _, err := repos.Items().Save(ctx, model.SetCompleted(false)); err != nil {
				return err
			}
			if err := repos.Schedules().Reopen(ctx, userID, target.ScheduleRuleID, target.OccurrenceDate, occurrence.Revision); err != nil {
				return err
			}
			return bumpTodoRevision(ctx, repos, &todo, &result)
		}
		ref := itemRef(*target)
		workSources, err := repos.Sources().ReadWorkSources(ctx, userID, todo.Date, todo.Timezone, []dao.WorkSourceRef{ref})
		if err != nil {
			return err
		}
		source, ok := sourceByRef(workSources, ref)
		if !ok || !source.CanReopen {
			return ErrPermissionDenied
		}
		if _, err := repos.Items().Save(ctx, model.SetCompleted(false)); err != nil {
			return err
		}
		if target.Kind == string(domain.FrameTask) {
			err = repos.Tasks().ReopenFromTodoFrame(ctx, userID, target.TaskID)
		} else {
			err = repos.ActionItems().ReopenFromTodoFrame(ctx, userID, source.TaskID, target.ActionItemID)
		}
		if err != nil {
			return err
		}
		return bumpTodoRevision(ctx, repos, &todo, &result)
	})
	if err == nil {
		logTodoStateChange(u.logger, ctx, "todo.frame.reopen", userID, result.ID, frameID)
	}
	return result, err
}
