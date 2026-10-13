package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type GetRulesUseCase struct {
	uow    UnitOfWork
	access shared.PlanAccessReader
}

func NewGetRulesUseCase(uow UnitOfWork, access ...shared.PlanAccessReader) *GetRulesUseCase {
	uc := &GetRulesUseCase{uow: uow}
	if len(access) > 0 {
		uc.access = access[0]
	}
	return uc
}

func (uc *GetRulesUseCase) Execute(ctx context.Context, userID string) (dao.RuleCollection, error) {
	if err := requirePaid(ctx, uc.access, userID); err != nil {
		return dao.RuleCollection{}, err
	}
	var result dao.RuleCollection
	err := uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		profile, err := repo.GetProfile(ctx, userID)
		if err != nil {
			return err
		}
		items, err := weeklyRules(ctx, repo, userID)
		if err != nil {
			return err
		}
		result = dao.RuleCollection{Revision: rulesRevision(userID, profile.Timezone, items), Timezone: profile.Timezone, Items: items}
		return nil
	})
	return result, err
}

func weeklyRules(ctx context.Context, repo Repository, userID string) ([]dao.Rule, error) {
	rules, err := repo.ListRules(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]dao.Rule, 0, len(rules))
	for _, rule := range rules {
		if rule.Frequency == "weekly" {
			items = append(items, rule)
		}
	}
	return items, nil
}
