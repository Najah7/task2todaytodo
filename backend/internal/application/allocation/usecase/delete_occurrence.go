package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

type DeleteOccurrenceUseCase struct {
	repo    Repository
	uow     UnitOfWork
	issueID func() string
	now     func() time.Time
	access  shared.PlanAccessReader
}

func NewDeleteOccurrenceUseCase(repo Repository, uow UnitOfWork, issueID func() string, now func() time.Time, access ...shared.PlanAccessReader) *DeleteOccurrenceUseCase {
	uc := &DeleteOccurrenceUseCase{repo: repo, uow: uow, issueID: issueID, now: now}
	if len(access) > 0 {
		uc.access = access[0]
	}
	return uc
}

func (uc *DeleteOccurrenceUseCase) Execute(ctx context.Context, userID, ruleID string, occurrenceDate time.Time, expected int32, scope string, ruleRevision int32) error {
	if err := requirePaid(ctx, uc.access, userID); err != nil {
		return err
	}
	if userID == "" || ruleID == "" || expected < 0 || (scope != "current" && scope != "following") {
		return ErrInvalidRequest
	}
	occurrenceDate = calendar.DateOnly(occurrenceDate)
	now := uc.now().UTC()
	return uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		current, _, err := resolveOccurrence(ctx, repo, userID, ruleID, occurrenceDate)
		if err != nil {
			return err
		}
		if err := repo.LockProjectsForMutation(ctx, userID, []string{current.ProjectID}); err != nil {
			return err
		}
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		current, virtual, err := resolveOccurrence(ctx, repo, userID, ruleID, occurrenceDate)
		if err != nil {
			return err
		}
		if virtual {
			if expected != 0 {
				return ErrPreconditionFailed
			}
			current, err = repo.MaterializeOccurrence(ctx, userID, domain.Occurrence{ID: uc.issueID(), RuleID: current.RuleID, SourceRevisionID: current.SourceRevisionID, ProjectID: current.ProjectID, OccurrenceDate: current.OccurrenceDate, StartAt: current.StartAt, EndAt: current.EndAt, Timezone: current.Timezone}, now)
			if err != nil {
				return err
			}
		} else if current.Revision != expected {
			return ErrPreconditionFailed
		}
		persistedRevision := current.Revision
		if scope == "current" {
			deleted := domainOccurrence(current)
			return repo.SaveOccurrenceException(ctx, userID, deleted, persistedRevision, true)
		}
		rules, err := repo.ListRules(ctx, userID)
		if err != nil {
			return err
		}
		var found bool
		for _, rule := range rules {
			if rule.ID == ruleID && rule.Revision == ruleRevision {
				found = true
				break
			}
		}
		if !found {
			return ErrPreconditionFailed
		}
		if err := repo.DeleteFollowingOrigins(ctx, userID, ruleID, current.OccurrenceDate, now); err != nil {
			return err
		}
		if err := repo.StopFollowingRule(ctx, userID, ruleID, now, current.OccurrenceDate.AddDate(0, 0, -1)); err != nil {
			return err
		}
		return nil
	})
}

func domainOccurrence(value dao.Allocation) domain.Occurrence {
	return domain.Occurrence{ID: value.ID, RuleID: value.RuleID, SourceRevisionID: value.SourceRevisionID, ProjectID: value.ProjectID, OccurrenceDate: value.OccurrenceDate, StartAt: value.StartAt, EndAt: value.EndAt, Timezone: value.Timezone}
}
