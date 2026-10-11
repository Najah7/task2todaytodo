package application

import (
	inboxrepository "github.com/Najah7/task2todaytodo/internal/application/inbox/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InboxStore struct {
	Metadata *inboxrepository.MetadataRepository
}

func newInboxStore(pool *pgxpool.Pool) InboxStore {
	return InboxStore{Metadata: inboxrepository.NewMetadataRepository(pool)}
}
