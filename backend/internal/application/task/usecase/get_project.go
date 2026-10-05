package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type getProjectRepository interface {
	GetByUserID(ctx context.Context, userID domain.UserID, id domain.ProjectID) (dao.Project, error)
}

type GetProjectUseCase struct {
	repo     getProjectRepository
	progress taskProgressSource
	logger   logging.Logger
}

func NewGetProjectUseCase(repo getProjectRepository, progress taskProgressSource, logger logging.Logger) *GetProjectUseCase {
	return &GetProjectUseCase{logger: logging.OrNop(logger), repo: repo, progress: progress}
}

func (uc *GetProjectUseCase) Execute(ctx context.Context, userID domain.UserID, projectID domain.ProjectID) (output dao.Project, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "GetProjectUseCase.Execute", err) }()

	project, err := uc.repo.GetByUserID(ctx, userID, projectID)
	if err != nil {
		return dao.Project{}, err
	}
	projects, err := applyProjectProgress(ctx, uc.progress, []dao.Project{project}, time.Now())
	if err != nil {
		return dao.Project{}, err
	}
	return projects[0], nil

}
