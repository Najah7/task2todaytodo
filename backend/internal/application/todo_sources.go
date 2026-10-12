package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	allocationdao "github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
	authdomain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	authrepository "github.com/Najah7/task2todaytodo/internal/application/auth/repository"
	schedulerepository "github.com/Najah7/task2todaytodo/internal/application/schedule/repository"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskrepository "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	tododao "github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	todousecase "github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// NewTodoSourceReader binds owner-context projections to the caller's Todo
// transaction. The Todo usecase sees only its primitive SourceReader port.
func NewTodoSourceReader(store Store, tx pgx.Tx) todousecase.SourceReader {
	return &todoSourceReader{
		users: store.Auth.WithTx(tx).Users, schedules: store.Schedule.WithTx(tx).Schedules,
		allocation: store.Allocation.WithTx(tx), priorities: store.Task.WithTx(tx).TaskPriorities,
		queries: sqlc.New(tx),
	}
}

// NewTodoSourceReaderFromTx builds the read adapter for owner workflows that
// already hold the shared transaction and do not have the pool-backed Store.
func NewTodoSourceReaderFromTx(tx pgx.Tx) todousecase.SourceReader {
	return &todoSourceReader{
		users: authrepository.NewUserRepository(tx), schedules: schedulerepository.NewScheduleRepository(tx),
		allocation: allocationrepository.NewAllocationRepositoryFromTx(tx), priorities: taskrepository.NewTaskPriorityRepository(tx),
		queries: sqlc.New(tx),
	}
}

func todoSourceBool(value pgtype.Bool) bool {
	return value.Valid && value.Bool
}

type todoSourceReader struct {
	users      *authrepository.UserRepository
	schedules  *schedulerepository.ScheduleRepository
	allocation allocationusecase.Repository
	priorities *taskrepository.TaskPriorityRepository
	queries    *sqlc.Queries
}

// todoAllocationCalendar keeps the allocation owner's projection logic while
// resolving profile-based week and sleep boundaries in the Todo calendar.
type todoAllocationCalendar struct {
	allocationusecase.Repository
	timezone string
}

func (r todoAllocationCalendar) GetProfile(ctx context.Context, userID string) (allocationdao.Profile, error) {
	profile, err := r.Repository.GetProfile(ctx, userID)
	if err != nil {
		return allocationdao.Profile{}, err
	}
	profile.Timezone = r.timezone
	return profile, nil
}

func (r *todoSourceReader) CurrentDay(ctx context.Context, userID string) (tododao.CurrentDay, error) {
	row, err := r.queries.GetTodoCurrentDay(ctx, userID)
	if err != nil {
		return tododao.CurrentDay{}, err
	}
	if _, err := time.LoadLocation(row.Timezone); err != nil {
		return tododao.CurrentDay{}, err
	}
	return tododao.CurrentDay{
		Date:     sourceDateOnly(row.LocalDate.Time),
		Timezone: row.Timezone, Gap: int(row.MinimumGapMinutes),
	}, nil
}

