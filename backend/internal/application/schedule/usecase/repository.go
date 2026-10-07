package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

var (
	ErrScheduleProjectNotFound     = errors.New("project not found")
	ErrScheduleAssigneeNotEligible = errors.New("schedule assignee is not eligible")
	ErrScheduleScopeInvalid        = errors.New("schedule scope is invalid")
	ErrOccurrenceDateRequired      = errors.New("occurrence date is required for recurring series")
	ErrOccurrenceNotFound          = errors.New("recurrence occurrence not found")
	ErrOccurrenceInactive          = errors.New("recurrence occurrence is inactive")
	ErrOccurrenceCompleted         = errors.New("completed occurrence cannot be skipped")
	ErrOccurrenceRuleMismatch      = errors.New("date does not match active recurrence rule")
	ErrRescheduleDateMismatch      = errors.New("future reschedule must keep the occurrence's local date")
	ErrPermissionDenied            = errors.New("permission denied")
	ErrInvalidSchedulePage         = errors.New("invalid schedule page")
	ErrScheduleTagNotFound         = errors.New("schedule tag not found")
)

type UserTimezoneReader interface {
	GetTimezone(ctx context.Context, userID string) (string, error)
}

type ScheduleRepository interface {
	LockProjectForScheduleMutation(ctx context.Context, projectID domain.ProjectID) error
	LockSeriesProjectForMutation(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID, capability shared.Capability) error
	ListScheduleRevisionsByActor(ctx context.Context, actorID domain.UserID, id domain.ScheduleID, limit int, anchor *CursorAnchor) ([]dao.ScheduleRevision, error)
	GetByUserIDWithPermission(ctx context.Context, actorID domain.UserID, id domain.ScheduleID, capability shared.Capability) (dao.Schedule, error)
	ListByAssigneeUserID(ctx context.Context, actorID domain.UserID) ([]dao.Schedule, error)
	ListByProjectAndUserID(ctx context.Context, actorID domain.UserID, projectID domain.ProjectID) ([]dao.Schedule, error)
	ListOccurrencesForPermission(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID, capability shared.Capability) ([]dao.Schedule, error)
	ListActiveSeriesByAssigneeUserID(ctx context.Context, userID domain.UserID) ([]dao.Schedule, error)
	CreateByUserID(ctx context.Context, actorID domain.UserID, schedule domain.Schedule) (dao.Schedule, error)
	UpdateByUserID(ctx context.Context, actorID domain.UserID, schedule domain.Schedule, capability shared.Capability) (dao.Schedule, error)
	SetCompletedByUserID(ctx context.Context, actorID domain.UserID, id domain.ScheduleID, completed bool, capability shared.Capability) error
	DeleteByUserID(ctx context.Context, actorID domain.UserID, id domain.ScheduleID, capability shared.Capability) error
	TombstoneByUserID(ctx context.Context, actorID domain.UserID, id domain.ScheduleID, capability shared.Capability) error
	DeleteUneditedFutureBySeries(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID, fromAt time.Time) error
	SetRecurrenceByUserID(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID, anchor time.Time, intervalWeeks int, frequencies []dao.Frequency) error
	StopRecurrenceByUserID(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID) error
	UpdateSeriesTemplateByUserID(ctx context.Context, actorID domain.UserID, seriesID, snapshotID domain.ScheduleID, schedule domain.Schedule) (dao.Schedule, error)
	UpsertOverrideByUserID(ctx context.Context, actorID domain.UserID, schedule domain.Schedule) (string, error)
	ListSkippedOccurrences(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID) ([]int64, error)
	ListSkippedOccurrencesForPermission(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID, capability shared.Capability) ([]int64, error)
	SetSkippedOccurrence(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID, occurrenceDate time.Time, skipped bool) error
	SetAssigneeByUserID(ctx context.Context, actorID domain.UserID, id domain.ScheduleID, assigneeID domain.UserID) error
	SetProjectByUserID(ctx context.Context, actorID domain.UserID, id domain.ScheduleID, projectID *domain.ProjectID) error
	ListEligibleAssignees(ctx context.Context, actorID domain.UserID, id domain.ScheduleID) ([]dao.Assignee, error)
	ListTags(ctx context.Context, actorID domain.UserID, id domain.ScheduleID) ([]dao.ScheduleTag, error)
	ListTagsForSchedules(ctx context.Context, actorID domain.UserID, ids []domain.ScheduleID) (map[string][]dao.ScheduleTag, error)
	AddTag(ctx context.Context, actorID domain.UserID, id domain.ScheduleID, tagID string) error
	RemoveTag(ctx context.Context, actorID domain.UserID, id domain.ScheduleID, tagID string) error
	DeleteProjectSchedulesByActor(ctx context.Context, actorID domain.UserID, projectID domain.ProjectID) error
	ReassignProjectMemberSchedules(ctx context.Context, actorID domain.UserID, projectID domain.ProjectID, memberID domain.UserID) error
}

type Repositories interface {
	Schedules() ScheduleRepository
}

type UOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}
