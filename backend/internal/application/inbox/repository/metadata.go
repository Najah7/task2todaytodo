package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/inbox/dao"
)

type MetadataRepository struct {
	queries *sqlc.Queries
}

func NewMetadataRepository(db sqlc.DBTX) *MetadataRepository {
	return &MetadataRepository{queries: sqlc.New(db)}
}

func (r MetadataRepository) ListScheduleRuleMetadata(ctx context.Context, ruleIDs []string) ([]dao.ScheduleRuleMetadata, error) {
	if len(ruleIDs) == 0 {
		return []dao.ScheduleRuleMetadata{}, nil
	}
	rows, err := r.queries.ListInboxScheduleRuleCreatedAt(ctx, ruleIDs)
	if err != nil {
		return nil, err
	}
	metadata := make([]dao.ScheduleRuleMetadata, 0, len(rows))
	for _, row := range rows {
		if !row.CreatedAt.Valid {
			continue
		}
		metadata = append(metadata, dao.ScheduleRuleMetadata{
			RuleID: row.ID, CreatedAt: row.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
		})
	}
	return metadata, nil
}

func (r MetadataRepository) ListProjectMetadata(ctx context.Context, actorID string, projectIDs []string) ([]dao.ProjectMetadata, error) {
	if len(projectIDs) == 0 {
		return []dao.ProjectMetadata{}, nil
	}
	rows, err := r.queries.ListInboxProjectNames(ctx, sqlc.ListInboxProjectNamesParams{ProjectIds: projectIDs, ActorID: actorID})
	if err != nil {
		return nil, err
	}
	metadata := make([]dao.ProjectMetadata, 0, len(rows))
	for _, row := range rows {
		metadata = append(metadata, dao.ProjectMetadata{ProjectID: row.ID, Name: row.Title})
	}
	return metadata, nil
}
