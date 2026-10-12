package application

import (
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

func NewProjectUseCases(store ProjectStore, uow projectusecase.UnitOfWork, logger telemetry.Logger, readUOWs ...projectusecase.UnitOfWork) *projectusecase.UseCases {
	return projectusecase.NewUseCases(store.Projects, uow, logger, readUOWs...)
}
