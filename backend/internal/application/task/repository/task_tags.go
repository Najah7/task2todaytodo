package repository

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.TaskTagRepository = TaskTagRepository{}

type TaskTagRepository struct {
	queries *sqlc.Queries
}

func NewTaskTagRepository(db sqlc.DBTX) *TaskTagRepository {
	return &TaskTagRepository{queries: sqlc.New(db)}
}

func (r *TaskTagRepository) WithTx(tx pgx.Tx) *TaskTagRepository {
	return &TaskTagRepository{queries: r.queries.WithTx(tx)}
}

func (r TaskTagRepository) GetByUserID(ctx context.Context, userID domain.UserID, id domain.TaskTagID) (dao.TaskTag, error) {
	record, err := r.queries.GetTaskTagByUserID(ctx, sqlc.GetTaskTagByUserIDParams{
		ID:     string(id),
		UserID: string(userID),
	})
	if err != nil {
		return dao.TaskTag{}, taskTagRepositoryError(err)
	}
	return recordToTaskTag(record), nil
}

func (r TaskTagRepository) ListByUserID(ctx context.Context, userID domain.UserID) ([]dao.TaskTag, error) {
	records, err := r.queries.ListTaskTagsByUserID(ctx, string(userID))
	if err != nil {
		return nil, err
	}
	tags := make([]dao.TaskTag, 0, len(records))
	for _, record := range records {
		tags = append(tags, recordToTaskTag(record))
	}
	return tags, nil
}

func (r TaskTagRepository) ListByUserIDCursor(ctx context.Context, userID domain.UserID, limit int, anchor *usecase.CursorAnchor) ([]dao.TaskTag, error) {
	arg := sqlc.ListTaskTagsByUserIDPageParams{UserID: string(userID), PageLimit: int32(limit)}
	if anchor != nil {
		arg.CursorName = pgtype.Text{String: anchor.Name, Valid: true}
		arg.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	records, err := r.queries.ListTaskTagsByUserIDPage(ctx, arg)
	if err != nil {
		return nil, err
	}
	tags := make([]dao.TaskTag, 0, len(records))
	for _, record := range records {
		tags = append(tags, recordToTaskTag(record))
	}
	return tags, nil
}

func (r TaskTagRepository) ListByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TaskTag, error) {
	records, err := r.queries.ListTaskTagsByTaskAndUserID(ctx, sqlc.ListTaskTagsByTaskAndUserIDParams{
		TaskID: string(taskID),
		UserID: string(userID),
	})
	if err != nil {
		return nil, err
	}
	tags := make([]dao.TaskTag, 0, len(records))
	for _, record := range records {
		tags = append(tags, recordToTaskTag(record))
	}
	return tags, nil
}

func (r TaskTagRepository) Create(ctx context.Context, tag domain.TaskTag) (dao.TaskTag, error) {
	record, err := r.queries.CreateTaskTag(ctx, sqlc.CreateTaskTagParams{
		ID:     string(tag.ID),
		UserID: string(tag.UserID),
		Name:   tag.Name,
	})
	if err != nil {
		return dao.TaskTag{}, taskTagRepositoryError(err)
	}
	return recordToTaskTag(record), nil
}

func (r TaskTagRepository) RenameByUserID(ctx context.Context, userID domain.UserID, tag domain.TaskTag) (dao.TaskTag, error) {
	record, err := r.queries.RenameTaskTagByUserID(ctx, sqlc.RenameTaskTagByUserIDParams{
		ID:     string(tag.ID),
		UserID: string(userID),
		Name:   tag.Name,
	})
	if err != nil {
		return dao.TaskTag{}, taskTagRepositoryError(err)
	}
	return recordToTaskTag(record), nil
}

func (r TaskTagRepository) DeleteByUserID(ctx context.Context, userID domain.UserID, id domain.TaskTagID) error {
	rowsAffected, err := r.queries.DeleteTaskTagByUserID(ctx, sqlc.DeleteTaskTagByUserIDParams{
		ID:     string(id),
		UserID: string(userID),
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return usecase.ErrTaskTagNotFound
	}
	return nil
}

// AddToTask is idempotent. It reports an ownership error if either record is
// absent or belongs to another user; duplicate assignments are successful.
func (r TaskTagRepository) AddToTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, tagID domain.TaskTagID) error {
	result, err := r.queries.AddTaskTagToTaskByUserID(ctx, sqlc.AddTaskTagToTaskByUserIDParams{
		TaskID: string(taskID),
		TagID:  string(tagID),
		UserID: string(userID),
	})
	if err != nil {
		return err
	}
	if !result.Owned {
		return usecase.ErrTaskTagAssignmentNotOwned
	}
	return nil
}

// RemoveFromTask is idempotent. It reports an ownership error if either record
// is absent or belongs to another user.
func (r TaskTagRepository) RemoveFromTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, tagID domain.TaskTagID) error {
	result, err := r.queries.RemoveTaskTagFromTaskByUserID(ctx, sqlc.RemoveTaskTagFromTaskByUserIDParams{
		TaskID: string(taskID),
		TagID:  string(tagID),
		UserID: string(userID),
	})
	if err != nil {
		return err
	}
	if !result.Owned {
		return usecase.ErrTaskTagAssignmentNotOwned
	}
	return nil
}

func recordToTaskTag(record sqlc.TaskTag) dao.TaskTag {
	return dao.TaskTag{
		ID:        record.ID,
		UserID:    record.UserID,
		Name:      record.Name,
		CreatedAt: pgUnix(record.CreatedAt),
		UpdatedAt: pgUnix(record.UpdatedAt),
	}
}

func taskTagRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrTaskTagNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "task_tags_user_id_name_key" {
		return usecase.ErrTaskTagNameConflict
	}
	return err
}
