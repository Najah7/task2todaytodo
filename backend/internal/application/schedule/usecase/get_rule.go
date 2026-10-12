package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
)

type GetRuleUseCase struct{ uow UOW }

func NewGetRuleUseCase(uow UOW) *GetRuleUseCase { return &GetRuleUseCase{uow: uow} }

func (uc *GetRuleUseCase) Execute(ctx context.Context, actorID, ruleID string) (dao.Rule, error) {
	var result dao.Rule
	err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		result, err = repos.Rules().GetRule(ctx, actorID, ruleID)
		return err
	})
	return result, err
}
