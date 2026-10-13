package usecase

import (
	"context"
	"math"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type CompleteFrameUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewCompleteFrameUseCase(uow UOW, loggers ...telemetry.Logger) *CompleteFrameUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &CompleteFrameUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (u *CompleteFrameUseCase) Execute(ctx context.Context, userID, frameID string, expectedRevision int32, actualMinutes *int) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "CompleteFrameUseCase.Execute", err) }()
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
		if target.Completed {
			return ErrFrameCompleted
		}
		if actualMinutes != nil && (*actualMinutes < 0 || int64(*actualMinutes) > math.MaxInt32) {
			return domain.ErrInvalidInput
		}
		model, err := restoredItem(*target)
		if err != nil {
			return err
		}
		updated := model.SetCompleted(true)
		if target.Kind == string(domain.FrameSchedule) {
			if actualMinutes != nil {
				return domain.ErrInvalidInput
			}
			if target.Hidden {
				return ErrPermissionDenied
			}
			occurrence, ok := findSchedule(sources.Schedules, scheduleKey(target.ScheduleRuleID, target.OccurrenceDate))
			if !ok || !occurrence.CanComplete {
				return ErrPermissionDenied
			}
			if err := repos.Schedules().Complete(ctx, userID, target.ScheduleRuleID, target.OccurrenceDate, occurrence.Revision); err != nil {
				return err
			}
			if _, err := repos.Items().Save(ctx, updated); err != nil {
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
		if !ok || !source.CanComplete {
			return ErrPermissionDenied
		}
		if actualMinutes != nil && *actualMinutes < 0 {
			return domain.ErrInvalidInput
		}
		if _, err := repos.Items().Save(ctx, updated); err != nil {
			return err
		}
		if target.Kind == string(domain.FrameTask) {
			err = repos.Tasks().CompleteFromTodoFrame(ctx, userID, target.TaskID, actualMinutes)
		} else {
			err = repos.ActionItems().CompleteFromTodoFrame(ctx, userID, source.TaskID, target.ActionItemID, actualMinutes)
		}
		if err != nil {
			return err
		}
		return bumpTodoRevision(ctx, repos, &todo, &result)
	})
	if err == nil {
		logTodoStateChange(u.logger, ctx, "todo.frame.complete", userID, result.ID, frameID)
	}
	return result, err
}

func bumpTodoRevision(ctx context.Context, repos Repositories, todo *dao.Todo, result *dao.Todo) error {
	if _, err := repos.Todos().BumpRevision(ctx, todo.ID, todo.Revision); err != nil {
		return err
	}
	todo.Revision++
	*result = *todo
	return nil
}
