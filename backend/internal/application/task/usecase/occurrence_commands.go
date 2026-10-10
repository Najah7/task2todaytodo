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

type actionItemOverrideWriter interface {
	UpsertActionItemOverride(context.Context, domain.UserID, domain.ActionItem) (string, error)
}

func listActionItemOccurrenceProjection(ctx context.Context, repo ActionItemRepository, userID domain.UserID, taskID domain.TaskID) ([]dao.ActionItem, error) {
	if reader, ok := repo.(actionItemOccurrenceProjectionReader); ok {
		return reader.ListByTaskForOccurrenceProjection(ctx, userID, taskID)
	}
	return repo.ListByTask(ctx, userID, taskID)
}

func listActionItemOccurrenceCommandProjection(ctx context.Context, repo ActionItemRepository, userID domain.UserID, taskID domain.TaskID, capability shared.Capability) ([]dao.ActionItem, error) {
	if reader, ok := repo.(actionItemOccurrenceCommandReader); ok {
		return reader.ListByTaskForOccurrenceCommand(ctx, userID, taskID, capability)
	}
	return listActionItemOccurrenceProjection(ctx, repo, userID, taskID)
}

func getActionItemForCommand(ctx context.Context, repo ActionItemRepository, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, capability shared.Capability) (dao.ActionItem, error) {
	if reader, ok := repo.(actionItemOccurrenceCommandReader); ok {
		return reader.GetForCommand(ctx, userID, taskID, id, capability)
	}
	return repo.GetForOwnedTask(ctx, userID, taskID, id)
}

type actionItemOccurrenceState struct {
	root        dao.ActionItem
	current     *dao.ActionItem
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

func loadActionItemOccurrence(ctx context.Context, repos Repositories, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, occurrenceDate string, asOf time.Time) (actionItemOccurrenceState, error) {
	return loadActionItemOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.ActionItemRead())
}

func loadActionItemOccurrenceWithCapability(ctx context.Context, repos Repositories, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, occurrenceDate string, asOf time.Time, capability shared.Capability) (actionItemOccurrenceState, error) {
	rows, err := listActionItemOccurrenceCommandProjection(ctx, repos.ActionItems(), userID, taskID, capability)
	if err != nil {
		return actionItemOccurrenceState{}, err
	}
	var root dao.ActionItem
	var current *dao.ActionItem
	rootOccurrence := false
	for index := range rows {
		row := rows[index]
		if row.ID == string(seriesID) && row.SeriesID == string(seriesID) {
			root = row
		}
	}
	if root.ID == "" {
		return actionItemOccurrenceState{}, ErrOccurrenceNotFound
	}
	if root.Deleted {
		return actionItemOccurrenceState{}, ErrOccurrenceInactive
	}
	if occurrenceDate == "" {
		if root.IntervalWeeks != domain.OnceIntervalWeeks {
			return actionItemOccurrenceState{}, ErrOccurrenceDateRequired
		}
		occurrenceDate = root.OccurrenceDate
	}
	date, err := parseOccurrenceDate(occurrenceDate)
	if err != nil {
		return actionItemOccurrenceState{}, err
	}
	for _, row := range rows {
		if row.SeriesID == string(seriesID) && row.OccurrenceDate == occurrenceDate && row.Deleted {
			return actionItemOccurrenceState{}, ErrOccurrenceInactive
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
		return actionItemOccurrenceState{}, ErrOccurrenceInactive
	}
	if !saved {
		if err := validateActionItemOccurrenceDate(root, date); err != nil {
			return actionItemOccurrenceState{}, err
		}
		location, err := time.LoadLocation(root.Timezone)
		if err != nil {
			return actionItemOccurrenceState{}, domain.ErrRecurrenceTimezoneInvalid
		}
		if date.Before(localToday(asOf, location)) {
			return actionItemOccurrenceState{}, ErrOccurrenceInactive
		}
	}
	state := actionItemOccurrenceState{root: root, current: current, date: date, saved: saved}
	if skipped, ok := repos.ActionItems().(actionItemSkippedOccurrenceCommandStore); ok {
		dates, err := skipped.ListActionItemSkippedOccurrencesForCapability(ctx, userID, taskID, seriesID, capability)
		if err != nil {
			return actionItemOccurrenceState{}, err
		}
		for _, skipped := range dates {
			if time.Unix(skipped, 0).UTC().Format("2006-01-02") == occurrenceDate {
				state.skipped = true
				break
			}
		}
	} else if skipped, ok := repos.ActionItems().(actionItemSkippedOccurrenceStore); ok {
		dates, err := skipped.ListActionItemSkippedOccurrences(ctx, userID, taskID, seriesID)
		if err != nil {
			return actionItemOccurrenceState{}, err
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

func projectDoneSuppressesActionItemOccurrence(state actionItemOccurrenceState) bool {
	return state.projectDone && !state.saved
}

func validateActionItemOccurrenceDate(root dao.ActionItem, date time.Time) error {
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

func actionItemDomainOccurrence(root dao.ActionItem, current *dao.ActionItem, date time.Time, id domain.ActionItemID, completed bool, asOf time.Time) (domain.ActionItem, error) {
	source := root
	if current != nil && (current.ID != root.ID || date.Equal(mustParseDate(root.OccurrenceDate))) {
		source = *current
	}
	frequencies, err := taskFrequenciesFromDAO(root.Frequencies)
	if err != nil {
		return domain.ActionItem{}, err
	}
	dueDate := time.Time{}
	if source.DueDate != 0 {
		if current != nil && (current.IsException || current.ID == root.ID) {
			dueDate = time.Unix(source.DueDate, 0).UTC()
		} else {
			dueDate = date
		}
	}
	return domain.NewExistingActionItemWithPlanning(id, domain.TaskID(root.TaskID), source.Title, source.Description, dueDate, completed,
		source.Position, root.IntervalWeeks, frequencies, time.Unix(source.CreatedAt, 0), asOf,
		domain.ActionItemPlanning{EstimatedMinutes: source.EstimatedMinutes, Priority: domain.TaskPriority{Value: source.Priority.Value}},
		domain.RecurrenceMetadata{SeriesID: root.ID, OccurrenceDate: date, Timezone: root.Timezone, IsException: true})
}

func actionItemDAOFromOccurrence(root dao.ActionItem, item domain.ActionItem, id string) dao.ActionItem {
	var due int64
	if !item.DueDate.IsZero() {
		due = item.DueDate.Unix()
	}
	return dao.ActionItem{ID: id, TaskID: string(item.TaskID), Title: item.Title, Description: item.Description, DueDate: due, EstimatedMinutes: item.EstimatedMinutes, Priority: dao.Priority{Value: item.Priority.String()}, Completed: item.Completed, Position: item.Position,
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

func requireActionItemOverride(repo any) (actionItemOverrideWriter, error) {
	writer, ok := repo.(actionItemOverrideWriter)
	if !ok {
		return nil, errors.New("action item override repository is unavailable")
	}
	return writer, nil
}

func generatedActionItemID(ID shared.ID) (domain.ActionItemID, error) {
	if ID == nil {
		return "", fmt.Errorf("%w: ID generator unavailable", ErrOccurrenceInactive)
	}
	return domain.ActionItemID(ID.Generate()), nil
}
