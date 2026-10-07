package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type projectListRepository interface {
	ListByUserID(ctx context.Context, userID domain.UserID) ([]dao.Project, error)
}

type ListProjectsUseCase struct {
	repo     projectListRepository
	progress ProjectProgressReader
	logger   logging.Logger
}

func NewListProjectsUseCase(repo projectListRepository, progress ProjectProgressReader, logger logging.Logger) *ListProjectsUseCase {
	return &ListProjectsUseCase{logger: logging.OrNop(logger), repo: repo, progress: progress}
}

func (uc *ListProjectsUseCase) Execute(ctx context.Context, userID domain.UserID) (output []dao.Project, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "ListProjectsUseCase.Execute", err) }()

	projects, err := uc.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return applyProjectProgress(ctx, uc.progress, projects, time.Now())

}

func (uc *ListProjectsUseCase) ExecutePage(ctx context.Context, userID domain.UserID, request CursorPageRequest) (output CursorPage[dao.Project], err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "ListProjectsUseCase.ExecutePage", err) }()

	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.Project]{}, err
	}
	repo, ok := uc.repo.(projectCursorRepository)
	if !ok {
		return CursorPage[dao.Project]{}, ErrInvalidProjectPage
	}
	rows, err := repo.ListByUserIDCursor(ctx, userID, request.Size+1, request.Anchor)
	if err != nil {
		return CursorPage[dao.Project]{}, err
	}
	rows, err = applyProjectProgress(ctx, uc.progress, rows, time.Now())
	if err != nil {
		return CursorPage[dao.Project]{}, err
	}
	return projectPage(rows, request.Size), nil

}
