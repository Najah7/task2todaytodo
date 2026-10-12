package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type TodaySnapshot struct {
	Todo            dao.Todo
	Items           []TodayItem
	HiddenSchedules []TodayItem
	Progress        Progress
	NextSchedule    *TodayItem
	Policy          domain.Policy
}

type TodayItem struct {
	ID               string
	Kind             string
	ActionItemID     string
	ScheduleRuleID   string
	OccurrenceDate   time.Time
	TaskID           string
	TaskTitle        string
	ProjectID        string
	ProjectTitle     string
	Title            string
	Description      string
	PlannedMinutes   *int
	DurationMinutes  int
	ScheduledStart   *time.Time
	ScheduledEnd     *time.Time
	Completed        bool
	Hidden           bool
	History          bool
	Started          bool
	Position         int32
	ActualMinutes    *int
	CanComplete      bool
	WorkComplete     bool
	SourceCompleted  bool
	CanReopen        bool
	CanMove          bool
	CanRemove        bool
	CanHide          bool
	CanRestore       bool
	CompletesSource  bool
	CanAddActionItem bool
	EstimateEditable bool
}

type Progress struct {
	CompletedItems   int
	TotalItems       int
	CompletedMinutes int
	TotalMinutes     int
	RemainingMinutes int
}

type GetTodayUseCase struct {
	uow         UOW
	regenerator *RegenerateTodayUseCase
	newID       func() string
	logger      telemetry.Logger
}

func NewGetTodayUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *GetTodayUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	logger = telemetry.OrNop(logger)
	return &GetTodayUseCase{uow: uow, newID: newID, logger: logger, regenerator: NewRegenerateTodayUseCase(uow, newID, logger)}
}

func (u *GetTodayUseCase) Execute(ctx context.Context, userID string) (result TodaySnapshot, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "GetTodayUseCase.Execute", err) }()
	created := false
	err = u.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		day, err := repos.Sources().CurrentDay(ctx, userID)
		if err != nil {
			return err
		}
		if err := repos.Todos().LockUser(ctx, userID); err != nil {
			return err
		}
		todo, err := repos.Todos().GetByUserDate(ctx, userID, day.Date)
		if err == ErrNotFound {
			model, modelErr := domain.NewTodo(u.newID(), userID, day.Date, day.Timezone)
			if modelErr != nil {
				return modelErr
			}
			todo, err = repos.Todos().Create(ctx, model)
			created = err == nil
		}
		if err != nil {
			return err
		}
		if created {
			todo, err = u.regenerator.generateExistingTodayInTx(ctx, userID, day.Date, day.Timezone, todo, repos, false, false)
			if err != nil {
				return err
			}
		}
		saved, err := repos.Items().ListByTodoID(ctx, todo.ID)
		if err != nil {
			return err
		}
		sources, err := repos.Sources().ReadPlanningSources(ctx, userID, todo.Date, todo.Timezone)
		if err != nil {
			return err
		}
		workSources, err := repos.Sources().ReadWorkSources(ctx, userID, todo.Date, todo.Timezone, workSourceRefs(saved, sources.History))
		if err != nil {
			return err
		}
		result, err = buildTodaySnapshot(todo, saved, sources, workSources, time.Now())
		return err
	})
	if err == nil && created {
		logTodoStateChange(u.logger, ctx, "todo.generate.initial", userID, result.Todo.ID, "")
	}
	return result, err
}

