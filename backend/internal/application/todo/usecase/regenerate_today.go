package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type RegenerateTodayUseCase struct {
	uow    UOW
	newID  func() string
	logger telemetry.Logger
}

func NewRegenerateTodayUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *RegenerateTodayUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &RegenerateTodayUseCase{uow: uow, newID: newID, logger: telemetry.OrNop(logger)}
}

func (u *RegenerateTodayUseCase) Execute(ctx context.Context, userID string, expectedRevision int32) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "RegenerateTodayUseCase.Execute", err) }()
	err = u.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
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
		if expectedRevision != todo.Revision {
			return ErrRevisionConflict
		}
		result, err = u.RegenerateTodayInTx(ctx, userID, day.Date, repos)
		return err
	})
	if err == nil {
		logTodoStateChange(u.logger, ctx, "todo.regenerate", userID, result.ID, "")
	}
	return result, err
}

// RegenerateTodayInTx runs the full planning and persistence workflow using
// only repositories already bound to the caller's transaction.
func (u *RegenerateTodayUseCase) RegenerateTodayInTx(ctx context.Context, userID string, todoDate time.Time, repos Repositories) (dao.Todo, error) {
	todoDate = calendarDate(todoDate)
	day, err := repos.Sources().CurrentDay(ctx, userID)
	if err != nil {
		return dao.Todo{}, err
	}
	todo, err := repos.Todos().GetByUserDate(ctx, userID, todoDate)
	if err == ErrNotFound {
		return dao.Todo{}, err
	}
	if err != nil {
		return dao.Todo{}, err
	}
	return u.generateExistingTodayInTx(ctx, userID, todoDate, day.Timezone, todo, repos, true, false)
}

func (u *RegenerateTodayUseCase) generateExistingTodayInTx(ctx context.Context, userID string, todoDate time.Time, timezone string, todo dao.Todo, repos Repositories, bumpRevision, preserveHidden bool) (dao.Todo, error) {

	items, err := repos.Items().ListByTodoID(ctx, todo.ID)
	if err != nil {
		return dao.Todo{}, err
	}
	sources, err := repos.Sources().ReadPlanningSources(ctx, userID, todoDate, timezone)
	if err != nil {
		return dao.Todo{}, err
	}
	sources = withSavedScheduleVisibility(sources, items, preserveHidden)
	in, err := plannerInput(todoDate, sources, items)
	if err != nil {
		return dao.Todo{}, err
	}
	plan, err := domain.Generate(in)
	if err != nil {
		return dao.Todo{}, err
	}
	if !preserveHidden {
		for index := range items {
			items[index].Hidden = false
		}
	}
	if err := persistPlan(ctx, todo.ID, items, plan, u.newID, repos); err != nil {
		return dao.Todo{}, err
	}
	if bumpRevision {
		model, err := domain.RestoreTodo(domain.Todo{
			ID: todo.ID, UserID: todo.UserID, Date: todo.Date,
			Timezone: todo.Timezone, Revision: todo.Revision,
		})
		if err != nil {
			return dao.Todo{}, err
		}
		model, err = model.WithTimezone(sources.Timezone)
		if err != nil {
			return dao.Todo{}, err
		}
		revision, err := repos.Todos().UpdateTimezoneAndBumpRevision(ctx, model, todo.Revision)
		if err != nil {
			return dao.Todo{}, err
		}
		todo.Timezone, todo.Revision = model.Timezone, revision
	}
	return todo, nil
}

func withSavedScheduleVisibility(sources dao.PlanningSources, items []dao.Item, preserveHidden bool) dao.PlanningSources {
	for index := range sources.Schedules {
		sources.Schedules[index].Hidden = false
		if !preserveHidden {
			continue
		}
		for _, item := range items {
			if item.Kind == string(domain.FrameSchedule) && scheduleKey(item.ScheduleRuleID, item.OccurrenceDate) == scheduleKey(sources.Schedules[index].RuleID, sources.Schedules[index].OccurrenceDate) {
				sources.Schedules[index].Hidden = item.Hidden
				break
			}
		}
	}
	return sources
}

