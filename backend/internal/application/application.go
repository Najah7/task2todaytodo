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

func New(database settings.DatabaseSettings, id IDIssuer, logger telemetry.Logger) (*Application, func(context.Context) error, error) {
	ctx := context.Background()
	pool, err := newPool(ctx, database)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize application database: %w", err)
	}

	store := NewStore(pool)
	uow := NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule)
	uc, stopUseCases := NewUseCase(store, uow, id, logger)
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
