package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r TaskRepository) ReadTaskListCandidates(ctx context.Context, userID domain.UserID, request usecase.TaskListRequest) ([]dao.Task, error) {
	arg := sqlc.ListTaskListCandidatesParams{
		UserID: string(userID), DueFilter: request.DueFilter,
		AsOf: pgtype.Timestamptz{Time: request.AsOf, Valid: true},
	}
	if request.ProjectID != "" {
		arg.ProjectID = pgtype.Text{String: request.ProjectID, Valid: true}
	}
	if request.Title != "" {
		arg.Title = pgtype.Text{String: request.Title, Valid: true}
	}
	rows, err := r.queries.ListTaskListCandidates(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]dao.Task, 0, len(rows))
	for _, row := range rows {
		task := taskListCandidateDAO(sqlc.Task{ID: row.ID, UserID: row.UserID, ProjectID: row.ProjectID, AssigneeID: row.AssigneeID, Title: row.Title, Description: row.Description, DueDate: row.DueDate, ManualEstimatedMinutes: row.ManualEstimatedMinutes, ActualMinutes: row.ActualMinutes, Priority: row.Priority, Status: row.Status, Revision: row.Revision, DeletedAt: row.DeletedAt, ChangedBy: row.ChangedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, row.ProjectName, row.ProjectStatus)
		task.CanUpdate = row.CanUpdate
		if row.DueDate.Valid {
			remainingDays := int(row.RemainingDays)
			task.RemainingDays = &remainingDays
		}
		out = append(out, task)
	}
	return out, nil
}

func (r TaskRepository) ListTaskPage(ctx context.Context, userID domain.UserID, request usecase.TaskListRequest, limit int) ([]dao.Task, error) {
	arg := sqlc.ListTaskListPageParams{
		UserID: string(userID), DueFilter: request.DueFilter, SortBy: request.SortBy,
		SortOrder: request.SortOrder, Direction: "forward", PageLimit: int32(limit),
		AsOf: pgtype.Timestamptz{Time: request.AsOf, Valid: true},
	}
	if request.Status != "all" {
		arg.Status = pgtype.Text{String: request.Status, Valid: true}
	}
	if request.ProjectID != "" {
		arg.ProjectID = pgtype.Text{String: request.ProjectID, Valid: true}
	}
	if request.Title != "" {
		arg.Title = pgtype.Text{String: request.Title, Valid: true}
	}
	if request.Anchor != nil {
		arg.Direction = request.Anchor.Direction
		arg.AnchorID = pgtype.Text{String: request.Anchor.ID, Valid: true}
		at, err := time.Parse(time.RFC3339Nano, request.Anchor.CreatedAt)
		if err != nil {
			return nil, usecase.ErrInvalidTaskListRequest
		}
		arg.AnchorAt = pgtype.Timestamptz{Time: at, Valid: true}
		switch request.SortBy {
		case "title":
			arg.AnchorTitle = pgtype.Text{String: request.Anchor.Title, Valid: true}
		case "due_date":
			if !request.Anchor.DueDateNull {
				date, err := time.Parse("2006-01-02", request.Anchor.DueDate)
				if err != nil {
					return nil, usecase.ErrInvalidTaskListRequest
				}
				arg.AnchorDueDate = pgtype.Date{Time: date, Valid: true}
			}
		}
	}
	rows, err := r.queries.ListTaskListPage(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]dao.Task, 0, len(rows))
	for _, row := range rows {
		task := taskListCandidateDAO(sqlc.Task{ID: row.ID, UserID: row.UserID, ProjectID: row.ProjectID, AssigneeID: row.AssigneeID, Title: row.Title, Description: row.Description, DueDate: row.DueDate, ManualEstimatedMinutes: row.ManualEstimatedMinutes, ActualMinutes: row.ActualMinutes, Priority: row.Priority, Status: row.Status, Revision: row.Revision, DeletedAt: row.DeletedAt, ChangedBy: row.ChangedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, row.ProjectName, row.ProjectStatus)
		task.CanUpdate = row.CanUpdate
		if row.DueDate.Valid {
			remainingDays := int(row.RemainingDays)
			task.RemainingDays = &remainingDays
		}
		out = append(out, task)
	}
	return out, nil
}

func taskListCandidateDAO(record sqlc.Task, projectName, projectStatus string) dao.Task {
	task := recordToTask(record)
	task.ProjectName = projectName
	task.ProjectStatus = projectStatus
	return task
}
