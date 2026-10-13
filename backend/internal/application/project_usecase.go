package application

import (
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

func NewProjectUseCases(store ProjectStore, uow projectusecase.UnitOfWork, logger telemetry.Logger, invitationSender projectusecase.ProjectInvitationSender, frontendURL string, readUOWs ...projectusecase.UnitOfWork) *projectusecase.UseCases {
	return projectusecase.NewUseCases(store.Projects, uow, logger, invitationSender, frontendURL, readUOWs...)
}

func NewProjectUseCasesWithAccess(store ProjectStore, uow projectusecase.UnitOfWork, logger telemetry.Logger, invitationSender projectusecase.ProjectInvitationSender, frontendURL string, access shared.PlanAccessReader, readUOWs ...projectusecase.UnitOfWork) *projectusecase.UseCases {
	return projectusecase.NewUseCasesWithAccess(store.Projects, uow, logger, invitationSender, frontendURL, access, readUOWs...)
}