func buildTodaySnapshot(todo dao.Todo, saved []dao.Item, sources dao.PlanningSources, workSources []dao.WorkSource, now time.Time) (TodaySnapshot, error) {
	result := TodaySnapshot{Todo: todo, Policy: domain.ScoringPolicy()}
	location, err := time.LoadLocation(todo.Timezone)
	if err != nil {
		return TodaySnapshot{}, domain.ErrInvalidInput
	}
	todoDate := calendarDate(todo.Date)
	dayStart := time.Date(todoDate.Year(), todoDate.Month(), todoDate.Day(), 0, 0, 0, 0, location)
	dayEnd := time.Date(todoDate.Year(), todoDate.Month(), todoDate.Day()+1, 0, 0, 0, 0, location)
	result.Policy.PriorityWeights = sources.PriorityWeights
	result.Policy.GapMinutes = sources.GapMinutes
	sourceByID := make(map[string]dao.WorkSource)
	// Saved pending/completed rows may no longer be auto candidates; read them
	// through the separate assignment- and ACL-checked source projection.
	for _, source := range workSources {
		sourceByID[sourceKey(source.Kind, source.ID)] = source
	}
	hiddenByKey := make(map[string]bool)
	for _, item := range saved {
		if item.Kind == string(domain.FrameSchedule) {
			key := scheduleKey(item.ScheduleRuleID, item.OccurrenceDate)
			hiddenByKey[key] = item.Hidden
		}
	}
	allDay := false
	for _, schedule := range sources.Schedules {
		key := scheduleKey(schedule.RuleID, schedule.OccurrenceDate)
		if schedule.AllDay && !hiddenByKey[key] {
			allDay = true
			break
		}
	}
	var completedActionMinutes = make(map[string]int)
	for _, item := range saved {
		if domain.FrameKind(item.Kind).IsWork() && item.PlannedMinutes != nil {
			if item.Completed {
				completedActionMinutes[sourceKey(item.Kind, itemSourceID(item))] += *item.PlannedMinutes
			}
		}
	}
	for _, item := range saved {
		if domain.FrameKind(item.Kind).IsWork() {
			if allDay {
				continue
			}
			key := sourceKey(item.Kind, itemSourceID(item))
			source, ok := sourceByID[key]
			if !ok {
				continue
			}
			row := workTodayItem(item, source, completedActionMinutes[key])
			result.Items = append(result.Items, row)
			continue
		}
		key := scheduleKey(item.ScheduleRuleID, item.OccurrenceDate)
		occurrence, ok := findSchedule(sources.Schedules, key)
		if !ok {
			continue
		}
		row := scheduleTodayItem(item, occurrence)
		if row.ScheduledStart != nil && row.ScheduledEnd != nil {
			row.DurationMinutes = clippedMinutes(*row.ScheduledStart, *row.ScheduledEnd, dayStart, dayEnd)
		}
		if item.Hidden {
			result.HiddenSchedules = append(result.HiddenSchedules, row)
		} else {
			result.Items = append(result.Items, row)
		}
	}
	if !allDay {
		savedActionItems := make(map[string]bool)
		for _, item := range saved {
			if domain.FrameKind(item.Kind).IsWork() {
				savedActionItems[sourceKey(item.Kind, itemSourceID(item))] = true
			}
		}
		for _, frame := range sources.History {
			ref := historyRef(frame)
			key := sourceKey(ref.Kind, ref.ID)
			if !savedActionItems[key] {
				continue
			}
			source, ok := sourceByID[key]
			if !ok {
				continue
			}
			minutes := frame.PlannedMinutes
			result.Items = append(result.Items, TodayItem{
				ID: frame.ID, Kind: frame.Kind, ActionItemID: frame.ActionItemID,
				TaskID: source.TaskID, TaskTitle: source.TaskTitle, ProjectID: source.ProjectID, ProjectTitle: source.ProjectTitle,
				Title: source.Title, Description: source.Description, PlannedMinutes: &minutes, Completed: true,
				History: true, DurationMinutes: minutes, ActualMinutes: source.ActualMinutes, WorkComplete: source.Completed, SourceCompleted: source.Completed,
			})
		}
	}
	sort.SliceStable(result.Items, func(i, j int) bool {
		if result.Items[i].ScheduledStart == nil {
			return false
		}
		if result.Items[j].ScheduledStart == nil {
			return true
		}
		return result.Items[i].ScheduledStart.Before(*result.Items[j].ScheduledStart)
	})
	result.Progress = calculateProgress(result.Items)
	for index := range result.Items {
		item := result.Items[index]
		if item.Kind == string(domain.FrameSchedule) && !item.Completed && item.ScheduledStart != nil && !item.ScheduledStart.Before(now) {
			if result.NextSchedule == nil || item.ScheduledStart.Before(*result.NextSchedule.ScheduledStart) {
				copy := item
				result.NextSchedule = &copy
			}
		}
	}
	return result, nil
}

