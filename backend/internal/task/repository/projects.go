package repository

import (
	"context"
	"errors"
	usecase "github.com/Najah7/task2todaytodo/internal/task/usecase"

	"github.com/Najah7/task2todaytodo/internal/repositories/sqlc"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	"github.com/jackc/pgx/v5"
)

var _ usecase.ProjectRepository = ProjectRepository{}

type ProjectRepository struct {
	queries *sqlc.Queries
}

func NewProjectRepository(db sqlc.DBTX) *ProjectRepository {
	return &ProjectRepository{
		queries: sqlc.New(db),
	}
}

func (r *ProjectRepository) WithTx(tx pgx.Tx) *ProjectRepository {
	return &ProjectRepository{
		queries: r.queries.WithTx(tx),
	}
}

func (r ProjectRepository) GetByUser(ctx context.Context, userID domain.UserID, id domain.ProjectID) (domain.Project, error) {
	record, err := r.queries.GetProjectByUser(ctx, sqlc.GetProjectByUserParams{
		ID:     string(id),
		UserID: string(userID),
	})
	if err != nil {
		return domain.NewZeroProject(), projectRepositoryError(err)
	}
	return recordToProject(record)
}

func (r ProjectRepository) GetAggregateByUser(ctx context.Context, userID domain.UserID, id domain.ProjectID) (domain.ProjectAggregate, error) {
	project, err := r.GetByUser(ctx, userID, id)
	if err != nil {
		return domain.ProjectAggregate{}, err
	}

	taskRecords, err := r.queries.ListProjectTasksByUser(ctx, sqlc.ListProjectTasksByUserParams{
		ProjectID: stringToPgText(string(id)),
		UserID:    string(userID),
	})
	if err != nil {
		return domain.ProjectAggregate{}, err
	}

	tasks, err := recordsToTasks(taskRecords)
	if err != nil {
		return domain.ProjectAggregate{}, err
	}

	return domain.ProjectAggregate{
		Project: project,
		Tasks:   tasks,
	}, nil
}

func (r ProjectRepository) Create(ctx context.Context, project domain.Project) (domain.Project, error) {
	record, err := r.queries.CreateProject(ctx, sqlc.CreateProjectParams{
		ID:          string(project.ID),
		UserID:      string(project.UserID),
		Type:        project.Type.String(),
		Title:       project.Title,
		Goal:        stringToPgText(project.Goal),
		Description: stringToPgText(project.Description),
		DueDate:     timeToPgDate(project.DueDate),
		Progress:    int16(project.Progress),
		Priority:    taskPriorityString(project.Priority),
		StartAt:     timeToPgTime(project.StartAt),
		EndAt:       timeToPgTime(project.EndAt),
	})
	if err != nil {
		return domain.NewZeroProject(), err
	}
	return recordToProject(record)
}

func (r ProjectRepository) UpdateByUser(ctx context.Context, userID domain.UserID, project domain.Project) (domain.Project, error) {
	record, err := r.queries.UpdateProjectByUser(ctx, sqlc.UpdateProjectByUserParams{
		ID:          string(project.ID),
		UserID:      string(userID),
		Type:        project.Type.String(),
		Title:       project.Title,
		Goal:        stringToPgText(project.Goal),
		Description: stringToPgText(project.Description),
		DueDate:     timeToPgDate(project.DueDate),
		Progress:    int16(project.Progress),
		Priority:    taskPriorityString(project.Priority),
		StartAt:     timeToPgTime(project.StartAt),
		EndAt:       timeToPgTime(project.EndAt),
	})
	if err != nil {
		return domain.NewZeroProject(), projectRepositoryError(err)
	}
	return recordToProject(record)
}

func (r ProjectRepository) DeleteByUser(ctx context.Context, userID domain.UserID, id domain.ProjectID) error {
	rowsAffected, err := r.queries.DeleteProjectByUser(ctx, sqlc.DeleteProjectByUserParams{
		ID:     string(id),
		UserID: string(userID),
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrProjectNotFound
	}
	return nil
}

func projectRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrProjectNotFound
	}
	return err
}
