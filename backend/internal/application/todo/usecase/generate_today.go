package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
)

type GenerateTodayUseCase struct {
	newID func() string
}

func NewGenerateTodayUseCase(newID func() string) *GenerateTodayUseCase {
	return &GenerateTodayUseCase{newID: newID}
}

func (u *GenerateTodayUseCase) generateInitialInTx(ctx context.Context, userID string, todoDate time.Time, timezone string, todo dao.Todo, repos Repositories) (dao.Todo, error) {
	sources, err := repos.Sources().ReadPlanningSources(ctx, userID, todoDate, timezone)
	if err != nil {
		return dao.Todo{}, err
	}
	if err := u.generateFromSourcesInTx(ctx, todoDate, todo, sources, repos); err != nil {
		return dao.Todo{}, err
	}
	return todo, nil
}

func (u *GenerateTodayUseCase) generateFromSourcesInTx(ctx context.Context, todoDate time.Time, todo dao.Todo, sources dao.PlanningSources, repos Repositories) error {
	in, err := plannerInput(todoDate, sources, nil)
	if err != nil {
		return err
	}
	plan, err := domain.Generate(in)
	if err != nil {
		return err
	}
	items, err := planTodoItems(todo.ID, nil, plan, u.newID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, err := repos.Items().Insert(ctx, item); err != nil {
			return err
		}
	}
	return nil
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
	for _, interval := range sources.ActiveTime {
		in.ActiveTime = append(in.ActiveTime, domain.Interval{Start: interval.Start, End: interval.End})
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
