package application

import (
	"context"
	"fmt"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/settings"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type Application struct {
	Store   Store
	UseCase UseCase
	UOW     UOW
}

func New(database settings.DatabaseSettings, ids shared.ID, logger telemetry.Logger) (*Application, func(context.Context) error, error) {
	ctx := context.Background()
	pool, err := newPool(ctx, database)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize application database: %w", err)
	}

	store := NewStore(pool)
	uow := NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule)
	uc, stopUseCases := NewUseCase(store.Auth, store.Project, store.Task, store.Admin, uow.Project, uow.Task, uow.Schedule, ids, logger)
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
