package dao

import "time"

type Todo struct {
	ID        string
	UserID    string
	Date      time.Time // Calendar date in the user's timezone.
	Timezone  string
	Revision  int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Item struct {
	ID             string
	TodoID         string
	Kind           string
	ActionItemID   string
	TaskID         string
	ScheduleRuleID string
	OccurrenceDate time.Time // Source occurrence's original local date.
	PlannedMinutes *int
	ScheduledStart *time.Time
	ScheduledEnd   *time.Time
	Completed      bool
	Hidden         bool
	Position       int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CurrentDay struct {
	Date     time.Time
	Timezone string
	Gap      int
}

type PlanningSources struct {
	Timezone        string
	GapMinutes      int
	PriorityWeights map[string]int
	Candidates      []WorkSource
	Schedules       []ScheduleOccurrence
	ActiveTime      []Interval
	Allocations     []ProjectAllocation
	History         []HistoryFrame
}

type WorkSourceRef struct {
	Kind string
	ID   string
}

type WorkSource struct {
	Kind                      string
	ID                        string
	TaskID                    string
	TaskTitle                 string
	ProjectID                 string
	ProjectTitle              string
	Title                     string
	Description               string
	TaskStatus                string
	Priority                  string
	PriorityWeight            int
	DueAt                     *time.Time
	EstimatedMinutes          int
	CompletedBeforeDayMinutes int // Prior-day completed planned minutes only.
	HasCompletedFrame         bool
	Completed                 bool
	ActualMinutes             *int
	CanComplete               bool
	CanReopen                 bool
	CanAddToToday             bool
	CanAddActionItem          bool
	CanRemoveFromToday        bool
	EstimateEditable          bool
	CreatedAt                 time.Time
}

type ScheduleOccurrence struct {
	RuleID         string
	OccurrenceDate time.Time // Stable source key; never replaced with a virtual occurrence ID.
	Title          string
	Description    string
	ProjectID      string
	StartAt        time.Time
	EndAt          time.Time
	AllDay         bool
	Completed      bool
	Hidden         bool
	Revision       int32
	CanComplete    bool
	CanReopen      bool
	CanHide        bool
	CanRestore     bool
}

type Interval struct {
	Start time.Time
	End   time.Time
}

type ProjectAllocation struct {
	ProjectID string
	StartAt   time.Time
	EndAt     time.Time
}

type HistoryFrame struct {
	ID             string
	Kind           string
	TaskID         string
	ActionItemID   string
	PlannedMinutes int
	CompletedAt    time.Time
}