func plannerInput(todoDate time.Time, sources dao.PlanningSources, saved []dao.Item) (domain.Input, error) {
	location, err := time.LoadLocation(sources.Timezone)
	if err != nil {
		return domain.Input{}, fmt.Errorf("load todo timezone: %w", err)
	}
	date := calendarDate(todoDate)
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)
	dayEnd := time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, location)
	in := domain.Input{
		Day: domain.Interval{Start: dayStart, End: dayEnd}, GapMinutes: sources.GapMinutes,
		Today: dayStart, Location: location,
	}
	for _, interval := range sources.Sleep {
		in.Sleep = append(in.Sleep, domain.Interval{Start: interval.Start, End: interval.End})
	}
	for _, allocation := range sources.Allocations {
		in.Allocations = append(in.Allocations, domain.ProjectAllocation{ProjectID: allocation.ProjectID, Interval: domain.Interval{Start: allocation.StartAt, End: allocation.EndAt}})
	}
	for _, occurrence := range sources.Schedules {
		key := scheduleKey(occurrence.RuleID, occurrence.OccurrenceDate)
		in.Schedules = append(in.Schedules, domain.Schedule{
			ID: key, RuleID: occurrence.RuleID, OccurrenceDate: occurrence.OccurrenceDate,
			Interval: domain.Interval{Start: occurrence.StartAt, End: occurrence.EndAt},
			AllDay:   occurrence.AllDay, Completed: occurrence.Completed, Hidden: occurrence.Hidden,
		})
	}
	completedIDs := make(map[string]bool)
	for _, item := range saved {
		if !domain.FrameKind(item.Kind).IsWork() || !item.Completed || completedIDs[item.ID] {
			continue
		}
		if item.PlannedMinutes == nil || *item.PlannedMinutes < 1 {
			return domain.Input{}, fmt.Errorf("completed action item %s has invalid planned minutes", item.ID)
		}
		var start, end *time.Time
		if item.ScheduledStart != nil {
			start, end = item.ScheduledStart, item.ScheduledEnd
		}
		in.CompletedFrames = append(in.CompletedFrames, domain.Frame{
			ID: item.ID, Kind: domain.FrameKind(item.Kind), SourceID: itemSourceID(item),
			PlannedMinutes: *item.PlannedMinutes, Start: start, End: end, Completed: true, Position: int(item.Position),
		})
		completedIDs[item.ID] = true
	}
	for _, source := range sources.Candidates {
		if source.Completed {
			continue
		}
		in.Candidates = append(in.Candidates, domain.Candidate{
			Kind: domain.FrameKind(source.Kind), ID: source.ID, TaskID: source.TaskID, ProjectID: source.ProjectID,
			PriorityWeight: source.PriorityWeight, DueAt: source.DueAt,
			EstimatedMinutes: source.EstimatedMinutes, CompletedBeforeDayMinutes: source.CompletedBeforeDayMinutes,
			HasCompletedFrame: source.HasCompletedFrame, TaskStatus: source.TaskStatus, CreatedAt: source.CreatedAt,
		})
	}
	return in, nil
}

func itemFromFrame(todoID string, frame domain.Frame, generatedID string) (domain.TodoItem, error) {
	id := frame.ID
	if id == "" {
		id = frame.OriginalFrameID
	}
	if id == "" {
		id = generatedID
	}
	if frame.Kind == domain.FrameActionItem {
		return domain.NewActionTodoItem(id, todoID, frame.SourceID, frame.PlannedMinutes, frame.Start, frame.End, frame.Completed, frame.Position)
	}
	if frame.Kind == domain.FrameTask {
		return domain.NewTaskTodoItem(id, todoID, frame.SourceID, frame.PlannedMinutes, frame.Start, frame.End, frame.Completed, frame.Position)
	}
	if frame.Start == nil || frame.End == nil {
		return domain.TodoItem{}, domain.ErrInvalidTodoItem
	}
	return domain.NewScheduleTodoItem(id, todoID, frame.RuleID, frame.OccurrenceDate, *frame.Start, *frame.End, frame.Completed, frame.Hidden, frame.Position)
}

func scheduleKey(ruleID string, occurrenceDate time.Time) string {
	return ruleID + "/" + occurrenceDate.Format("2006-01-02")
}

func calendarDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
