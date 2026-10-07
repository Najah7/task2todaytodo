package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

type occurrenceState struct {
	root           dao.Schedule
	current        *dao.Schedule
	date           time.Time
	skipped        bool
	rootOccurrence bool
}

func loadOccurrence(ctx context.Context, repo ScheduleRepository, actorID domain.UserID, seriesID domain.ScheduleID, occurrenceDate string, asOf time.Time, capability shared.Capability) (occurrenceState, error) {
	rows, err := repo.ListOccurrencesForPermission(ctx, actorID, seriesID, capability)
	if err != nil {
		return occurrenceState{}, err
	}
	var root dao.Schedule
	for _, row := range rows {
		if row.ID == string(seriesID) && row.SeriesID == string(seriesID) {
			root = row
			break
		}
	}
	if root.ID == "" {
		return occurrenceState{}, ErrOccurrenceNotFound
	}
	if root.Deleted {
		return occurrenceState{}, ErrOccurrenceInactive
	}
	if occurrenceDate == "" {
		if root.IntervalWeeks != domain.OnceIntervalWeeks {
			return occurrenceState{}, ErrOccurrenceDateRequired
		}
		occurrenceDate = root.OccurrenceDate
	}
	date, err := parseOccurrenceDate(occurrenceDate)
	if err != nil {
		return occurrenceState{}, err
	}
	for _, row := range rows {
		if row.SeriesID == string(seriesID) && row.OccurrenceDate == occurrenceDate && row.Deleted {
			return occurrenceState{}, ErrOccurrenceInactive
		}
	}
	var current *dao.Schedule
	rootOccurrence := false
	for index := range rows {
		row := rows[index]
		if row.SeriesID == string(seriesID) && row.OccurrenceDate == occurrenceDate && !row.Deleted && row.ID != string(seriesID) {
			copy := row
			current = &copy
			break
		}
		if row.ID == string(seriesID) && row.OccurrenceDate == occurrenceDate && !row.Deleted {
			rootOccurrence = true
		}
	}
	saved := current != nil || rootOccurrence
	if !saved && root.RepeatState == repeatStateStopped {
		return occurrenceState{}, ErrOccurrenceInactive
	}
	if !saved {
		if err := validateOccurrenceDate(root, date); err != nil {
			return occurrenceState{}, err
		}
		location, err := time.LoadLocation(root.Timezone)
		if err != nil {
			return occurrenceState{}, recurrence.ErrRecurrenceTimezoneInvalid
		}
		if date.Before(localToday(asOf, location)) {
			return occurrenceState{}, ErrOccurrenceInactive
		}
		startTemplate, endTemplate := time.Unix(root.StartAt, 0), time.Unix(root.EndAt, 0)
		if _, valid := calendar.ResolveWallTime(date, startTemplate.In(location), 0, location); !valid {
			return occurrenceState{}, ErrOccurrenceInactive
		}
		if _, valid := calendar.ResolveWallTime(date, endTemplate.In(location), calendar.CalendarDayOffset(startTemplate.In(location), endTemplate.In(location)), location); !valid {
			return occurrenceState{}, ErrOccurrenceInactive
		}
	}
	state := occurrenceState{root: root, current: current, date: date, rootOccurrence: rootOccurrence}
	if skippedDates, err := repo.ListSkippedOccurrencesForPermission(ctx, actorID, seriesID, capability); err != nil {
		return occurrenceState{}, err
	} else {
		for _, skipped := range skippedDates {
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

func scheduleDomainOccurrence(root dao.Schedule, current *dao.Schedule, date time.Time, id domain.ScheduleID, completed bool, asOf time.Time) (domain.Schedule, error) {
	source := root
	if current != nil && (current.ID != root.ID || date.Equal(mustParseDate(root.OccurrenceDate))) {
		source = *current
	}
	location := recurrenceLocation(root.Timezone)
	startTemplate, endTemplate := time.Unix(root.StartAt, 0).In(location), time.Unix(root.EndAt, 0).In(location)
	startAt, startOK := calendar.ResolveWallTime(date, startTemplate, 0, location)
	endAt, endOK := calendar.ResolveWallTime(date, endTemplate, calendar.CalendarDayOffset(startTemplate, endTemplate), location)
	if current != nil && current.IsException {
		startAt, endAt = time.Unix(current.StartAt, 0), time.Unix(current.EndAt, 0)
	}
	if !startOK || !endOK {
		return domain.Schedule{}, ErrOccurrenceInactive
	}
	frequencies, err := frequenciesFromDAO(root.Frequencies)
	if err != nil {
		return domain.Schedule{}, err
	}
	return domain.NewExistingSchedule(id, domain.UserID(root.UserID), domain.ProjectID(root.ProjectID), domain.UserID(root.AssigneeID), source.Title, source.Description, source.Location,
		root.IntervalWeeks, frequencies, startAt, endAt, completed, time.Unix(source.CreatedAt, 0), asOf,
		recurrence.Metadata{SeriesID: root.ID, OccurrenceDate: date, Timezone: root.Timezone, IsException: true})
}

func generatedScheduleID(ID shared.ID) (domain.ScheduleID, error) {
	if ID == nil {
		return "", fmt.Errorf("%w: ID generator unavailable", ErrOccurrenceInactive)
	}
	return domain.ScheduleID(ID.Generate()), nil
}
