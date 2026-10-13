package domain

import (
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

var (
	ErrInvalidWeekday       = errors.New("weekday must be from 0 through 6")
	ErrInvalidLocalTime     = errors.New("invalid local time")
	ErrInvalidEndDayOffset  = errors.New("end day offset must be 0 or 1")
	ErrNonexistentLocalTime = errors.New("local time does not exist in timezone")
)

type Slot struct {
	Weekday      int
	Start        time.Time
	End          time.Time
	EndDayOffset int
	Timezone     string
	Frequency    string
}

type Occurrence struct {
	ID               string
	RuleID           string
	SourceRevisionID string
	ProjectID        string
	OccurrenceDate   time.Time
	StartAt          time.Time
	EndAt            time.Time
	Timezone         string
}

type ActiveTimeException struct {
	LocalDate        time.Time
	ActiveStartLocal time.Time
	ActiveEndLocal   time.Time
	Timezone         string
}

func NewSlot(weekday int, startText, endText string, endDayOffset int, timezone, frequency string) (Slot, error) {
	if weekday < 0 || weekday > 6 {
		return Slot{}, ErrInvalidWeekday
	}
	start, err := time.Parse("15:04", startText)
	if err != nil {
		return Slot{}, ErrInvalidLocalTime
	}
	end, err := time.Parse("15:04", endText)
	if err != nil {
		return Slot{}, ErrInvalidLocalTime
	}
	if endDayOffset < 0 || endDayOffset > 1 {
		return Slot{}, ErrInvalidEndDayOffset
	}
	if (endDayOffset == 0 && !end.After(start)) || (endDayOffset == 1 && end.After(start)) {
		return Slot{}, ErrInvalidLocalTime
	}
	if frequency != "weekly" && frequency != "once" {
		return Slot{}, ErrInvalidLocalTime
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return Slot{}, calendar.ErrRecurrenceTimezoneInvalid
	}
	return Slot{Weekday: weekday, Start: start, End: end, EndDayOffset: endDayOffset, Timezone: timezone, Frequency: frequency}, nil
}

func (s Slot) At(date time.Time) (Interval, error) {
	location, _ := time.LoadLocation(s.Timezone)
	start, ok := calendar.ResolveEarlier(date, s.Start, 0, location)
	if !ok {
		return Interval{}, ErrNonexistentLocalTime
	}
	end, ok := calendar.ResolveEarlier(date, s.End, s.EndDayOffset, location)
	if !ok || !end.After(start) {
		return Interval{}, ErrNonexistentLocalTime
	}
	return Interval{Start: start, End: end}, nil
}
