package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type CreateBlueprintInput struct {
	Title       string
	Description string
	Location    string
	ProjectID   string
	AssigneeID  string
	Timezone    string
	LocalStart  time.Time
	LocalEnd    time.Time
	Frequency   string
	Interval    int
	Weekdays    []string
	EndDate     *time.Time
}

type UpdateBlueprintInput struct {
	Title       *string
	Description *string
	Location    *string
	ProjectID   *string
	AssigneeID  *string
	Timezone    *string
	LocalStart  *time.Time
	LocalEnd    *time.Time
	Frequency   *string
	Interval    *int
	Weekdays    *[]string
	EndDate     **time.Time
}

type UpdateScheduleInput struct {
	Title       *string
	Description *string
	Location    *string
	ProjectID   *string
	AssigneeID  *string
	StartAt     *time.Time
	EndAt       *time.Time
}

type ProjectBlueprintLifecycle interface {
	PauseProjectBlueprints(ctx context.Context, actorID, projectID string, at time.Time) error
	ResumeProjectBlueprints(ctx context.Context, actorID, projectID string, at time.Time) error
}

type BackfillResult struct {
	ProcessedBlueprints int
	GeneratedSchedules  int
}

type ScheduleBackfill interface {
	Execute(ctx context.Context, actorID string, cutoff time.Time, report func(processedBlueprints, generatedSchedules int) error) (BackfillResult, error)
}

// BlueprintRepository contains persistence operations for recurrence
// definitions and concrete occurrence snapshots. Implementations are bound to
// the caller's transaction through the context's Unit of Work.
type BlueprintRepository interface {
	CreateBlueprint(ctx context.Context, actorID string, definition dao.BlueprintRevision, firstOccurrence *dao.ScheduleOccurrence) (dao.Blueprint, error)
	GetBlueprint(ctx context.Context, actorID, blueprintID string) (dao.Blueprint, error)
	GetEffectiveRevisionForDate(ctx context.Context, actorID, blueprintID, occurrenceDate string, capability shared.Capability) (dao.BlueprintRevision, error)
	GetBlueprintRevisionForMutation(ctx context.Context, actorID, blueprintID, occurrenceDate string, capability shared.Capability) (int32, error)
	ListBlueprintRevisions(ctx context.Context, actorID, blueprintID string, limit int, beforeRevision int32) ([]dao.BlueprintRevision, error)
	ListOccurrenceRevisions(ctx context.Context, actorID, blueprintID, occurrenceDate string, limit int, beforeRevision int32) ([]dao.ScheduleOccurrenceRevision, error)
	ListBlueprintRevisionsForRange(ctx context.Context, actorID string, from, to time.Time) ([]dao.BlueprintRevision, error)
	ListPersistedOccurrences(ctx context.Context, actorID string, from, to time.Time) ([]dao.ScheduleOccurrence, error)
	ListOccurrenceOrigins(ctx context.Context, actorID string, blueprintIDs []string, from, to time.Time) ([]dao.ScheduleOccurrenceOrigin, error)
	ListPauseIntervals(ctx context.Context, actorID, blueprintID string) ([]dao.ProjectPauseInterval, error)
	PauseProjectBlueprints(ctx context.Context, actorID, projectID string, at time.Time) error
	ResumeProjectBlueprints(ctx context.Context, actorID, projectID string, at time.Time) error
	GetPersistedOccurrence(ctx context.Context, actorID, blueprintID, occurrenceDate string) (dao.ScheduleOccurrence, error)
	GetPersistedOccurrenceForHistory(ctx context.Context, actorID, blueprintID, occurrenceDate string) (dao.ScheduleOccurrence, error)
	GetPersistedOccurrenceForMutation(ctx context.Context, actorID, blueprintID, occurrenceDate string, capability shared.Capability) (dao.ScheduleOccurrence, error)
	LockOccurrenceForMutation(ctx context.Context, actorID, blueprintID, occurrenceDate string, capability shared.Capability) error
	MaterializeOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence, capability shared.Capability) (dao.ScheduleOccurrence, error)
	UpdatePersistedOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence, expectedRevision int32, capability shared.Capability) (dao.ScheduleOccurrence, error)
	PersistOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence, expectedRevision *int32, capability shared.Capability) (dao.ScheduleOccurrence, error)
	PublishRevision(ctx context.Context, actorID, blueprintID string, revision dao.BlueprintRevision, expectedRevision int32, capability shared.Capability) (dao.Blueprint, error)
	DeleteFollowing(ctx context.Context, actorID, blueprintID, occurrenceDate, revisionID string, expectedRevision int32) error
	ListOccurrenceProjectIDsForFollowing(ctx context.Context, blueprintID, occurrenceDate string) ([]string, error)
	LockBlueprintForRevisionMutation(ctx context.Context, actorID, blueprintID, occurrenceDate string, capability shared.Capability) error
	IsAssigneeEligible(ctx context.Context, projectID, ownerID, assigneeID string) (bool, error)
	ListBlueprintEligibleAssignees(ctx context.Context, actorID, blueprintID string) ([]dao.Assignee, error)
	ListRevisionsForBackfill(ctx context.Context) ([]dao.BlueprintRevision, error)
	ListRevisionsForBlueprintBackfill(ctx context.Context, blueprintID string) ([]dao.BlueprintRevision, error)
	LockBlueprintForBackfill(ctx context.Context, blueprintID string) error
	GetRevisionBackfillCheckpoint(ctx context.Context, revisionID string) (*string, error)
	AdvanceRevisionBackfillCheckpoint(ctx context.Context, revisionID, throughDate string) error
	BackfillOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence) (bool, error)
}
