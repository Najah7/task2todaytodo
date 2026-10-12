package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type AddActionItemUseCase struct {
	uow         UOW
	newID       func() string
	regenerator *RegenerateTodayUseCase
	logger      telemetry.Logger
}

func NewAddActionItemUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *AddActionItemUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &AddActionItemUseCase{uow: uow, newID: newID, regenerator: NewRegenerateTodayUseCase(uow, newID), logger: telemetry.OrNop(logger)}
}

func (u *AddActionItemUseCase) Execute(ctx context.Context, userID, actionItemID string, expectedRevision int32) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "AddActionItemUseCase.Execute", err) }()
	result, err = addWork(ctx, u.uow, u.regenerator, u.newID, userID, dao.WorkSourceRef{Kind: string(domain.FrameActionItem), ID: actionItemID}, expectedRevision)
	if err == nil && result.ID != "" {
		logTodoStateChange(u.logger, ctx, "todo.action_item.add", userID, result.ID, actionItemID)
	}
	return result, err
}

func addWork(ctx context.Context, uow UOW, regenerator *RegenerateTodayUseCase, newID func() string, userID string, ref dao.WorkSourceRef, expectedRevision int32) (result dao.Todo, err error) {
	err = uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		day, err := repos.Sources().CurrentDay(ctx, userID)
		if err != nil {
			return err
		}
		if err := repos.Todos().LockUser(ctx, userID); err != nil {
			return err
		}
		todo, err := ensureTodayTodoInTx(ctx, userID, day, repos, regenerator, newID)
		if err != nil {
			return err
		}
		if expectedRevision != todo.Revision {
			return ErrRevisionConflict
		}
		todo, saved, sources, in, err := loadPlanningState(ctx, userID, day.Date, repos)
		if err != nil {
			return err
		}
		if hasVisibleAllDaySchedule(sources, saved) {
			return ErrAllDayBlocksAction
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
			if !item.Completed {
				result = todo
				return nil
			}
		}
		candidate := candidateFromSource(source)
		found := false
		for _, existing := range in.Candidates {
			if existing.Kind == candidate.Kind && existing.ID == candidate.ID {
				found = true
				break
			}
		}
		if !found {
			in.Candidates = append(in.Candidates, candidate)
		}
		order := []domain.OrderedItem{{Kind: domain.FrameKind(ref.Kind), SourceID: ref.ID}}
		order = append(order, orderedItemsForSaved(saved)...)
		plan, err := domain.Reflow(domain.ReflowInput{Input: in, Order: order})
		if err != nil {
			return err
		}
		if err := persistPlan(ctx, todo.ID, saved, plan, newID, repos); err != nil {
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

func candidateFromSource(source dao.WorkSource) domain.Candidate {
	return domain.Candidate{Kind: domain.FrameKind(source.Kind), ID: source.ID, TaskID: source.TaskID, ProjectID: source.ProjectID,
		PriorityWeight: source.PriorityWeight, DueAt: source.DueAt, EstimatedMinutes: source.EstimatedMinutes,
		CompletedBeforeDayMinutes: source.CompletedBeforeDayMinutes, HasCompletedFrame: source.HasCompletedFrame,
		TaskStatus: source.TaskStatus, CreatedAt: source.CreatedAt}
}

func orderedItemsForSaved(saved []dao.Item) []domain.OrderedItem {
	items := make([]domain.OrderedItem, 0, len(saved))
	for _, item := range saved {
		if domain.FrameKind(item.Kind).IsWork() {
			items = append(items, domain.OrderedItem{Kind: domain.FrameKind(item.Kind), FrameID: item.ID,
				SourceID: itemSourceID(item), PlannedMinutes: intValue(item.PlannedMinutes)})
		} else {
			items = append(items, domain.OrderedItem{Kind: domain.FrameSchedule,
				ScheduleID: scheduleKey(item.ScheduleRuleID, item.OccurrenceDate)})
		}
	}
	return items
}
