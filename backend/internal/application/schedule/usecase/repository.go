package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

var (
	ErrScheduleProjectNotFound     = errors.New("project not found")
	ErrScheduleProjectChanged      = errors.New("schedule project association changed")
	ErrProjectLifecycleUnavailable = errors.New("project lifecycle service is unavailable")
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
)

type UserTimezoneReader interface {
	GetTimezone(ctx context.Context, userID string) (string, error)
}

type TimezoneValidator interface {
	TimezoneExists(ctx context.Context, timezone string) (bool, error)
}

type ScheduleRepository interface {
	CheckProjectPermission(ctx context.Context, actorID domain.UserID, projectID domain.ProjectID, capability shared.Capability) (bool, error)
	LockProjectForScheduleMutation(ctx context.Context, projectID domain.ProjectID) error
	DeleteProjectSchedulesByActor(ctx context.Context, actorID domain.UserID, projectID domain.ProjectID) error
	ReassignProjectMemberSchedules(ctx context.Context, actorID domain.UserID, projectID domain.ProjectID, memberID domain.UserID) error
}

type Repositories interface {
	ProjectLifecycle() shared.ProjectWorkLifecycle
	Schedules() ScheduleRepository
	Rules() RuleRepository
}

type UOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
	DoWithTodoImpact(ctx context.Context, scope TodoImpactScope, fn func(context.Context, Repositories) error) error
}

// TodoImpactScope narrowly identifies schedule reservations that may affect
// generated local-day plans. The root resolves old assignees and occurrences
// inside the transaction; callers may provide the proposed assignee.
type TodoImpactScope struct {
	ActorID        string
	RuleID         string
	OccurrenceDate string
	Following      bool
	ProjectID      string
	AssigneeID     string
}

func doWithTodoImpact(ctx context.Context, uow UOW, scope TodoImpactScope, fn func(context.Context, Repositories) error) error {
	return uow.DoWithTodoImpact(ctx, scope, fn)
}
