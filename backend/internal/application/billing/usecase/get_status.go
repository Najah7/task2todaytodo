package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

type billingAccountReader interface {
	GetAccount(context.Context, string) (dao.Account, error)
}

type GetStatusUseCase struct {
	repo    billingAccountReader
	enabled bool
}

func NewGetStatusUseCase(repo billingAccountReader, enabled bool) *GetStatusUseCase {
	return &GetStatusUseCase{repo: repo, enabled: enabled}
}

func (u *GetStatusUseCase) Execute(ctx context.Context, userID string) (dao.State, error) {
	if !u.enabled {
		return dao.State{}, ErrDisabled
	}
	account, err := u.repo.GetAccount(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return dao.State{Plan: "free", Status: "none"}, nil
	}
	return account.State, err
}
