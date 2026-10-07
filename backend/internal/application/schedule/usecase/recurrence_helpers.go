package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

const (
	repeatStateOneOff   = "one_off"
	repeatStateActive   = "active"
	repeatStateStopped  = "stopped"
	VirtualOccurrenceID = "TASK2TODAYTODO000000000000"
	scopeCurrent        = "current"
	scopeFuture         = "future"
)

type recurrenceWriter interface {
	SetRecurrenceByUserID(context.Context, domain.UserID, domain.ScheduleID, time.Time, int, []dao.Frequency) error
	StopRecurrenceByUserID(context.Context, domain.UserID, domain.ScheduleID) error
}

type skippedOccurrenceStore interface {
	ListSkippedOccurrences(context.Context, domain.UserID, domain.ScheduleID) ([]int64, error)
	SetSkippedOccurrence(context.Context, domain.UserID, domain.ScheduleID, time.Time, bool) error
}

type occurrenceProjectionReader interface {
	ListOccurrencesForPermission(context.Context, domain.UserID, domain.ScheduleID, shared.Capability) ([]dao.Schedule, error)
}

type seriesTemplateWriter interface {
	UpdateSeriesTemplateByUserID(context.Context, domain.UserID, domain.ScheduleID, domain.ScheduleID, domain.Schedule) (dao.Schedule, error)
}

type overrideWriter interface {
	UpsertOverrideByUserID(context.Context, domain.UserID, domain.Schedule) (string, error)
}

func frequenciesFromDAO(values []dao.Frequency) (domain.Frequencies, error) {
	result := make(domain.Frequencies, 0, len(values))
	for _, value := range values {
		frequency, err := recurrence.NewFrequency(value.Value)
		if err != nil {
			return nil, err
		}
		result = append(result, frequency)
	}
	return result, nil
}

func frequenciesDAO(frequencies domain.Frequencies) []dao.Frequency {
	result := make([]dao.Frequency, 0, len(frequencies))
	for _, frequency := range frequencies {
		result = append(result, dao.Frequency{Value: frequency.Value, Label: frequency.Label, LabelJp: frequency.LabelJp})
	}
	return result
}

func localToday(now time.Time, location *time.Location) time.Time {
	return calendar.CalendarDate(now, location)
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

func recurrenceLocation(timezone string) *time.Location {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.UTC
	}
	return location
}

func scheduleIsRecurring(root dao.Schedule) bool {
	if root.RepeatState != "" {
		return root.RepeatState != repeatStateOneOff
	}
	return root.IntervalWeeks > domain.OnceIntervalWeeks
}

func requireOverride(repo any) (overrideWriter, error) {
	writer, ok := repo.(overrideWriter)
	if !ok {
		return nil, errors.New("schedule override repository is unavailable")
	}
	return writer, nil
}

func requireSeriesTemplate(repo any) (seriesTemplateWriter, error) {
	writer, ok := repo.(seriesTemplateWriter)
	if !ok {
		return nil, errors.New("schedule series template repository is unavailable")
	}
	return writer, nil
}

func requireRecurrenceWriter(repo any) (recurrenceWriter, error) {
	writer, ok := repo.(recurrenceWriter)
	if !ok {
		return nil, errors.New("schedule recurrence repository is unavailable")
	}
	return writer, nil
}

func requireSkippedOccurrenceStore(repo any) (skippedOccurrenceStore, error) {
	store, ok := repo.(skippedOccurrenceStore)
	if !ok {
		return nil, errors.New("schedule skipped occurrence repository is unavailable")
	}
	return store, nil
}

func validateOccurrenceDate(root dao.Schedule, date time.Time) error {
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
	frequencies, err := frequenciesFromDAO(root.Frequencies)
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
