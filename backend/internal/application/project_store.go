package application

import (
	projectrepository "github.com/Najah7/task2todaytodo/internal/application/project/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectStore struct {
	Projects *projectrepository.ProjectRepository
}

func newProjectStore(pool *pgxpool.Pool) ProjectStore {
	return ProjectStore{Projects: projectrepository.NewProjectRepository(pool)}
}

func (store ProjectStore) WithTx(tx pgx.Tx) ProjectStore {
	return ProjectStore{Projects: store.Projects.WithTx(tx)}
}
