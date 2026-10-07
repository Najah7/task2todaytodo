package application

import (
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

func NewProjectUseCases(store ProjectStore, uow projectusecase.UnitOfWork, logger logging.Logger) *projectusecase.UseCases {
	return projectusecase.NewUseCases(store.Projects, uow, logger)
}
