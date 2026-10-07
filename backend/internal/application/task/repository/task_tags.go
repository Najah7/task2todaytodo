package repository

import (
	"context"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
)

var _ usecase.TaskTagRepository = (*TaskTagRepository)(nil)

// TaskTagRepository manages Task-to-Tag links. Tag definitions live in the Tag context.
type TaskTagRepository struct{ queries *sqlc.Queries }

func NewTaskTagRepository(db sqlc.DBTX) *TaskTagRepository {
	return &TaskTagRepository{queries: sqlc.New(db)}
}
func (repo *TaskTagRepository) WithTx(tx pgx.Tx) *TaskTagRepository {
	return &TaskTagRepository{queries: repo.queries.WithTx(tx)}
}

func (repo TaskTagRepository) ListByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TaskTag, error) {
	rows, err := repo.queries.ListTaskTagsByTaskAndUserID(ctx, sqlc.ListTaskTagsByTaskAndUserIDParams{TaskID: string(taskID), UserID: string(userID)})
	if err != nil {
		return nil, err
	}
	result := make([]dao.TaskTag, 0, len(rows))
	for _, row := range rows {
		result = append(result, recordToTaskTag(row))
	}
	return result, nil
}

func (repo TaskTagRepository) AddToTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, tagID string) error {
	result, err := repo.queries.AddTaskTagToTaskByUserID(ctx, sqlc.AddTaskTagToTaskByUserIDParams{TaskID: string(taskID), TagID: tagID, UserID: string(userID)})
	if err != nil {
		return err
	}
	if !result.Owned {
		return usecase.ErrTaskTagAssignmentNotOwned
	}
	return nil
}

func (repo TaskTagRepository) RemoveFromTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, tagID string) error {
	result, err := repo.queries.RemoveTaskTagFromTaskByUserID(ctx, sqlc.RemoveTaskTagFromTaskByUserIDParams{TaskID: string(taskID), TagID: tagID, UserID: string(userID)})
	if err != nil {
		return err
	}
	if !result.Owned {
		return usecase.ErrTaskTagAssignmentNotOwned
	}
	return nil
}

func recordToTaskTag(row sqlc.Tag) dao.TaskTag {
	return dao.TaskTag{ID: row.ID, UserID: row.UserID, Name: row.Name, CreatedAt: row.CreatedAt.Time.Unix(), UpdatedAt: row.UpdatedAt.Time.Unix()}
}
