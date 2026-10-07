package repository

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/tag/dao"
	"github.com/Najah7/task2todaytodo/internal/application/tag/domain"
	"github.com/Najah7/task2todaytodo/internal/application/tag/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.Repository = (*TagRepository)(nil)

type TagRepository struct{ queries *sqlc.Queries }

func NewTagRepository(db sqlc.DBTX) *TagRepository { return &TagRepository{queries: sqlc.New(db)} }

func (repo *TagRepository) GetByUserID(ctx context.Context, userID string, id domain.TagID) (dao.Tag, error) {
	row, err := repo.queries.GetTagByUserID(ctx, sqlc.GetTagByUserIDParams{ID: string(id), UserID: userID})
	if err != nil {
		return dao.Tag{}, tagRepositoryError(err)
	}
	return recordToTag(row), nil
}

func (repo *TagRepository) ListByUserIDCursor(ctx context.Context, userID string, limit int, anchor *usecase.CursorAnchor) ([]dao.Tag, error) {
	params := sqlc.ListTagsByUserIDPageParams{UserID: userID, PageLimit: int32(limit)}
	if anchor != nil {
		params.CursorName = pgtype.Text{String: anchor.Name, Valid: true}
		params.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	rows, err := repo.queries.ListTagsByUserIDPage(ctx, params)
	if err != nil {
		return nil, err
	}
	result := make([]dao.Tag, 0, len(rows))
	for _, row := range rows {
		result = append(result, recordToTag(row))
	}
	return result, nil
}

func (repo *TagRepository) Create(ctx context.Context, tag domain.Tag) (dao.Tag, error) {
	row, err := repo.queries.CreateTag(ctx, sqlc.CreateTagParams{ID: string(tag.ID), UserID: tag.UserID, Name: tag.Name})
	if err != nil {
		return dao.Tag{}, tagRepositoryError(err)
	}
	return recordToTag(row), nil
}

func (repo *TagRepository) RenameByUserID(ctx context.Context, userID string, tag domain.Tag) (dao.Tag, error) {
	row, err := repo.queries.RenameTagByUserID(ctx, sqlc.RenameTagByUserIDParams{ID: string(tag.ID), UserID: userID, Name: tag.Name})
	if err != nil {
		return dao.Tag{}, tagRepositoryError(err)
	}
	return recordToTag(row), nil
}

func (repo *TagRepository) DeleteByUserID(ctx context.Context, userID string, id domain.TagID) error {
	count, err := repo.queries.DeleteTagByUserID(ctx, sqlc.DeleteTagByUserIDParams{ID: string(id), UserID: userID})
	if err != nil {
		return err
	}
	if count == 0 {
		return usecase.ErrTagNotFound
	}
	return nil
}

func recordToTag(row sqlc.Tag) dao.Tag {
	return dao.Tag{ID: row.ID, UserID: row.UserID, Name: row.Name, CreatedAt: row.CreatedAt.Time.Unix(), UpdatedAt: row.UpdatedAt.Time.Unix()}
}

func tagRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrTagNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "tags_user_id_name_key" {
		return usecase.ErrTagNameConflict
	}
	return err
}
