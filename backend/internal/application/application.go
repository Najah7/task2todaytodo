package application

import (
	"context"
	"fmt"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type Application struct {
	Store   Store
	UseCase UseCase
	UOW     UOW
}

func New(database DatabaseConfig, ids shared.ID, logger logging.Logger) (*Application, error) {
	ctx := context.Background()
	pool, err := newPool(ctx, database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize application database: %w", err)
	}

	store := NewStore(pool)
	uow := NewUOW(pool, store.Auth, store.Task)
	uc := NewUseCase(store.Auth, store.Task, uow.Task, ids, logger)

	return &Application{
		Store:   store,
		UseCase: uc,
		UOW:     uow,
	}, nil
}
