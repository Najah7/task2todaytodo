package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r TaskRepository) ListTaskRevisionsByActor(ctx context.Context, actor domain.UserID, task domain.TaskID, limit int, anchor *usecase.CursorAnchor) ([]dao.TaskRevision, error) {
	arg := sqlc.ListTaskRevisionsByActorParams{ID: string(task), ActorID: string(actor), PageLimit: int32(limit)}
	if anchor != nil {
		if anchor.Revision < 1 {
			return nil, usecase.ErrInvalidTaskPage
		}
		arg.CursorRevision = pgtype.Int4{Int32: anchor.Revision, Valid: true}
	}
	rows, err := r.queries.ListTaskRevisionsByActor(ctx, arg)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 && anchor == nil {
		return nil, usecase.ErrTaskNotFound
	}
	revisions := make([]dao.TaskRevision, 0, len(rows))
	for _, row := range rows {
		revisions = append(revisions, dao.TaskRevision{
			ID: row.ID, Revision: row.Revision, UserID: row.UserID, ProjectID: pgTextString(row.ProjectID),
			AssigneeID: row.AssigneeID, Title: row.Title, Description: pgTextString(row.Description),
			DueDate: pgDateUnix(row.DueDate), ManualEstimatedMinutes: pgIntPointer(row.ManualEstimatedMinutes),
			ActualMinutes: pgIntPointer(row.ActualMinutes), Priority: row.Priority, Status: row.Status,
			DeletedAt: pgUnixPointer(row.DeletedAt), CreatedAt: row.CreatedAt.Time.Unix(),
			UpdatedAt: row.UpdatedAt.Time.Unix(), ChangedBy: row.ChangedBy, ChangedAt: row.ChangedAt.Time.Unix(),
			CursorAt: row.ChangedAt.Time.UTC().Format(time.RFC3339Nano),
		})
	}
	return revisions, nil
}
