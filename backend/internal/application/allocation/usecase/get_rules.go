package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
)

type GetRulesUseCase struct{ uow UnitOfWork }

func NewGetRulesUseCase(uow UnitOfWork) *GetRulesUseCase { return &GetRulesUseCase{uow: uow} }

func (uc *GetRulesUseCase) Execute(ctx context.Context, userID string) (dao.RuleCollection, error) {
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
