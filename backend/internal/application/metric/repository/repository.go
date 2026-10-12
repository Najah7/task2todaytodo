package repository

import (
	"context"
	"fmt"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/metric/dao"
	"github.com/Najah7/task2todaytodo/internal/application/metric/usecase"
)

type MetricRepository struct {
	queries *sqlc.Queries
}

func NewMetricRepository(db sqlc.DBTX) *MetricRepository {
	return &MetricRepository{queries: sqlc.New(db)}
}

func (r *MetricRepository) ReadTodoCompletion(ctx context.Context, actorID string, days int) (dao.TodoCompletion, error) {
	rows, err := r.queries.GetTodoCompletionMetrics(ctx, sqlc.GetTodoCompletionMetricsParams{
		UserID: actorID,
		Days:   int32(days),
	})
	if err != nil {
		return dao.TodoCompletion{}, err
	}
	if len(rows) == 0 {
		return dao.TodoCompletion{}, fmt.Errorf("metrics user %q not found", actorID)
	}
	result := dao.TodoCompletion{
		Timezone:  rows[0].Timezone,
		StartDate: rows[0].StartDate,
		EndDate:   rows[0].EndDate,
		Days:      make([]dao.CompletionDay, 0, len(rows)),
	}
	for _, row := range rows {
		result.Days = append(result.Days, dao.CompletionDay{
			Date:           row.MetricDate,
			PlannedItems:   row.PlannedItems,
			CompletedItems: row.CompletedItems,
		})
	}
	return result, nil
}

func (r *MetricRepository) ReadOverdueWork(ctx context.Context, actorID string) (dao.OverdueWork, error) {
	rows, err := r.queries.GetOverdueWorkMetrics(ctx, actorID)
	if err != nil {
		return dao.OverdueWork{}, err
	}
	if len(rows) == 0 {
		return dao.OverdueWork{}, fmt.Errorf("metrics user %q not found", actorID)
	}
	result := dao.OverdueWork{
		Timezone: rows[0].Timezone,
		AsOfDate: rows[0].AsOfDate,
		Projects: make([]dao.OverdueProject, 0, len(rows)),
	}
	for _, row := range rows {
		if !row.OverdueCount.Valid {
			continue
		}
		project := dao.OverdueProject{Count: row.OverdueCount.Int32}
		result.TotalCount += row.OverdueCount.Int32
		if row.ProjectID.Valid {
			id := row.ProjectID.String
			project.ProjectID = &id
		}
		if row.ProjectTitle.Valid {
			title := row.ProjectTitle.String
			project.ProjectTitle = &title
		}
		result.Projects = append(result.Projects, project)
	}
	return result, nil
}

var _ usecase.TodoCompletionReader = (*MetricRepository)(nil)
var _ usecase.OverdueWorkReader = (*MetricRepository)(nil)
