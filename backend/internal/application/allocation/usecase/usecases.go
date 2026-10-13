package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
)

type UnitOfWork interface {
	Do(context.Context, func(context.Context, Repository) error) error
}

// Repository exposes transaction-bound persistence primitives. The
// application layer resolves recurrence, ownership policy, cutovers, and
// overlap validation before it calls these methods.
type Repository interface {
	LockProjectsForMutation(context.Context, string, []string) error
	LockUser(context.Context, string) error
	GetProfile(context.Context, string) (dao.Profile, error)
	GetProjectStatus(context.Context, string, string) (string, error)
	ListProjects(context.Context, string) ([]dao.Project, error)
	ListRules(context.Context, string) ([]dao.Rule, error)
	ListRuleRevisionsForProjection(context.Context, string) ([]dao.Rule, error)
	ListAllocations(context.Context, string, time.Time, time.Time) ([]dao.Allocation, error)
	ListActiveTimeExceptions(context.Context, string, time.Time, time.Time) ([]dao.ActiveTimeException, error)
	ListExceptionOrigins(context.Context, string, time.Time, time.Time) (map[string]bool, error)
	ListPauseIntervals(context.Context, string) ([]dao.PauseInterval, error)
	ListPlans(context.Context, string) ([]dao.PlanItem, error)
	SetPlan(context.Context, string, string, int) error
	DeletePlan(context.Context, string, string) error
	CreateRule(context.Context, string, string, string) error
	CreateRuleRevision(context.Context, string, string, string, string, int32, time.Time, time.Time, time.Time, domain.Slot) error
	SupersedeFutureRevisions(context.Context, string, time.Time) error
	StopRule(context.Context, string, string, time.Time) error
	StopFollowingRule(context.Context, string, string, time.Time, time.Time) error
	SetRulePause(context.Context, string, string, string, time.Time) error
	StartPauseInterval(context.Context, string, string, time.Time) error
	ClosePauseInterval(context.Context, string, string, time.Time) error
	DeleteFutureRows(context.Context, string, time.Time, string, bool) error
	DeleteFollowingOrigins(context.Context, string, string, time.Time, time.Time) error
	DeleteFutureOrdinaryOriginsAfter(context.Context, string, string, time.Time, time.Time) error
	InsertOccurrence(context.Context, string, domain.Occurrence) (dao.Allocation, error)
	RefreshOccurrence(context.Context, string, domain.Occurrence, time.Time) error
	MaterializeOccurrence(context.Context, string, domain.Occurrence, time.Time) (dao.Allocation, error)
	GetOccurrence(context.Context, string, string, time.Time) (dao.Allocation, error)
	ReadOccurrence(context.Context, string, string, time.Time) (dao.Allocation, error)
	SaveOccurrenceException(context.Context, string, domain.Occurrence, int32, bool) error
	SaveActiveTimeException(context.Context, string, int32, domain.ActiveTimeException) (dao.ActiveTimeException, error)
	DeleteActiveTimeException(context.Context, string, time.Time, int32) error
	FindAllocationConflict(context.Context, string, time.Time, time.Time, string, time.Time) (dao.Allocation, error)
	ListUserIDs(context.Context) ([]string, error)
}

type UseCases struct {
	GetWeek                   *GetWeekUseCase
	GetRules                  *GetRulesUseCase
	PutRules                  *PutRulesUseCase
	GetPlans                  *GetPlansUseCase
	PutPlans                  *PutPlansUseCase
	Create                    *CreateAllocationUseCase
	Update                    *UpdateOccurrenceUseCase
	Delete                    *DeleteOccurrenceUseCase
	PutActiveTimeException    *PutActiveTimeExceptionUseCase
	DeleteActiveTimeException *DeleteActiveTimeExceptionUseCase
}

func NewUseCases(repo Repository, uow UnitOfWork, issueID func() string, now func() time.Time, readUOWs ...UnitOfWork) UseCases {
	if now == nil {
		now = time.Now
	}
	readUOW := uow
	if len(readUOWs) > 0 && readUOWs[0] != nil {
		readUOW = readUOWs[0]
	}
	return UseCases{
		GetWeek:  NewGetWeekUseCase(repo),
		GetRules: NewGetRulesUseCase(readUOW), PutRules: NewPutRulesUseCase(repo, uow, issueID, now),
		GetPlans: NewGetPlansUseCase(readUOW), PutPlans: NewPutPlansUseCase(repo, uow),
		Create:                    NewCreateAllocationUseCase(repo, uow, issueID, now),
		Update:                    NewUpdateOccurrenceUseCase(repo, uow, issueID, now),
		Delete:                    NewDeleteOccurrenceUseCase(repo, uow, issueID, now),
		PutActiveTimeException:    NewPutActiveTimeExceptionUseCase(repo, uow, now),
		DeleteActiveTimeException: NewDeleteActiveTimeExceptionUseCase(repo, uow, now),
	}
}
