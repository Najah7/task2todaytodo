package application

import (
	"context"
	"errors"
	"time"

	billingrepository "github.com/Najah7/task2todaytodo/internal/application/billing/repository"
	billingusecase "github.com/Najah7/task2todaytodo/internal/application/billing/usecase"
	projectdomain "github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectrepository "github.com/Najah7/task2todaytodo/internal/application/project/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/jackc/pgx/v5"
)

type projectOwnerMemberReader interface {
	GetProjectOwnerAndMemberCount(context.Context, projectdomain.ProjectID) (string, int64, error)
}

// projectWriteAccessReader composes project membership facts and raw billing
// snapshots from the caller's transaction, then applies the shared entitlement
// rule once for both actor and owner.
type projectWriteAccessReader struct {
	projects projectOwnerMemberReader
	billing  *billingrepository.Repository
	enabled  bool
	now      func() time.Time
}

func newProjectWriteAccessReader(projects *projectrepository.ProjectRepository, billing *billingrepository.Repository, enabled bool) *projectWriteAccessReader {
	return &projectWriteAccessReader{projects: projects, billing: billing, enabled: enabled, now: time.Now}
}

func billingWithTx(repository *billingrepository.Repository, tx pgx.Tx) *billingrepository.Repository {
	if repository == nil {
		return nil
	}
	return repository.WithTx(tx)
}

type projectRepositoryWithWriteAccess struct {
	projectusecase.Repository
	shared.ProjectWriteAccessReader
	shared.PlanAccessReader
}

type projectInvitationRepositoryWithWriteAccess struct {
	*projectrepository.ProjectRepository
	shared.ProjectWriteAccessReader
}

func (r projectWriteAccessReader) GetPlanAccess(ctx context.Context, userID string) (shared.PlanAccess, error) {
	if !r.enabled {
		return shared.PlanAccessFree, nil
	}
	return r.planAccess(ctx, userID)
}

func (r projectWriteAccessReader) GetProjectWriteAccess(ctx context.Context, actorID, projectID string) (shared.ProjectWriteAccess, error) {
	ownerID, memberCount, err := r.projects.GetProjectOwnerAndMemberCount(ctx, projectdomain.ProjectID(projectID))
	if err != nil {
		return shared.ProjectWriteAccess{}, err
	}
	result := shared.ProjectWriteAccess{OwnerUserID: ownerID, MemberCount: memberCount}
	if !r.enabled {
		return result, nil
	}
	result.ActorPlanAccess, err = r.planAccess(ctx, actorID)
	if err != nil {
		return shared.ProjectWriteAccess{}, err
	}
	if actorID == ownerID {
		result.OwnerPlanAccess = result.ActorPlanAccess
		return result, nil
	}
	result.OwnerPlanAccess, err = r.planAccess(ctx, ownerID)
	if err != nil {
		return shared.ProjectWriteAccess{}, err
	}
	return result, nil
}

func (r projectWriteAccessReader) planAccess(ctx context.Context, userID string) (shared.PlanAccess, error) {
	if r.billing == nil {
		return shared.PlanAccessFree, nil
	}
	account, err := r.billing.GetAccount(ctx, userID)
	if errors.Is(err, billingusecase.ErrNotFound) {
		return shared.PlanAccessFree, nil
	}
	if err != nil {
		return shared.PlanAccessFree, err
	}
	return shared.EffectivePlanAccess(account.State.Plan, account.State.Status, account.State.CurrentPeriodEnd, r.now()), nil
}
