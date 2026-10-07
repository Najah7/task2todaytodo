package application

import (
	"github.com/Najah7/task2todaytodo/internal/application/tag/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TagStore struct{ Tags *repository.TagRepository }

func newTagStore(pool *pgxpool.Pool) TagStore {
	return TagStore{Tags: repository.NewTagRepository(pool)}
}
