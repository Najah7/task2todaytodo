package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
)

type PutRulesUseCase struct {
	repo    Repository
	uow     UnitOfWork
	issueID func() string
	now     func() time.Time
}

func NewPutRulesUseCase(repo Repository, uow UnitOfWork, issueID func() string, now func() time.Time) *PutRulesUseCase {
	if now == nil {
		now = time.Now
	}
	return &PutRulesUseCase{repo: repo, uow: uow, issueID: issueID, now: now}
}

func (uc *PutRulesUseCase) Execute(ctx context.Context, userID, expectedRevision string, items []dao.Rule) (dao.RuleCollection, error) {
	if userID == "" || uc.issueID == nil || expectedRevision == "" {
		return dao.RuleCollection{}, ErrInvalidRequest
	}
	if err := validateRules(items); err != nil {
		return dao.RuleCollection{}, err
	}
	current, err := weeklyRules(ctx, uc.repo, userID)
	if err != nil {
		return dao.RuleCollection{}, err
	}
	projectIDs := make([]string, 0, len(current)+len(items))
	for _, rule := range current {
		projectIDs = append(projectIDs, rule.ProjectID)
	}
	for _, rule := range items {
		projectIDs = append(projectIDs, rule.ProjectID)
	}
	now := uc.now().UTC()
	var result dao.RuleCollection
	err = uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		if err := repo.LockProjectsForMutation(ctx, userID, projectIDs); err != nil {
			return err
		}
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		profile, err := repo.GetProfile(ctx, userID)
		if err != nil {
			return err
		}
		current, err := weeklyRules(ctx, repo, userID)
		if err != nil {
			return err
		}
		if rulesRevision(userID, profile.Timezone, current) != expectedRevision {
			return ErrPreconditionFailed
		}
		statuses := make(map[string]string)
		for _, rule := range items {
			if _, exists := statuses[rule.ProjectID]; exists {
				continue
			}
			status, err := repo.GetProjectStatus(ctx, userID, rule.ProjectID)
			if err != nil {
				return err
			}
			statuses[rule.ProjectID] = status
		}
		if err := replaceRules(ctx, repo, userID, items, current, statuses, now, uc.issueID); err != nil {
			return err
		}
		items, err := weeklyRules(ctx, repo, userID)
		if err != nil {
			return err
		}
		profile, err = repo.GetProfile(ctx, userID)
		if err != nil {
			return err
		}
		result = dao.RuleCollection{Revision: rulesRevision(userID, profile.Timezone, items), Timezone: profile.Timezone, Items: items}
		return nil
	})
	return result, err
}