func (r *todoSourceReader) ReadPlanningSources(ctx context.Context, userID string, todoDate time.Time, timezone string) (tododao.PlanningSources, error) {
	user, err := r.users.Get(ctx, authdomain.UserID(userID))
	if err != nil {
		return tododao.PlanningSources{}, err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return tododao.PlanningSources{}, err
	}
	date := sourceDateOnly(todoDate)
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)
	dayEnd := time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, location)
	result := tododao.PlanningSources{Timezone: timezone, GapMinutes: user.MinimumGapMinutes}

	rows, err := r.queries.ListTodoPlanningActionItemSources(ctx, sqlc.ListTodoPlanningActionItemSourcesParams{
		UserID: userID, TodoDate: pgDate(date),
	})
	if err != nil {
		return tododao.PlanningSources{}, err
	}
	result.Candidates = make([]tododao.WorkSource, 0, len(rows))
	for _, row := range rows {
		result.Candidates = append(result.Candidates, actionItemSourceFromPlanningRow(row, location))
	}
	taskRows, err := r.queries.ListTodoPlanningTaskSources(ctx, sqlc.ListTodoPlanningTaskSourcesParams{UserID: userID, TodoDate: pgDate(date)})
	if err != nil {
		return tododao.PlanningSources{}, err
	}
	for _, row := range taskRows {
		result.Candidates = append(result.Candidates, taskSourceFromPlanningRow(row, location))
	}
	historyRows, err := r.queries.ListTodoActionItemHistory(ctx, sqlc.ListTodoActionItemHistoryParams{UserID: userID, TodoDate: pgDate(date)})
	if err != nil {
		return tododao.PlanningSources{}, err
	}
	for _, row := range historyRows {
		if !row.ActionItemID.Valid || !row.PlannedMinutes.Valid || !row.CompletedAt.Valid {
			continue
		}
		result.History = append(result.History, tododao.HistoryFrame{
			ID: row.ID, Kind: "action_item", ActionItemID: row.ActionItemID.String, PlannedMinutes: int(row.PlannedMinutes.Int32), CompletedAt: row.CompletedAt.Time,
		})
	}
	taskHistory, err := r.queries.ListTodoTaskHistory(ctx, sqlc.ListTodoTaskHistoryParams{UserID: userID, TodoDate: pgDate(date)})
	if err != nil {
		return tododao.PlanningSources{}, err
	}
	for _, row := range taskHistory {
		if !row.TaskID.Valid || !row.PlannedMinutes.Valid || !row.CompletedAt.Valid {
			continue
		}
		result.History = append(result.History, tododao.HistoryFrame{ID: row.ID, Kind: "task", TaskID: row.TaskID.String, PlannedMinutes: int(row.PlannedMinutes.Int32), CompletedAt: row.CompletedAt.Time})
	}

	weekStart := date.AddDate(0, 0, -((int(date.Weekday()) + 6) % 7))
	weekRepo := todoAllocationCalendar{Repository: r.allocation, timezone: timezone}
	week, err := allocationusecase.NewGetWeekUseCase(weekRepo).Execute(ctx, userID, weekStart)
	if err != nil {
		return tododao.PlanningSources{}, err
	}
	for _, interval := range week.SleepIntervals {
		start, end := interval.StartAt, interval.EndAt
		if start.Before(dayStart) {
			start = dayStart
		}
		if end.After(dayEnd) {
			end = dayEnd
		}
		if end.After(start) {
			result.Sleep = append(result.Sleep, tododao.Interval{Start: start, End: end})
		}
	}
	for _, allocation := range week.Allocations {
		if allocation.EndAt.After(dayStart) && allocation.StartAt.Before(dayEnd) {
			result.Allocations = append(result.Allocations, tododao.ProjectAllocation{
				ProjectID: allocation.ProjectID, StartAt: allocation.StartAt, EndAt: allocation.EndAt,
			})
		}
	}

	result.Schedules, err = r.readScheduleOccurrences(ctx, userID, dayStart, dayEnd)
	if err != nil {
		return tododao.PlanningSources{}, err
	}
	sort.Slice(result.Schedules, func(i, j int) bool {
		if !result.Schedules[i].StartAt.Equal(result.Schedules[j].StartAt) {
			return result.Schedules[i].StartAt.Before(result.Schedules[j].StartAt)
		}
		if result.Schedules[i].RuleID != result.Schedules[j].RuleID {
			return result.Schedules[i].RuleID < result.Schedules[j].RuleID
		}
		return result.Schedules[i].OccurrenceDate.Before(result.Schedules[j].OccurrenceDate)
	})

	priorities, err := r.priorities.List(ctx)
	if err != nil {
		return tododao.PlanningSources{}, err
	}
	result.PriorityWeights = make(map[string]int, len(priorities))
	for _, priority := range priorities {
		result.PriorityWeights[priority.Value] = priority.Weight
	}
	return result, nil
}

func (r *todoSourceReader) ReadSavedScheduleSources(ctx context.Context, userID string, todoDate time.Time, timezone string) ([]tododao.ScheduleOccurrence, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}
	date := sourceDateOnly(todoDate)
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)
	dayEnd := time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, location)
	return r.readScheduleOccurrences(ctx, userID, dayStart, dayEnd)
}

