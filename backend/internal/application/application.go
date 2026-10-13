package application

import (
	"context"
	"fmt"

	"github.com/Najah7/task2todaytodo/internal/settings"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type Application struct {
	Store   Store
	UseCase UseCase
	UOW     UOW
}

type IDIssuer interface {
	NewULID() string
	NewUUID() string
}

func New(database settings.DatabaseSettings, id IDIssuer, logger telemetry.Logger, emailSettings ...AuthEmailSettings) (*Application, func(context.Context) error, error) {
	return NewWithBilling(database, id, logger, settings.BillingSettings{}, emailSettings...)
}

func NewWithBilling(database settings.DatabaseSettings, id IDIssuer, logger telemetry.Logger, billingSettings settings.BillingSettings, emailSettings ...AuthEmailSettings) (*Application, func(context.Context) error, error) {
	ctx := context.Background()
	pool, err := newPool(ctx, database)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize application database: %w", err)
	}

	store := NewStore(pool)
	uow := NewUOWWithID(pool, store.Auth, store.Project, store.Task, store.Schedule, store.Allocation, id.NewULID)
	uow.Billing = NewBillingUOW(pool, store.Billing)
	uc, stopUseCases := NewUseCaseWithBilling(store, uow, id, logger, billingSettings, emailSettings...)
	closeApp := func(ctx context.Context) error {
		if err := stopUseCases(ctx); err != nil {
			return fmt.Errorf("shutdown admin operations: %w", err)
		}
		pool.Close()
		return nil
	}

	return &Application{
		Store: store, UseCase: uc, UOW: uow,
	}, closeApp, nil
}
