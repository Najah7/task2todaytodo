package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
)

type ListRuleRevisionsUseCase struct{ uow UOW }

func NewListRuleRevisionsUseCase(uow UOW) *ListRuleRevisionsUseCase {
	return &ListRuleRevisionsUseCase{uow: uow}
}

func (uc *ListRuleRevisionsUseCase) ExecutePage(ctx context.Context, actorID, ruleID string, size int, beforeRevision int32) (CursorPage[dao.RuleRevision], error) {
	if size < 1 || size > pagination.MaxPageSize {
		return CursorPage[dao.RuleRevision]{}, ErrInvalidSchedulePage
	}
	rows, err := uc.Execute(ctx, actorID, ruleID, size+1, beforeRevision)
	if err != nil {
		return CursorPage[dao.RuleRevision]{}, err
	}
	page := CursorPage[dao.RuleRevision]{Items: rows}
	if len(rows) > size {
		page.Items = rows[:size]
		if len(page.Items) > 0 {
			page.Next = &CursorAnchor{Revision: page.Items[len(page.Items)-1].Revision}
		}
	}
	return page, nil
}

func (uc *ListRuleRevisionsUseCase) Execute(ctx context.Context, actorID, ruleID string, limit int, beforeRevision int32) ([]dao.RuleRevision, error) {
	var result []dao.RuleRevision
	err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		result, err = repos.Rules().ListRuleRevisions(ctx, actorID, ruleID, limit, beforeRevision)
		return err
	})
	return result, err
}
