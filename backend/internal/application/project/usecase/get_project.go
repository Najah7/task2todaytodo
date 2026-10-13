package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type getProjectRepository interface {
	GetByUserID(ctx context.Context, userID domain.UserID, id domain.ProjectID) (dao.Project, error)
	HasPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (bool, error)
}

type GetProjectUseCase struct {
	repo     getProjectRepository
	progress ProjectProgressReader
	logger   telemetry.Logger
	readUOW  projectReadUnitOfWork
}

func NewGetProjectUseCase(repo getProjectRepository, progress ProjectProgressReader, logger telemetry.Logger, readUOWs ...projectReadUnitOfWork) *GetProjectUseCase {
	var readUOW projectReadUnitOfWork
	if len(readUOWs) > 0 {
		readUOW = readUOWs[0]
	}
	return &GetProjectUseCase{logger: telemetry.OrNop(logger), repo: repo, progress: progress, readUOW: readUOW}
}

func (uc *GetProjectUseCase) Execute(ctx context.Context, userID domain.UserID, projectID domain.ProjectID) (output dao.Project, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "GetProjectUseCase.Execute", err) }()

	var project dao.Project
	if uc.readUOW != nil {
		err = uc.readUOW.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
			var err error
			project, err = repo.GetByUserID(ctx, userID, projectID)
			if err != nil {
				return err
			}
			return readProjectCapabilities(ctx, repo, userID, &project)
		})
	} else {
		project, err = uc.repo.GetByUserID(ctx, userID, projectID)
		if err == nil {
			err = readProjectCapabilities(ctx, uc.repo, userID, &project)
		}
	}
	if err != nil {
		return dao.Project{}, err
	}
	projects, err := applyProjectProgress(ctx, uc.progress, []dao.Project{project}, time.Now())
	if err != nil {
		return dao.Project{}, err
	}
	return projects[0], nil

}
