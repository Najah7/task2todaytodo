package domain

import (
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

var (
	ErrInvalidInterval = errors.New("interval end must be after start")
	ErrIntervalOverlap = errors.New("allocation intervals overlap")
)

type Interval struct {
	Start time.Time
	End   time.Time
}

func NewInterval(start, end time.Time) (Interval, error) {
	start, end = start.UTC(), end.UTC()
	if !end.After(start) {
		return Interval{}, ErrInvalidInterval
	}
	return Interval{Start: start, End: end}, nil
}

func (i Interval) Overlaps(other Interval) bool {
	return calendar.Overlaps(i.Start, i.End, other.Start, other.End)
}

// ValidateNoOverlap permits adjacent half-open intervals.
func ValidateNoOverlap(intervals []Interval) error {
	for i := range intervals {
		if !intervals[i].End.After(intervals[i].Start) {
			return ErrInvalidInterval
		}
		for j := 0; j < i; j++ {
			if intervals[i].Overlaps(intervals[j]) {
				return ErrIntervalOverlap
			}
		}
	}
	return nil
}
