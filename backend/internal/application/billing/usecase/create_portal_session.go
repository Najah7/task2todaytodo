package usecase

import (
	"context"
)

type portalStripe interface {
	CreatePortal(context.Context, string, string) (string, error)
}

type CreatePortalSessionUseCase struct {
	repo        billingAccountReader
	stripe      portalStripe
	enabled     bool
	frontendURL string
}

func NewCreatePortalSessionUseCase(repo billingAccountReader, stripe portalStripe, enabled bool, frontendURL string) *CreatePortalSessionUseCase {
	return &CreatePortalSessionUseCase{repo: repo, stripe: stripe, enabled: enabled, frontendURL: frontendURL}
}

func (u *CreatePortalSessionUseCase) Execute(ctx context.Context, userID string) (string, error) {
	if !u.enabled {
		return "", ErrDisabled
	}
	account, err := u.repo.GetAccount(ctx, userID)
	if err != nil {
		return "", err
	}
	if account.CustomerID == "" {
		return "", ErrNotFound
	}
	return u.stripe.CreatePortal(ctx, account.CustomerID, u.frontendURL+"/billing")
}
