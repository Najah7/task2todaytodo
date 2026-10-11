package application

import (
	adminrepo "github.com/Najah7/task2todaytodo/internal/application/admin/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminStore struct {
	Operations *adminrepo.OperationRepository
}

func newAdminStore(pool *pgxpool.Pool) AdminStore {
	return AdminStore{Operations: adminrepo.NewOperationRepository(pool)}
}
