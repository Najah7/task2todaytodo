package domain

import (
	"time"
)

type Policy struct {
	PriorityWeights   map[string]int
	GapMinutes        int
	UrgencySameOrPast int
	Urgency1To7       int
	Urgency8To14      int
	Urgency15To30     int
	Urgency31Plus     int
	UrgencyNone       int
}

func ScoringPolicy() Policy {
	return Policy{
		UrgencySameOrPast: 200,
		Urgency1To7:       100,
		Urgency8To14:      50,
		Urgency15To30:     25,
		Urgency31Plus:     10,
		UrgencyNone:       1,
	}
}

func score(c Candidate, today time.Time, location *time.Location) int {
	policy := ScoringPolicy()
	urgency := policy.UrgencyNone
	if c.DueAt != nil {
		days := calendarDaysBetween(today.In(location), c.DueAt.In(location))
		switch {
		case days <= 0:
			urgency = policy.UrgencySameOrPast
		case days <= 7:
			urgency = policy.Urgency1To7
		case days <= 14:
			urgency = policy.Urgency8To14
		case days <= 30:
			urgency = policy.Urgency15To30
		default:
			urgency = policy.Urgency31Plus
		}
	}
	return c.PriorityWeight * urgency
}

func calendarDaysBetween(today, due time.Time) int {
	// Inputs are local-date values in the caller's timezone. UTC date arithmetic
	// avoids daylight-saving duration changing the calendar-day boundary.
	a := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}

func localDateValue(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}
