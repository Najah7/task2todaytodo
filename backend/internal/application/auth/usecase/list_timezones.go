package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListTimezonesUseCase struct {
	repo   TimezoneRepository
	logger logging.Logger
}

func NewListTimezonesUseCase(repo TimezoneRepository, logger logging.Logger) *ListTimezonesUseCase {
	return &ListTimezonesUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *ListTimezonesUseCase) Execute(ctx context.Context) (timezones []string, err error) {
	defer func() { logUnexpectedFailure(uc.logger, ctx, "ListTimezonesUseCase.Execute", err) }()
	return uc.repo.ListTimezones(ctx)
}

type TimezoneUseCases struct {
	List *ListTimezonesUseCase
}