func (r *todoSourceReader) readScheduleOccurrences(ctx context.Context, userID string, dayStart, dayEnd time.Time) ([]tododao.ScheduleOccurrence, error) {
	scheduleRepos := scheduleRepositories{store: ScheduleStore{Schedules: r.schedules}}
	scheduleRows, err := scheduleusecase.NewListOccurrencesUseCase(nil).RangeInTx(
		ctx, userID, dayStart.UTC(), dayEnd.UTC(), scheduleRepos,
	)
	if err != nil {
		return nil, err
	}
	capabilities, err := scheduleRepos.Rules().ListOccurrenceCapabilities(ctx, userID, scheduleRows)
	if err != nil {
		return nil, err
	}
	result := make([]tododao.ScheduleOccurrence, 0, len(scheduleRows))
	for _, occurrence := range scheduleRows {
		if occurrence.AssigneeID != userID {
			continue
		}
		occurrenceDate, parseErr := time.Parse("2006-01-02", occurrence.OccurrenceDate)
		if parseErr != nil {
			return nil, fmt.Errorf("parse Schedule occurrence date: %w", parseErr)
		}
		capability := capabilities[occurrence.RuleID+"/"+occurrence.OccurrenceDate]
		result = append(result, tododao.ScheduleOccurrence{
			RuleID: occurrence.RuleID, OccurrenceDate: occurrenceDate,
			Title: occurrence.Title, Description: occurrence.Description, ProjectID: occurrence.ProjectID,
			StartAt: time.Unix(occurrence.StartAt, 0).UTC(), EndAt: time.Unix(occurrence.EndAt, 0).UTC(),
			AllDay: occurrence.AllDay, Completed: occurrence.Completed, Revision: occurrence.Revision,
			CanComplete: capability.CanUpdate && !occurrence.Completed, CanReopen: capability.CanUpdate,
			CanHide: capability.CanUpdate, CanRestore: capability.CanUpdate,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].StartAt.Equal(result[j].StartAt) {
			return result[i].StartAt.Before(result[j].StartAt)
		}
		if result[i].RuleID != result[j].RuleID {
			return result[i].RuleID < result[j].RuleID
		}
		return result[i].OccurrenceDate.Before(result[j].OccurrenceDate)
	})
	return result, nil
}

func (r *todoSourceReader) ReadWorkSources(ctx context.Context, userID string, todoDate time.Time, timezone string, refs []tododao.WorkSourceRef) ([]tododao.WorkSource, error) {
	if len(refs) == 0 {
		return []tododao.WorkSource{}, nil
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}
	todoDate = sourceDateOnly(todoDate)
	actionIDs, taskIDs := make([]string, 0, len(refs)), make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Kind == "task" {
			taskIDs = append(taskIDs, ref.ID)
		} else if ref.Kind == "action_item" {
			actionIDs = append(actionIDs, ref.ID)
		}
	}
	result := make([]tododao.WorkSource, 0, len(refs))
	if len(actionIDs) > 0 {
		rows, queryErr := r.queries.ListTodoActionItemSourcesByIDs(ctx, sqlc.ListTodoActionItemSourcesByIDsParams{UserID: userID, TodoDate: pgDate(todoDate), ActionItemIds: actionIDs})
		if queryErr != nil {
			return nil, queryErr
		}
		for _, row := range rows {
			result = append(result, actionItemSourceFromIDRow(row, location))
		}
	}
	if len(taskIDs) > 0 {
		rows, queryErr := r.queries.ListTodoTaskSourcesByIDs(ctx, sqlc.ListTodoTaskSourcesByIDsParams{UserID: userID, TodoDate: pgDate(todoDate), TaskIds: taskIDs})
		if queryErr != nil {
			return nil, queryErr
		}
		for _, row := range rows {
			result = append(result, taskSourceFromIDRow(row, location))
		}
	}
	return result, nil
}

func actionItemSourceFromPlanningRow(row sqlc.ListTodoPlanningActionItemSourcesRow, location *time.Location) tododao.WorkSource {
	return todoActionItemSource(row.ID, row.TaskID, row.TaskTitle, row.ProjectID, row.ProjectTitle, row.Title, row.Description,
		row.TaskStatus, row.Priority, int(row.PriorityWeight), row.DueAt, int(row.EstimatedMinutes), int(row.CompletedBeforeDayMinutes),
		row.HasCompletedFrame, row.Completed, row.ActualMinutes, row.CanUpdate, row.CanAddToToday.Bool, todoSourceBool(row.CanAddActionItem), todoSourceBool(row.EstimateEditable), location, row.CreatedAt.Time)
}

func actionItemSourceFromIDRow(row sqlc.ListTodoActionItemSourcesByIDsRow, location *time.Location) tododao.WorkSource {
	return todoActionItemSource(row.ID, row.TaskID, row.TaskTitle, row.ProjectID, row.ProjectTitle, row.Title, row.Description,
		row.TaskStatus, row.Priority, int(row.PriorityWeight), row.DueAt, int(row.EstimatedMinutes), int(row.CompletedBeforeDayMinutes),
		row.HasCompletedFrame, row.Completed, row.ActualMinutes, row.CanUpdate, row.CanAddToToday.Bool, todoSourceBool(row.CanAddActionItem), todoSourceBool(row.EstimateEditable), location, row.CreatedAt.Time)
}

