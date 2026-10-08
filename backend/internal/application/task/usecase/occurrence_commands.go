package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	calendar "github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type todoOverrideWriter interface {
	UpsertTodoItemOverride(context.Context, domain.UserID, domain.TodoItem) (string, error)
}

func listTodoOccurrenceProjection(ctx context.Context, repo TodoItemRepository, userID domain.UserID, taskID domain.TaskID) ([]dao.TodoItem, error) {
	if reader, ok := repo.(todoOccurrenceProjectionReader); ok {
		return reader.ListByTaskForOccurrenceProjection(ctx, userID, taskID)
	}
	return repo.ListByTask(ctx, userID, taskID)
}

func listTodoOccurrenceCommandProjection(ctx context.Context, repo TodoItemRepository, userID domain.UserID, taskID domain.TaskID, capability shared.Capability) ([]dao.TodoItem, error) {
	if reader, ok := repo.(todoOccurrenceCommandReader); ok {
		return reader.ListByTaskForOccurrenceCommand(ctx, userID, taskID, capability)
	}
	return listTodoOccurrenceProjection(ctx, repo, userID, taskID)
}

func getTodoItemForCommand(ctx context.Context, repo TodoItemRepository, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID, capability shared.Capability) (dao.TodoItem, error) {
	if reader, ok := repo.(todoOccurrenceCommandReader); ok {
		return reader.GetForCommand(ctx, userID, taskID, id, capability)
	}
	return repo.GetForOwnedTask(ctx, userID, taskID, id)
}

type todoOccurrenceState struct {
	root        dao.TodoItem
	current     *dao.TodoItem
	date        time.Time
	skipped     bool
	saved       bool
	projectDone bool
}

func frequenciesDAO(frequencies domain.TaskFrequencies) []dao.TaskFrequency {
	result := make([]dao.TaskFrequency, 0, len(frequencies))
	for _, frequency := range frequencies {
		result = append(result, dao.TaskFrequency{Value: frequency.Value, Label: frequency.Label, LabelJp: frequency.LabelJp})
	}
	return result
}

func loadTodoOccurrence(ctx context.Context, repos Repositories, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, occurrenceDate string, asOf time.Time) (todoOccurrenceState, error) {
	return loadTodoOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.TodoItemRead())
}

func loadTodoOccurrenceWithCapability(ctx context.Context, repos Repositories, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, occurrenceDate string, asOf time.Time, capability shared.Capability) (todoOccurrenceState, error) {
	rows, err := listTodoOccurrenceCommandProjection(ctx, repos.TodoItems(), userID, taskID, capability)
	if err != nil {
		return todoOccurrenceState{}, err
	}
	var root dao.TodoItem
	var current *dao.TodoItem
	rootOccurrence := false
	for index := range rows {
		row := rows[index]
		if row.ID == string(seriesID) && row.SeriesID == string(seriesID) {
			root = row
		}
	}
	if root.ID == "" {
		return todoOccurrenceState{}, ErrOccurrenceNotFound
	}
	if root.Deleted {
		return todoOccurrenceState{}, ErrOccurrenceInactive
	}
	if occurrenceDate == "" {
		if root.IntervalWeeks != domain.OnceIntervalWeeks {
			return todoOccurrenceState{}, ErrOccurrenceDateRequired
		}
		occurrenceDate = root.OccurrenceDate
	}
	date, err := parseOccurrenceDate(occurrenceDate)
	if err != nil {
		return todoOccurrenceState{}, err
	}
	for _, row := range rows {
		if row.SeriesID == string(seriesID) && row.OccurrenceDate == occurrenceDate && row.Deleted {
			return todoOccurrenceState{}, ErrOccurrenceInactive
		}
	}
	for index := range rows {
		row := rows[index]
		if row.SeriesID == string(seriesID) && row.OccurrenceDate == occurrenceDate && !row.Deleted && row.ID != string(seriesID) {
			copy := row
			current = &copy
			break
		} else if row.ID == string(seriesID) && row.OccurrenceDate == occurrenceDate && !row.Deleted {
			rootOccurrence = true
		}
	}
	saved := current != nil || rootOccurrence
	if !saved && root.RepeatState == repeatStateStopped {
		return todoOccurrenceState{}, ErrOccurrenceInactive
	}
	if !saved {
		if err := validateTodoOccurrenceDate(root, date); err != nil {
			return todoOccurrenceState{}, err
		}
		location, err := time.LoadLocation(root.Timezone)
		if err != nil {
			return todoOccurrenceState{}, domain.ErrRecurrenceTimezoneInvalid
		}
		if date.Before(localToday(asOf, location)) {
			return todoOccurrenceState{}, ErrOccurrenceInactive
		}
	}
	state := todoOccurrenceState{root: root, current: current, date: date, saved: saved}
	if skipped, ok := repos.TodoItems().(todoSkippedOccurrenceCommandStore); ok {
		dates, err := skipped.ListTodoItemSkippedOccurrencesForCapability(ctx, userID, taskID, seriesID, capability)
		if err != nil {
			return todoOccurrenceState{}, err
		}
		for _, skipped := range dates {
			if time.Unix(skipped, 0).UTC().Format("2006-01-02") == occurrenceDate {
				state.skipped = true
				break
			}
		}
	} else if skipped, ok := repos.TodoItems().(todoSkippedOccurrenceStore); ok {
		dates, err := skipped.ListTodoItemSkippedOccurrences(ctx, userID, taskID, seriesID)
		if err != nil {
			return todoOccurrenceState{}, err
		}
		for _, skipped := range dates {
			if time.Unix(skipped, 0).UTC().Format("2006-01-02") == occurrenceDate {
				state.skipped = true
				break
			}
		}
	}
	if state.skipped {
		return state, ErrOccurrenceInactive
	}
	return state, nil
}

