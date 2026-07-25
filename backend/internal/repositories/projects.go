package repositories

import (
	"context"

	domain "github.com/Najah7/task2schedule/internal/domain/task"
	"github.com/Najah7/task2schedule/internal/repositories/sqlc"
)

var _ domain.ProjectRepository = ProjectRepository{}

type ProjectRepository struct {
	queries *sqlc.Queries
}

func NewProjectRepository(db sqlc.DBTX) *ProjectRepository {
	return &ProjectRepository{
		queries: sqlc.New(db),
	}
}

func (r ProjectRepository) Get(ctx context.Context, id domain.ProjectID) (domain.Project, error) {
	record, err := r.queries.GetProject(ctx, string(id))
	if err != nil {
		return domain.NewZeroProject(), err
	}
	return recordToProject(record)
}

func (r ProjectRepository) Create(ctx context.Context, project domain.Project) (domain.Project, error) {
	record, err := r.queries.CreateProject(ctx, sqlc.CreateProjectParams{
		ID:          string(project.ID),
		UserID:      string(project.UserID),
		Type:        project.Type.String(),
		Title:       project.Title,
		Goal:        stringToPgText(project.Goal),
		Description: stringToPgText(project.Description),
		Progress:    int16(project.Progress),
		StartAt:     timeToPgTime(project.StartAt),
		EndAt:       timeToPgTime(project.EndAt),
	})
	if err != nil {
		return domain.NewZeroProject(), err
	}
	return recordToProject(record)
}

func (r ProjectRepository) Update(ctx context.Context, project domain.Project) (domain.Project, error) {
	record, err := r.queries.UpdateProject(ctx, sqlc.UpdateProjectParams{
		ID:          string(project.ID),
		Type:        project.Type.String(),
		Title:       project.Title,
		Goal:        stringToPgText(project.Goal),
		Description: stringToPgText(project.Description),
		Progress:    int16(project.Progress),
		StartAt:     timeToPgTime(project.StartAt),
		EndAt:       timeToPgTime(project.EndAt),
	})
	if err != nil {
		return domain.NewZeroProject(), err
	}
	return recordToProject(record)
}

func (r ProjectRepository) Delete(ctx context.Context, id domain.ProjectID) error {
	return r.queries.DeleteProject(ctx, string(id))
}
