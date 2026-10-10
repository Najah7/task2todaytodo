package application

import (
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

func NewProjectUseCases(store ProjectStore, uow projectusecase.UnitOfWork, logger logging.Logger, progress ...projectusecase.ProjectProgressReader) *projectusecase.UseCases {
	if len(progress) > 0 {
		return projectusecase.NewUseCasesWithProgressReader(store.Projects, uow, progress[0], logger)
	}
	return projectusecase.NewUseCases(store.Projects, uow, logger)
}
