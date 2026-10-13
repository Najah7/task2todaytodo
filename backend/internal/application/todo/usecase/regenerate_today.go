package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

// RegenerateTodayUseCase replaces every saved row for today's Todo with a new
// plan from current sources. Source completion state remains authoritative.
type RegenerateTodayUseCase struct {
	uow       UOW
	generator *GenerateTodayUseCase
	logger    telemetry.Logger
	access    shared.PlanAccessReader
}

func NewRegenerateTodayUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *RegenerateTodayUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &RegenerateTodayUseCase{
		uow: uow, generator: NewGenerateTodayUseCase(newID), logger: telemetry.OrNop(logger),
	}
}

func (u *RegenerateTodayUseCase) Execute(ctx context.Context, userID string, expectedRevision int32, ignoreAllocations ...bool) (result TodaySnapshot, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "RegenerateTodayUseCase.Execute", err) }()
	ignoreRequested := len(ignoreAllocations) > 0 && ignoreAllocations[0]
	err = u.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		plan, accessErr := todoPlanAccess(ctx, repos, userID, u.access)
		if accessErr != nil {
			return accessErr
		}
		freePlan := !plan.IsPaid()
		day, err := repos.Sources().CurrentDay(ctx, userID)
		if err != nil {
			return err
		}
		if err := repos.Todos().LockUser(ctx, userID); err != nil {
			return err
		}
		todo, err := repos.Todos().GetByUserDate(ctx, userID, day.Date)
		if err != nil {
			return err
		}
		if todo.Revision != expectedRevision {
			return ErrRevisionConflict
		}

		// Regeneration uses the user's current calendar even if the existing
		// Todo was created before their timezone changed.
		if todo.Timezone != day.Timezone {
			model, restoreErr := domain.RestoreTodo(domain.Todo{
				ID: todo.ID, UserID: todo.UserID, Date: day.Date, Timezone: todo.Timezone, Revision: todo.Revision,
			})
			if restoreErr != nil {
				return restoreErr
			}
			model, restoreErr = model.WithTimezone(day.Timezone)
			if restoreErr != nil {
				return restoreErr
			}
			todo.Revision, err = repos.Todos().UpdateTimezoneAndBumpRevision(ctx, model, expectedRevision)
		} else {
			todo.Revision, err = repos.Todos().BumpRevision(ctx, todo.ID, expectedRevision)
		}
		if err != nil {
			return err
		}
		todo.Date = day.Date
		todo.Timezone = day.Timezone

		if err := repos.Items().DeleteAllByTodoID(ctx, todo.ID); err != nil {
			return err
		}
		sources, err := repos.Sources().ReadPlanningSources(ctx, userID, day.Date, day.Timezone)
		if err != nil {
			return err
		}
		if err := u.generator.generateFromSourcesInTx(ctx, day.Date, todo, sources, repos, freePlan || ignoreRequested || len(sources.Allocations) == 0); err != nil {
			return err
		}

		saved, err := repos.Items().ListByTodoID(ctx, todo.ID)
		if err != nil {
			return err
		}
		workSources, err := repos.Sources().ReadWorkSources(ctx, userID, todo.Date, todo.Timezone, workSourceRefs(saved, sources.History))
		if err != nil {
			return err
		}
		if err := restrictSharedWriteCapabilities(ctx, repos, userID, workSources, sources.Schedules); err != nil {
			return err
		}
		result, err = buildTodaySnapshot(todo, saved, sources, workSources, time.Now())
		return err
	})
	if err == nil {
		logTodoStateChange(u.logger, ctx, "todo.regenerate", userID, result.Todo.ID, "")
	}
	return result, err
}
