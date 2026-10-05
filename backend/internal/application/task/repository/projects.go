package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

func (r ProjectRepository) GetByUserID(ctx context.Context, userID domain.UserID, id domain.ProjectID) (dao.Project, error) {
	record, err := r.queries.GetProjectByUserID(ctx, sqlc.GetProjectByUserIDParams{
		ID:       string(id),
		ActorKey: string(userID),
	})
	if err != nil {
		return dao.Project{}, projectRepositoryError(err)
	}
	return recordToProject(record), nil
}

func (r ProjectRepository) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.ProjectID, capability shared.Capability) (dao.Project, error) {
	record, err := r.queries.GetProjectByUserIDForPermission(ctx, sqlc.GetProjectByUserIDForPermissionParams{
		ID: string(id), ActorID: string(userID), ResourceID: string(capability.Resource), Action: sqlc.PermissionAction(capability.Action),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, readErr := r.GetByUserID(ctx, userID, id); readErr == nil {
				return dao.Project{}, usecase.ErrPermissionDenied
			}
		}
		return dao.Project{}, projectRepositoryError(err)
	}
	return recordToProject(record), nil
}

func (r ProjectRepository) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.ProjectID, capability shared.Capability) (dao.Project, error) {
	record, err := r.queries.LockProjectByUserIDForPermission(ctx, sqlc.LockProjectByUserIDForPermissionParams{
		ID: string(id), UserID: string(userID), ResourceID: string(capability.Resource), Action: sqlc.PermissionAction(capability.Action),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, readErr := r.GetByUserID(ctx, userID, id); readErr == nil {
				return dao.Project{}, usecase.ErrPermissionDenied
			}
		}
		return dao.Project{}, projectRepositoryError(err)
	}
	return recordToProject(record), nil
}

func (r ProjectRepository) HasPermission(ctx context.Context, userID domain.UserID, id domain.ProjectID, capability shared.Capability) (bool, error) {
	allowed, err := r.queries.HasProjectPermission(ctx, sqlc.HasProjectPermissionParams{
		ProjectID: string(id), ActorID: string(userID), ResourceID: string(capability.Resource), Action: sqlc.PermissionAction(capability.Action),
	})
	if err != nil {
		return false, projectRepositoryError(err)
	}
	return allowed, nil
}

func (r ProjectRepository) LockActiveTasksForDeletion(ctx context.Context, projectID domain.ProjectID) error {
	_, err := r.queries.LockActiveProjectTasksForDeletion(ctx, string(projectID))
	return err
}

func (r ProjectRepository) ListByUserID(ctx context.Context, userID domain.UserID) ([]dao.Project, error) {
	records, err := r.queries.ListProjectsByUserID(ctx, string(userID))
	if err != nil {
		return nil, err
	}
	projects := make([]dao.Project, 0, len(records))
	for _, record := range records {
		projects = append(projects, recordToProject(record))
	}
	return projects, nil
}

func (r ProjectRepository) ListByUserIDCursor(ctx context.Context, userID domain.UserID, limit int, anchor *usecase.CursorAnchor) ([]dao.Project, error) {
	arg := sqlc.ListProjectsByUserIDPageParams{UserID: string(userID), PageLimit: int32(limit)}
	if anchor != nil {
		at, err := time.Parse(time.RFC3339Nano, anchor.At)
		if err != nil {
			return nil, usecase.ErrInvalidTaskPage
		}
		arg.CursorAt = pgtype.Timestamptz{Time: at, Valid: true}
		arg.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	records, err := r.queries.ListProjectsByUserIDPage(ctx, arg)
	if err != nil {
		return nil, err
	}
	projects := make([]dao.Project, 0, len(records))
	for _, record := range records {
		projects = append(projects, recordToProject(record))
	}
	return projects, nil
}

func (r ProjectRepository) GetDetailsByUserID(ctx context.Context, userID domain.UserID, id domain.ProjectID) (dao.ProjectDetails, error) {
	project, err := r.GetByUserID(ctx, userID, id)
	if err != nil {
		return dao.ProjectDetails{}, err
	}

	taskRecords, err := r.queries.ListProjectTasksByUserID(ctx, sqlc.ListProjectTasksByUserIDParams{
		ProjectID: stringToPgText(string(id)),
		ActorKey:  string(userID),
	})
	if err != nil {
		return dao.ProjectDetails{}, err
	}

	tasks := recordsToTasks(taskRecords)

	return dao.ProjectDetails{
		Project: project,
		Tasks:   tasks,
	}, nil
}

func (r ProjectRepository) Create(ctx context.Context, project domain.Project) (dao.Project, error) {
	record, err := r.queries.CreateProject(ctx, sqlc.CreateProjectParams{
		ID:          string(project.ID),
		UserID:      string(project.UserID),
		Type:        project.Type.String(),
		Title:       project.Title,
		Goal:        stringToPgText(project.Goal),
		Description: stringToPgText(project.Description),
		Priority:    taskPriorityString(project.Priority),
		StartDate:   projectDatePointerToPgDate(project.Schedule.StartDate),
		EndDate:     projectDatePointerToPgDate(project.Schedule.EndDate),
	})
	if err != nil {
		return dao.Project{}, err
	}
	return recordToProject(record), nil
}

func (r ProjectRepository) UpdateByUserID(ctx context.Context, userID domain.UserID, project domain.Project, expectedRevision int32) (dao.Project, error) {
	record, err := r.queries.UpdateProjectByUserID(ctx, sqlc.UpdateProjectByUserIDParams{
		ID:               string(project.ID),
		ChangedBy:        string(userID),
		UserID:           string(userID),
		ExpectedRevision: expectedRevision,
		Type:             project.Type.String(),
		Title:            project.Title,
		Goal:             stringToPgText(project.Goal),
		Description:      stringToPgText(project.Description),
		Priority:         taskPriorityString(project.Priority),
		StartDate:        projectDatePointerToPgDate(project.Schedule.StartDate),
		EndDate:          projectDatePointerToPgDate(project.Schedule.EndDate),
	})
	if err != nil {
		return dao.Project{}, r.projectWriteError(ctx, userID, project.ID, shared.ProjectUpdate(), err)
	}
	return recordToProject(record), nil
}

func (r ProjectRepository) DeleteByUserID(ctx context.Context, userID domain.UserID, id domain.ProjectID, expectedRevision int32) error {
	_, err := r.queries.DeleteProjectByUserID(ctx, sqlc.DeleteProjectByUserIDParams{
		ID:               string(id),
		UserID:           string(userID),
		ExpectedRevision: expectedRevision,
	})
	if err != nil {
		return r.projectWriteError(ctx, userID, id, shared.ProjectDelete(), err)
	}
	return nil
}

func (r ProjectRepository) projectWriteError(ctx context.Context, userID domain.UserID, projectID domain.ProjectID, capability shared.Capability, err error) error {
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	allowed, accessErr := r.HasPermission(ctx, userID, projectID, capability)
	if accessErr != nil {
		return accessErr
	}
	if !allowed {
		return usecase.ErrPermissionDenied
	}
	if _, getErr := r.GetByUserIDWithPermission(ctx, userID, projectID, capability); getErr != nil {
		return projectRepositoryError(getErr)
	}
	return usecase.ErrRevisionConflict
}

func projectRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrProjectNotFound
	}
	return err
}