func todoActionItemSource(id, taskID, taskTitle string, projectID pgtype.Text, projectTitle string, title, description, taskStatus, priority string,
	priorityWeight int, dueAt pgtype.Date, estimate, historicalMinutes int, hasCompletedFrame, completed bool,
	actualMinutes pgtype.Int4, canUpdate, canAddToToday, canAddActionItem, estimateEditable bool, location *time.Location, createdAt time.Time) tododao.WorkSource {
	var due *time.Time
	if dueAt.Valid && location != nil {
		value := time.Date(dueAt.Time.Year(), dueAt.Time.Month(), dueAt.Time.Day(), 0, 0, 0, 0, location)
		due = &value
	}
	var actual *int
	if actualMinutes.Valid {
		value := int(actualMinutes.Int32)
		actual = &value
	}
	return tododao.WorkSource{Kind: "action_item", ID: id, TaskID: taskID, TaskTitle: taskTitle,
		ProjectID: pgTextValue(projectID), ProjectTitle: projectTitle, Title: title, Description: description,
		TaskStatus: taskStatus, Priority: priority, PriorityWeight: priorityWeight, DueAt: due,
		EstimatedMinutes: estimate, CompletedBeforeDayMinutes: historicalMinutes, HasCompletedFrame: hasCompletedFrame,
		Completed: completed, ActualMinutes: actual, CanComplete: canUpdate, CanReopen: canUpdate,
		CanAddToToday: canAddToToday, CanAddActionItem: canAddActionItem, EstimateEditable: estimateEditable, CanRemoveFromToday: canUpdate, CreatedAt: createdAt,
	}
}

func taskSourceFromPlanningRow(row sqlc.ListTodoPlanningTaskSourcesRow, location *time.Location) tododao.WorkSource {
	estimate := 0
	if row.EstimatedMinutes.Valid {
		estimate = int(row.EstimatedMinutes.Int32)
	}
	return taskWorkSource(row.ID, row.TaskTitle, row.ProjectID, row.ProjectTitle, row.Description, row.TaskStatus, row.Priority, int(row.PriorityWeight), row.DueAt, estimate, int(row.CompletedBeforeDayMinutes), row.HasCompletedFrame, row.Completed, row.ActualMinutes, row.CanUpdate, row.CanAddToToday.Bool, todoSourceBool(row.CanAddActionItem), todoSourceBool(row.EstimateEditable), location, row.CreatedAt.Time)
}

func taskSourceFromIDRow(row sqlc.ListTodoTaskSourcesByIDsRow, location *time.Location) tododao.WorkSource {
	return taskWorkSource(row.ID, row.TaskTitle, row.ProjectID, row.ProjectTitle, row.Description, row.TaskStatus, row.Priority, int(row.PriorityWeight), row.DueAt, int(row.EstimatedMinutes), int(row.CompletedBeforeDayMinutes), row.HasCompletedFrame, row.Completed, row.ActualMinutes, row.CanUpdate, row.CanAddToToday.Bool, todoSourceBool(row.CanAddActionItem), todoSourceBool(row.EstimateEditable), location, row.CreatedAt.Time)
}

func taskWorkSource(id, title string, projectID pgtype.Text, projectTitle, description, status, priority string, weight int, dueAt pgtype.Date, estimate, history int, started, completed bool, actual pgtype.Int4, canUpdate, canAdd, canAddAction, estimateEditable bool, location *time.Location, created time.Time) tododao.WorkSource {
	var due *time.Time
	if dueAt.Valid && location != nil {
		value := time.Date(dueAt.Time.Year(), dueAt.Time.Month(), dueAt.Time.Day(), 0, 0, 0, 0, location)
		due = &value
	}
	var actualValue *int
	if actual.Valid {
		value := int(actual.Int32)
		actualValue = &value
	}
	return tododao.WorkSource{Kind: "task", ID: id, TaskID: id, TaskTitle: title, ProjectID: pgTextValue(projectID), ProjectTitle: projectTitle, Title: title, Description: description, TaskStatus: status, Priority: priority, PriorityWeight: weight, DueAt: due, EstimatedMinutes: estimate, CompletedBeforeDayMinutes: history, HasCompletedFrame: started, Completed: completed, ActualMinutes: actualValue, CanComplete: canUpdate && !completed, CanReopen: canUpdate, CanAddToToday: canAdd, CanAddActionItem: canAddAction, EstimateEditable: estimateEditable, CanRemoveFromToday: canUpdate, CreatedAt: created}
}

func pgDate(value time.Time) pgtype.Date {
	return pgtype.Date{Time: time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

func sourceDateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func pgTextValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
