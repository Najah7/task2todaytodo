package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ projectusecase.Repository = (*ProjectRepository)(nil)

type ProjectRepository struct{ queries *sqlc.Queries }

func NewProjectRepository(db sqlc.DBTX) *ProjectRepository {
	return &ProjectRepository{queries: sqlc.New(db)}
}
func (r *ProjectRepository) WithTx(tx pgx.Tx) *ProjectRepository {
	return &ProjectRepository{queries: r.queries.WithTx(tx)}
}

func (r *ProjectRepository) GetByUserID(ctx context.Context, actor domain.UserID, id domain.ProjectID) (dao.Project, error) {
	x, err := r.queries.GetProjectByUserID(ctx, sqlc.GetProjectByUserIDParams{ID: string(id), ActorKey: string(actor)})
	if err != nil {
		return dao.Project{}, repoError(err)
	}
	return projectDAO(x), nil
}
func (r *ProjectRepository) GetByUserIDWithPermission(ctx context.Context, actor domain.UserID, id domain.ProjectID, capability shared.Capability) (dao.Project, error) {
	x, err := r.queries.GetProjectByUserIDForPermission(ctx, sqlc.GetProjectByUserIDForPermissionParams{ID: string(id), ActorID: string(actor), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, e := r.GetByUserID(ctx, actor, id); e == nil {
				return dao.Project{}, projectusecase.ErrPermissionDenied
			}
		}
		return dao.Project{}, repoError(err)
	}
	return projectDAO(x), nil
}
func (r *ProjectRepository) LockByUserIDWithPermission(ctx context.Context, actor domain.UserID, id domain.ProjectID, capability shared.Capability) (dao.Project, error) {
	x, err := r.queries.LockProjectByUserIDForPermission(ctx, sqlc.LockProjectByUserIDForPermissionParams{ID: string(id), UserID: string(actor), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, e := r.GetByUserID(ctx, actor, id); e == nil {
				return dao.Project{}, projectusecase.ErrPermissionDenied
			}
		}
		return dao.Project{}, repoError(err)
	}
	return projectDAO(x), nil
}
func (r *ProjectRepository) HasPermission(ctx context.Context, actor domain.UserID, id domain.ProjectID, capability shared.Capability) (bool, error) {
	ok, err := r.queries.HasProjectPermission(ctx, sqlc.HasProjectPermissionParams{ProjectID: string(id), ActorID: string(actor), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action)})
	return ok, err
}
func (r *ProjectRepository) ListByUserID(ctx context.Context, actor domain.UserID) ([]dao.Project, error) {
	rows, err := r.queries.ListProjectsByUserID(ctx, string(actor))
	if err != nil {
		return nil, err
	}
	out := make([]dao.Project, 0, len(rows))
	for _, x := range rows {
		out = append(out, projectDAO(x))
	}
	return out, nil
}
func (r *ProjectRepository) ListByUserIDCursor(ctx context.Context, actor domain.UserID, limit int, anchor *projectusecase.CursorAnchor) ([]dao.Project, error) {
	arg := sqlc.ListProjectsByUserIDPageParams{UserID: string(actor), PageLimit: int32(limit)}
	if anchor != nil {
		at, err := time.Parse(time.RFC3339Nano, anchor.At)
		if err != nil {
			return nil, projectusecase.ErrInvalidProjectPage
		}
		arg.CursorAt = pgtype.Timestamptz{Time: at, Valid: true}
		arg.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	rows, err := r.queries.ListProjectsByUserIDPage(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]dao.Project, 0, len(rows))
	for _, x := range rows {
		out = append(out, projectDAO(x))
	}
	return out, nil
}
func (r *ProjectRepository) Create(ctx context.Context, p domain.Project) (dao.Project, error) {
	x, err := r.queries.CreateProject(ctx, sqlc.CreateProjectParams{ID: string(p.ID), UserID: string(p.UserID), Type: p.Type.String(), Title: p.Title, Goal: pgText(p.Goal), Description: pgText(p.Description), Priority: priorityValue(p.Priority), StartDate: projectDate(p.Schedule.StartDate), EndDate: projectDate(p.Schedule.EndDate)})
	if err != nil {
		return dao.Project{}, err
	}
	return projectDAO(x), nil
}
func (r *ProjectRepository) UpdateByUserID(ctx context.Context, actor domain.UserID, p domain.Project, expected int32) (dao.Project, error) {
	x, err := r.queries.UpdateProjectByUserID(ctx, sqlc.UpdateProjectByUserIDParams{ID: string(p.ID), ChangedBy: string(actor), UserID: string(actor), ExpectedRevision: expected, Type: p.Type.String(), Title: p.Title, Goal: pgText(p.Goal), Description: pgText(p.Description), Priority: priorityValue(p.Priority), StartDate: projectDate(p.Schedule.StartDate), EndDate: projectDate(p.Schedule.EndDate)})
	if err != nil {
		return dao.Project{}, r.writeError(ctx, actor, p.ID, shared.ProjectUpdate(), err)
	}
	return projectDAO(x), nil
}
func (r *ProjectRepository) DeleteByUserID(ctx context.Context, actor domain.UserID, id domain.ProjectID, expected int32) error {
	_, err := r.queries.DeleteProjectByUserID(ctx, sqlc.DeleteProjectByUserIDParams{ID: string(id), UserID: string(actor), ExpectedRevision: expected})
	if err != nil {
		return r.writeError(ctx, actor, id, shared.ProjectDelete(), err)
	}
	return nil
}
func (r *ProjectRepository) writeError(ctx context.Context, actor domain.UserID, id domain.ProjectID, capability shared.Capability, err error) error {
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	ok, e := r.HasPermission(ctx, actor, id, capability)
	if e != nil {
		return e
	}
	if !ok {
		return projectusecase.ErrPermissionDenied
	}
	if _, e = r.GetByUserIDWithPermission(ctx, actor, id, capability); e != nil {
		return repoError(e)
	}
	return projectusecase.ErrRevisionConflict
}
func repoError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrProjectNotFound
	}
	return err
}

func projectDAO(x sqlc.Project) dao.Project {
	return dao.Project{ID: x.ID, UserID: x.UserID, Type: dao.ProjectType{Value: x.Type}, Title: x.Title, Goal: textValue(x.Goal), Description: textValue(x.Description), Priority: dao.Priority{Value: x.Priority}, StartDate: dateString(x.StartDate), EndDate: dateString(x.EndDate), CreatedAt: x.CreatedAt.Time.Unix(), UpdatedAt: x.UpdatedAt.Time.Unix(), Revision: x.Revision, DeletedAt: unixPointer(x.DeletedAt), ChangedBy: x.ChangedBy, CursorCreatedAt: x.CreatedAt.Time.UTC().Format(time.RFC3339Nano)}
}
func pgText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{String: "", Valid: true}
	}
	return pgtype.Text{String: v, Valid: true}
}
func projectDate(v *time.Time) pgtype.Date {
	if v == nil {
		return pgtype.Date{}
	}
	d := time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
	return pgtype.Date{Time: d, Valid: true}
}
func priorityValue(p domain.Priority) string {
	if p.Value == "" {
		return "low"
	}
	return p.Value
}
func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}
func dateString(v pgtype.Date) *string {
	if !v.Valid {
		return nil
	}
	s := v.Time.Format("2006-01-02")
	return &s
}
func unixPointer(v pgtype.Timestamptz) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Time.Unix()
	return &n
}
