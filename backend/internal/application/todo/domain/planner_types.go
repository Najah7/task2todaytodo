package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput     = errors.New("invalid todo planner input")
	ErrInvalidInterval  = errors.New("interval end must be after start")
	ErrInvalidGap       = errors.New("gap must be from 0 through 1440 minutes")
	ErrInvalidCandidate = errors.New("candidate estimate must be positive")
	ErrInvalidReflow    = errors.New("invalid todo reflow order")
	ErrDuplicateID      = errors.New("duplicate todo planner id")
)

// Interval is a half-open interval of resolved instants. Callers resolve local
// dates, timezones, and daylight-saving transitions before invoking the planner.
type Interval struct {
	Start time.Time
	End   time.Time
}

type ProjectAllocation struct {
	ProjectID string
	Interval  Interval
}

type Schedule struct {
	ID             string
	RuleID         string
	OccurrenceDate time.Time
	Interval       Interval
	AllDay         bool
	Completed      bool
	Hidden         bool
}

type Candidate struct {
	Kind                      FrameKind
	ID                        string
	TaskID                    string
	ProjectID                 string
	PriorityWeight            int
	DueAt                     *time.Time // Already resolved from ActionItem, then Task.
	EstimatedMinutes          int
	CompletedBeforeDayMinutes int // Historical completed planned minutes; excludes today's frames.
	HasCompletedFrame         bool
	TaskStatus                string
	CreatedAt                 time.Time
}

type FrameKind string

const (
	FrameActionItem FrameKind = "action_item"
	FrameTask       FrameKind = "task"
	FrameSchedule   FrameKind = "schedule"
)

func (kind FrameKind) IsWork() bool { return kind == FrameActionItem || kind == FrameTask }

func sourceKey(kind FrameKind, id string) string { return string(kind) + "\x00" + id }

// Frame describes either an action-item work segment or a fixed schedule.
// Unscheduled work has nil Start/End and retains positive PlannedMinutes.
type Frame struct {
	ID             string // Existing frame ID; empty for a newly generated frame.
	Kind           FrameKind
	SourceID       string
	RuleID         string
	OccurrenceDate time.Time
	PlannedMinutes int
	Start          *time.Time
	End            *time.Time
	Completed      bool
	Hidden         bool
	Position       int
}

type Input struct {
	Day               Interval
	ActiveTime        []Interval
	Allocations       []ProjectAllocation
	IgnoreAllocations bool
	Schedules         []Schedule
	CompletedFrames   []Frame
	Candidates        []Candidate
	GapMinutes        int
	Today             time.Time // Date in caller's timezone; used for due urgency.
	Location          *time.Location
}

type Plan struct {
	Frames                   []Frame
	PreservedCompletedFrames []Frame
}

// Generate creates a deterministic day plan, placing each whole work item in
// one contiguous active-time window allowed by its Project allocation.
