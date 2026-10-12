package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type CreateRuleInput struct {
	Title                   string
	Description             string
	Location                string
	ProjectID               string
	AssigneeID              string
	Timezone                string
	LocalStart              time.Time
	LocalEnd                time.Time
	AllDay                  bool
	Frequency               string
	Interval                int
	Weekdays                []string
	EndDate                 *time.Time
	ConfirmTodoRegeneration bool
}

type UpdateRuleInput struct {
	Title                   *string
	Description             *string
	Location                *string
	ProjectID               *string
	AssigneeID              *string
	Timezone                *string
	LocalStart              *time.Time
	LocalEnd                *time.Time
	AllDay                  *bool
	Frequency               *string
	Interval                *int
	Weekdays                *[]string
	EndDate                 **time.Time
	ConfirmTodoRegeneration bool
}

type UpdateScheduleInput struct {
	Title                   *string
	Description             *string
	Location                *string
	ProjectID               *string
	AssigneeID              *string
	StartAt                 *time.Time
	EndAt                   *time.Time
	AllDay                  *bool
	ConfirmTodoRegeneration bool
}

type ProjectRuleLifecycle interface {
	PauseProjectRules(ctx context.Context, actorID, projectID string, at time.Time) error
	ResumeProjectRules(ctx context.Context, actorID, projectID string, at time.Time) error
}

type ScheduleBackfill interface {
	Execute(ctx context.Context, actorID string, cutoff time.Time, report func(processedRules, generatedSchedules int) error) (processedRules, generatedSchedules int, err error)
}

// RuleRepository contains persistence operations for recurrence
// definitions and concrete occurrence snapshots. Implementations are bound to
// the caller's transaction through the context's Unit of Work.
type RuleRepository interface {
	CreateRule(ctx context.Context, actorID string, definition dao.RuleRevision, firstOccurrence *dao.ScheduleOccurrence) (dao.Rule, error)
	GetRule(ctx context.Context, actorID, ruleID string) (dao.Rule, error)
	GetEffectiveRevisionForDate(ctx context.Context, actorID, ruleID, occurrenceDate string, capability shared.Capability) (dao.RuleRevision, error)
	GetRuleRevisionForMutation(ctx context.Context, actorID, ruleID, occurrenceDate string, capability shared.Capability) (int32, error)
	ListRuleRevisions(ctx context.Context, actorID, ruleID string, limit int, beforeRevision int32) ([]dao.RuleRevision, error)
	ListOccurrenceRevisions(ctx context.Context, actorID, ruleID, occurrenceDate string, limit int, beforeRevision int32) ([]dao.ScheduleOccurrenceRevision, error)
	ListRuleRevisionsForRange(ctx context.Context, actorID string, from, to time.Time) ([]dao.RuleRevision, error)
	ListPersistedOccurrences(ctx context.Context, actorID string, from, to time.Time, includeSkipped bool) ([]dao.ScheduleOccurrence, error)
	ListOccurrenceCapabilities(ctx context.Context, actorID string, occurrences []dao.ScheduleOccurrence) (map[string]dao.OccurrenceCapabilities, error)
	ListOccurrenceOrigins(ctx context.Context, actorID string, ruleIDs []string, from, to time.Time) ([]dao.ScheduleOccurrenceOrigin, error)
	ListPauseIntervals(ctx context.Context, actorID, ruleID string) ([]dao.ProjectPauseInterval, error)
	PauseProjectRules(ctx context.Context, actorID, projectID string, at time.Time) error
	ResumeProjectRules(ctx context.Context, actorID, projectID string, at time.Time) error
	GetPersistedOccurrence(ctx context.Context, actorID, ruleID, occurrenceDate string) (dao.ScheduleOccurrence, error)
	GetPersistedOccurrenceForHistory(ctx context.Context, actorID, ruleID, occurrenceDate string) (dao.ScheduleOccurrence, error)
	GetPersistedOccurrenceForMutation(ctx context.Context, actorID, ruleID, occurrenceDate string, capability shared.Capability) (dao.ScheduleOccurrence, error)
	LockOccurrenceForMutation(ctx context.Context, actorID, ruleID, occurrenceDate string, capability shared.Capability) error
	MaterializeOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence, capability shared.Capability) (dao.ScheduleOccurrence, error)
	UpdatePersistedOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence, expectedRevision int32, capability shared.Capability) (dao.ScheduleOccurrence, error)
	ApplyRuleRevisionToOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence) error
	PublishRevision(ctx context.Context, actorID, ruleID string, revision dao.RuleRevision, expectedRevision int32, capability shared.Capability) (dao.Rule, error)
	DeleteFollowing(ctx context.Context, actorID, ruleID, occurrenceDate, revisionID string, expectedRevision int32) error
	ListOccurrenceProjectIDsForFollowing(ctx context.Context, ruleID, occurrenceDate string) ([]string, error)
	LockRuleForRevisionMutation(ctx context.Context, actorID, ruleID, occurrenceDate string, capability shared.Capability) error
	IsAssigneeEligible(ctx context.Context, projectID, ownerID, assigneeID string) (bool, error)
	ListRuleEligibleAssignees(ctx context.Context, actorID, ruleID string) ([]dao.Assignee, error)
	ListRevisionsForBackfill(ctx context.Context) ([]dao.RuleRevision, error)
	ListRevisionsForRuleBackfill(ctx context.Context, ruleID string) ([]dao.RuleRevision, error)
	LockRuleForBackfill(ctx context.Context, ruleID string) error
	GetRevisionBackfillCheckpoint(ctx context.Context, revisionID string) (*string, error)
	AdvanceRevisionBackfillCheckpoint(ctx context.Context, revisionID, throughDate string) error
	BackfillOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence) (bool, error)
}
