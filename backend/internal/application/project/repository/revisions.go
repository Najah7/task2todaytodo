package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *ProjectRepository) ListProjectRevisionsByActor(ctx context.Context, actor domain.UserID, project domain.ProjectID, limit int, anchor *projectusecase.CursorAnchor) ([]dao.ProjectRevision, error) {
	arg := sqlc.ListProjectRevisionsByActorParams{ID: string(project), ActorID: string(actor), PageLimit: int32(limit)}
	if anchor != nil {
		if anchor.Revision < 1 {
			return nil, projectusecase.ErrInvalidProjectPage
		}
		arg.CursorRevision = pgtype.Int4{Int32: anchor.Revision, Valid: true}
	}
	rows, err := r.queries.ListProjectRevisionsByActor(ctx, arg)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 && anchor == nil {
		return nil, domain.ErrProjectNotFound
	}
	out := make([]dao.ProjectRevision, 0, len(rows))
	for _, x := range rows {
		out = append(out, dao.ProjectRevision{ID: x.ID, Revision: x.Revision, UserID: x.UserID, Type: x.Type, Title: x.Title, Goal: textValue(x.Goal), Description: textValue(x.Description), Priority: x.Priority, Status: x.Status, StartDate: dateString(x.StartDate), EndDate: dateString(x.EndDate), DeletedAt: unixPointer(x.DeletedAt), CreatedAt: x.CreatedAt.Time.Unix(), UpdatedAt: x.UpdatedAt.Time.Unix(), ChangedBy: x.ChangedBy, ChangedAt: x.ChangedAt.Time.Unix(), CursorAt: x.ChangedAt.Time.UTC().Format(time.RFC3339Nano)})
	}
	return out, nil
}

func (r *ProjectRepository) ListProjectTypes(ctx context.Context) ([]dao.ProjectType, error) {
	rows, err := r.queries.ListProjectTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dao.ProjectType, 0, len(rows))
	for _, x := range rows {
		out = append(out, dao.ProjectType{Value: x.Type, Label: x.Label, LabelJp: x.LabelJp, CreatedAt: x.CreatedAt.Time.Unix(), UpdatedAt: x.UpdatedAt.Time.Unix()})
	}
	return out, nil
}
