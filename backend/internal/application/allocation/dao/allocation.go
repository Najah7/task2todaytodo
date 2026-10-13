package dao

import "time"

type Rule struct {
	ID                  string
	ProjectID           string
	ProjectTitle        string
	Revision            int32
	CurrentRevision     int32
	SourceRevisionID    string
	Weekday             int
	StartTime           string
	EndTime             string
	EndDayOffset        int
	Timezone            string
	Frequency           string
	EffectiveFrom       time.Time
	RecurrenceStartDate time.Time
	EffectiveUntil      *time.Time
	EffectiveUntilAt    *time.Time
	EffectiveAt         time.Time
	PausedReason        string
	PausedAt            *time.Time
	StoppedAfter        *time.Time
	DeletedAt           *time.Time
}

type Project struct {
	ID                string
	Title             string
	Status            string
	TargetMinutes     int
	AllocatedMinutes  int
	DifferenceMinutes int
}

type Allocation struct {
	ID                 string
	RuleID             string
	OccurrenceDate     time.Time
	SourceRevisionID   string
	ProjectID          string
	ProjectTitle       string
	Timezone           string
	StartAt            time.Time
	EndAt              time.Time
	Revision           int32
	RuleRevision       int32
	IsException        bool
	CanUpdate          bool
	CanDelete          bool
	CanUpdateFollowing bool
	CanDeleteFollowing bool
}

type Day struct {
	Date               time.Time
	ActiveStartAt      time.Time
	ActiveEndAt        time.Time
	ActiveRevision     int32
	HasActiveException bool
	AllocatedMinutes   int
	FreeMinutes        int
}

type Interval struct {
	StartAt   time.Time
	EndAt     time.Time
	LocalDate time.Time
}

type PausedRule struct {
	RuleID    string
	ProjectID string
	Reason    string
}

type Week struct {
	WeekStart         time.Time
	Timezone          string
	Projects          []Project
	Allocations       []Allocation
	Days              []Day
	ActiveIntervals   []Interval
	AllocatedMinutes  int
	TargetMinutes     int
	DifferenceMinutes int
	FreeMinutes       int
	PausedRules       []PausedRule
}

type WeekSource struct {
	WeekStart            time.Time
	Timezone             string
	ActiveStartTime      string
	ActiveEndTime        string
	Projects             []Project
	Allocations          []Allocation
	ActiveTimeExceptions []ActiveTimeException
}

type Profile struct {
	Timezone        string
	ActiveStartTime string
	ActiveEndTime   string
}

type RuleCollection struct {
	Revision string
	Timezone string
	Items    []Rule
}

type PlanItem struct {
	ProjectID     string
	Title         string
	TargetMinutes int
}

type PlanCollection struct {
	Revision string
	Items    []PlanItem
}

type ActiveTimeException struct {
	LocalDate        time.Time
	ActiveStartLocal time.Time
	ActiveEndLocal   time.Time
	Timezone         string
	Revision         int32
	HasException     bool
}

type PauseInterval struct {
	PausedAt  time.Time
	ResumedAt *time.Time
}

type Occurrence struct {
	ID                 string
	RuleID             string
	OccurrenceDate     time.Time
	SourceRevisionID   string
	ProjectID          string
	ProjectTitle       string
	Timezone           string
	StartAt            time.Time
	EndAt              time.Time
	Revision           int32
	RuleRevision       int32
	IsException        bool
	CanUpdate          bool
	CanDelete          bool
	CanUpdateFollowing bool
	CanDeleteFollowing bool
}