func workTodayItem(item dao.Item, source dao.WorkSource, completed int) TodayItem {
	wholeComplete := source.Completed || source.CompletedBeforeDayMinutes+completed >= source.EstimatedMinutes
	completesSource := !item.Completed && !source.Completed && source.CompletedBeforeDayMinutes+completed+plannedMinutes(item.PlannedMinutes) >= source.EstimatedMinutes
	return TodayItem{
		ID: item.ID, Kind: item.Kind, ActionItemID: item.ActionItemID, TaskID: source.TaskID,
		TaskTitle: source.TaskTitle, ProjectID: source.ProjectID, ProjectTitle: source.ProjectTitle,
		Title: source.Title, Description: source.Description, PlannedMinutes: item.PlannedMinutes,
		ScheduledStart: item.ScheduledStart, ScheduledEnd: item.ScheduledEnd, Completed: item.Completed,
		DurationMinutes: plannedMinutes(item.PlannedMinutes), Position: item.Position, ActualMinutes: source.ActualMinutes,
		WorkComplete: wholeComplete, CompletesSource: completesSource, Started: source.HasCompletedFrame,
		CanComplete: !item.Completed && source.CanComplete, SourceCompleted: source.Completed, CanReopen: item.Completed && source.CanReopen,
		CanMove: !item.Completed, CanRemove: !item.Completed && !source.HasCompletedFrame && source.CanRemoveFromToday,
		CanAddActionItem: source.CanAddActionItem, EstimateEditable: source.EstimateEditable,
	}
}

func scheduleTodayItem(item dao.Item, source dao.ScheduleOccurrence) TodayItem {
	return TodayItem{ID: item.ID, Kind: item.Kind, ScheduleRuleID: item.ScheduleRuleID, OccurrenceDate: item.OccurrenceDate,
		ProjectID: source.ProjectID, Title: source.Title, Description: source.Description,
		ScheduledStart: item.ScheduledStart, ScheduledEnd: item.ScheduledEnd, Completed: source.Completed,
		Hidden: item.Hidden, Position: item.Position, CanComplete: !source.Completed && !item.Hidden && source.CanComplete,
		CanReopen: source.Completed && !item.Hidden && source.CanReopen, CanMove: false, CanRemove: false,
		CanHide: !source.Completed && !item.Hidden && source.CanHide, CanRestore: item.Hidden && source.CanRestore}
}

func plannedMinutes(minutes *int) int {
	if minutes == nil {
		return 0
	}
	return *minutes
}

func clippedMinutes(start, end, dayStart, dayEnd time.Time) int {
	if start.Before(dayStart) {
		start = dayStart
	}
	if end.After(dayEnd) {
		end = dayEnd
	}
	if !end.After(start) {
		return 0
	}
	return int(end.Sub(start).Minutes())
}

func calculateProgress(items []TodayItem) Progress {
	progress := Progress{}
	completedSources := make(map[string]bool)
	totalActionSources := make(map[string]bool)
	for _, item := range items {
		if item.History || item.Hidden {
			continue
		}
		if domain.FrameKind(item.Kind).IsWork() && item.PlannedMinutes != nil {
			totalActionSources[sourceKey(item.Kind, itemSourceIDFromToday(item))] = true
			progress.TotalMinutes += item.DurationMinutes
			if item.Completed {
				progress.CompletedMinutes += item.DurationMinutes
			}
		} else if item.Kind == string(domain.FrameSchedule) && item.ScheduledStart != nil && item.ScheduledEnd != nil {
			progress.TotalItems++
			progress.TotalMinutes += item.DurationMinutes
			if item.Completed {
				progress.CompletedMinutes += item.DurationMinutes
			}
			if item.Completed {
				progress.CompletedItems++
			}
		}
	}
	progress.TotalItems += len(totalActionSources)
	for _, item := range items {
		if !item.History && domain.FrameKind(item.Kind).IsWork() && item.WorkComplete && !completedSources[sourceKey(item.Kind, itemSourceIDFromToday(item))] {
			completedSources[sourceKey(item.Kind, itemSourceIDFromToday(item))] = true
			progress.CompletedItems++
		}
	}
	progress.RemainingMinutes = progress.TotalMinutes - progress.CompletedMinutes
	return progress
}

func itemSourceIDFromToday(item TodayItem) string {
	if item.Kind == string(domain.FrameTask) {
		return item.TaskID
	}
	return item.ActionItemID
}

func historyRef(frame dao.HistoryFrame) dao.WorkSourceRef {
	return dao.WorkSourceRef{Kind: frame.Kind, ID: itemHistorySourceID(frame)}
}

func findSchedule(schedules []dao.ScheduleOccurrence, key string) (dao.ScheduleOccurrence, bool) {
	for _, schedule := range schedules {
		if scheduleKey(schedule.RuleID, schedule.OccurrenceDate) == key {
			return schedule, true
		}
	}
	return dao.ScheduleOccurrence{}, false
}