func projectDoneSuppressesTodoOccurrence(state todoOccurrenceState) bool {
	return state.projectDone && !state.saved
}

func validateTodoOccurrenceDate(root dao.TodoItem, date time.Time) error {
	first, err := parseOccurrenceDate(root.OccurrenceDate)
	if err != nil {
		return err
	}
	if root.IntervalWeeks == domain.OnceIntervalWeeks {
		if !date.Equal(first) {
			return ErrOccurrenceInactive
		}
		return nil
	}
	frequencies, err := taskFrequenciesFromDAO(root.Frequencies)
	if err != nil {
		return err
	}
	anchor := first
	if root.FrequencyAnchorDate != 0 {
		anchor = time.Unix(root.FrequencyAnchorDate, 0).UTC()
	}
	dates, err := recurrence.GenerateRecurrenceDatesFromAnchorLimit(anchor, date, root.IntervalWeeks, frequencies, root.Timezone, 1)
	if err != nil {
		return err
	}
	if len(dates) == 0 || !dates[0].Date.Equal(date) {
		return ErrOccurrenceRuleMismatch
	}
	return nil
}

func todoDomainOccurrence(root dao.TodoItem, current *dao.TodoItem, date time.Time, id domain.TodoItemID, completed bool, asOf time.Time) (domain.TodoItem, error) {
	source := root
	if current != nil && (current.ID != root.ID || date.Equal(mustParseDate(root.OccurrenceDate))) {
		source = *current
	}
	frequencies, err := taskFrequenciesFromDAO(root.Frequencies)
	if err != nil {
		return domain.TodoItem{}, err
	}
	dueDate := time.Time{}
	if source.DueDate != 0 {
		if current != nil && (current.IsException || current.ID == root.ID) {
			dueDate = time.Unix(source.DueDate, 0).UTC()
		} else {
			dueDate = date
		}
	}
	return domain.NewExistingTodoItem(id, domain.TaskID(root.TaskID), source.Title, source.Description, dueDate, completed,
		source.Position, root.IntervalWeeks, frequencies, time.Unix(source.CreatedAt, 0), asOf,
		domain.RecurrenceMetadata{SeriesID: root.ID, OccurrenceDate: date, Timezone: root.Timezone, IsException: true})
}

func todoDAOFromOccurrence(root dao.TodoItem, item domain.TodoItem, id string) dao.TodoItem {
	var due int64
	if !item.DueDate.IsZero() {
		due = item.DueDate.Unix()
	}
	return dao.TodoItem{ID: id, TaskID: string(item.TaskID), Title: item.Title, Description: item.Description, DueDate: due, Completed: item.Completed, Position: item.Position,
		IntervalWeeks: root.IntervalWeeks, Frequencies: root.Frequencies, RepeatState: root.RepeatState, FrequencyAnchorDate: root.FrequencyAnchorDate, SeriesID: root.ID, OccurrenceDate: item.OccurrenceDate.Format("2006-01-02"), Timezone: root.Timezone,
		IsException: true, CreatedAt: item.CreatedAt.Unix(), UpdatedAt: item.UpdatedAt.Unix()}
}

func parseOccurrenceDate(value string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil || date.Format("2006-01-02") != value {
		return time.Time{}, ErrOccurrenceRuleMismatch
	}
	return date, nil
}

func mustParseDate(value string) time.Time {
	date, _ := time.Parse("2006-01-02", value)
	return date
}

func skippedDate(value time.Time) time.Time {
	return calendar.NormalizeCalendarDate(value)
}

func requireTodoOverride(repo any) (todoOverrideWriter, error) {
	writer, ok := repo.(todoOverrideWriter)
	if !ok {
		return nil, errors.New("todo item override repository is unavailable")
	}
	return writer, nil
}

func generatedTodoItemID(ID shared.ID) (domain.TodoItemID, error) {
	if ID == nil {
		return "", fmt.Errorf("%w: ID generator unavailable", ErrOccurrenceInactive)
	}
	return domain.TodoItemID(ID.Generate()), nil
}
