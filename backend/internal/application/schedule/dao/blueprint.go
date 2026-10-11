package dao

import "time"

type Blueprint struct {
	ID                   string
	UserID               string
	Revision             int32
	EffectiveFrom        string
	RecurrenceAnchorDate string
	LocalStart           string
	LocalEnd             string
	AllDay               bool
	Timezone             string
	Frequency            string
	Interval             int
	Weekdays             []string
	EndDate              *string
	CutoffAfter          *string
	Title                string
	Description          string
	Location             string
	ProjectID            string
	AssigneeID           string
	Deleted              bool
	CreatedAt            int64
}

type BlueprintRevision struct {
	Blueprint
	RevisionID     string
	Superseded     bool
	EffectiveUntil *string
	PauseIntervals []ProjectPauseInterval
	ChangedBy      string
	ChangedAt      int64
}

type ProjectPauseInterval struct {
	ProjectID string
	PausedAt  time.Time
	ResumedAt *time.Time
}

type ScheduleOccurrenceRevision struct {
	Revision         int32
	SourceRevisionID string
	UserID           string
	OccurrenceDate   string
	StartAt          int64
	EndAt            int64
	Title            string
	Description      string
	Location         string
	ProjectID        string
	AssigneeID       string
	Timezone         string
	AllDay           bool
	Completed        bool
	SkippedAt        *int64
	DeletedAt        *int64
	IsOverride       bool
	ChangedBy        string
	ChangedAt        int64
}

type ScheduleOccurrenceOrigin struct {
	BlueprintID    string
	OccurrenceDate string
}

// ScheduleOccurrence represents a persisted row or virtual projection. A nil
// ID means no occurrence row exists yet; it is never a magic identifier.
type ScheduleOccurrence struct {
	ID                 *string
	BlueprintID        string
	OccurrenceDate     string
	SourceRevisionID   string
	UserID             string
	ProjectID          string
	AssigneeID         string
	Title              string
	Description        string
	Location           string
	Timezone           string
	AllDay             bool
	StartAt            int64
	EndAt              int64
	Completed          bool
	Skipped            bool
	Deleted            bool
	IsOverride         bool
	Revision           int32
	CreatedAt          int64
	UpdatedAt          int64
	CanUpdate          bool
	CanDelete          bool
	CanSkip            bool
	CanAssign          bool
	CanUpdateFollowing bool
	CanDeleteFollowing bool
}

type OccurrenceCapabilities struct {
	CanUpdate          bool
	CanDelete          bool
	CanSkip            bool
	CanAssign          bool
	CanUpdateFollowing bool
	CanDeleteFollowing bool
}
