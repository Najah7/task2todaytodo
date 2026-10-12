package application

import (
	operationrepo "github.com/Najah7/task2todaytodo/internal/application/admin/operation/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminStore struct {
	Operations *operationrepo.OperationRepository
}

func newAdminStore(pool *pgxpool.Pool) AdminStore {
	return AdminStore{Operations: operationrepo.NewOperationRepository(pool)}
}
