package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type AddActionItemUseCase struct {
	uow              UOW
	newID            func() string
	initialGenerator *GenerateTodayUseCase
	logger           telemetry.Logger
	access           shared.PlanAccessReader
}

func NewAddActionItemUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *AddActionItemUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &AddActionItemUseCase{uow: uow, newID: newID, initialGenerator: NewGenerateTodayUseCase(newID), logger: telemetry.OrNop(logger)}
}

func (u *AddActionItemUseCase) Execute(ctx context.Context, userID, actionItemID string, expectedRevision int32) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "AddActionItemUseCase.Execute", err) }()
	result, err = addWork(ctx, u.uow, u.initialGenerator, u.newID, u.access, userID, dao.WorkSourceRef{Kind: string(domain.FrameActionItem), ID: actionItemID}, expectedRevision)
	if err == nil && result.ID != "" {
		logTodoStateChange(u.logger, ctx, "todo.action_item.add", userID, result.ID, actionItemID)
	}
	return result, err
}

func addWork(ctx context.Context, uow UOW, initialGenerator *GenerateTodayUseCase, newID func() string, access shared.PlanAccessReader, userID string, ref dao.WorkSourceRef, expectedRevision int32) (result dao.Todo, err error) {
	err = uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		plan, accessErr := todoPlanAccess(ctx, repos, userID, access)
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
		todo, err := ensureTodayTodoInTx(ctx, userID, day, repos, initialGenerator, newID, freePlan)
		if err != nil {
			return err
		}
		if expectedRevision != todo.Revision {
			return ErrRevisionConflict
		}
		todo, saved, _, _, err := loadPlanningState(ctx, userID, day.Date, repos)
		if err != nil {
			return err
		}
		workSources, err := repos.Sources().ReadWorkSources(ctx, userID, todo.Date, todo.Timezone, []dao.WorkSourceRef{ref})
		if err != nil {
			return err
		}
		source, ok := sourceByRef(workSources, ref)
		if !ok {
			if ref.Kind == string(domain.FrameTask) {
				return ErrTaskNotFound
			}
			return ErrActionItemNotFound
		}
		if err := requireSharedProjectWrite(ctx, repos, userID, source.ProjectID); err != nil {
			return err
		}
		if source.Completed {
			if ref.Kind == string(domain.FrameTask) {
				return ErrTaskStarted
			}
			return ErrActionItemStarted
		}
		if !source.CanAddToToday {
			return ErrPermissionDenied
		}
		for _, item := range saved {
			if item.Kind != ref.Kind || itemSourceID(item) != ref.ID {
				continue
			}
			result = todo
			return nil
		}
		position := 0
		for _, item := range saved {
			if int(item.Position) >= position {
				position = int(item.Position) + 1
			}
		}
		var item domain.TodoItem
		if ref.Kind == string(domain.FrameTask) {
			item, err = domain.NewTaskTodoItem(newID(), todo.ID, source.ID, source.EstimatedMinutes, nil, nil, false, position)
		} else {
			item, err = domain.NewActionTodoItem(newID(), todo.ID, source.ID, source.EstimatedMinutes, nil, nil, false, position)
		}
		if err != nil {
			return err
		}
		if _, err := repos.Items().Insert(ctx, item); err != nil {
			return err
		}
		if _, err := repos.Todos().BumpRevision(ctx, todo.ID, todo.Revision); err != nil {
			return err
		}
		todo.Revision++
		result = todo
		return nil
	})
	return result, err
}
