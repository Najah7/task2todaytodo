package repository

import (
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/jackc/pgx/v5/pgtype"
)

func recordToTask(record sqlc.Task) dao.Task {
	return dao.Task{
		ID: record.ID, UserID: record.UserID, ProjectID: pgTextString(record.ProjectID),
		AssigneeID: record.AssigneeID, Title: record.Title, Description: pgTextString(record.Description),
		DueDate: pgDateUnix(record.DueDate), ManualEstimatedMinutes: pgIntPointer(record.ManualEstimatedMinutes),
		EstimatedMinutes: pgIntPointer(record.ManualEstimatedMinutes), EstimateSource: "manual",
		ActualMinutes: pgIntPointer(record.ActualMinutes), Progress: 0, Priority: dao.Priority{Value: record.Priority},
		Status: dao.TaskStatus{Value: record.Status}, CreatedAt: pgUnix(record.CreatedAt), UpdatedAt: pgUnix(record.UpdatedAt),
		Revision: record.Revision, DeletedAt: pgUnixPointer(record.DeletedAt), ChangedBy: record.ChangedBy,
		CursorCreatedAt: record.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
	}
}

func recordsToTasks(records []sqlc.Task) []dao.Task {
	tasks := make([]dao.Task, 0, len(records))
	for _, record := range records {
		tasks = append(tasks, recordToTask(record))
	}
	return tasks
}

func recordsToActionItemsForTaskRows(records []sqlc.ListActionItemsByTaskForUserRow) []dao.ActionItem {
	items := make([]dao.ActionItem, 0, len(records))
	for _, record := range records {
		items = append(items, actionItemDAO(record.ID, record.TaskID, record.Title, record.Description,
			record.DueDate, record.EstimatedMinutes, record.ActualMinutes, record.Started, sqlcBoolean(record.EstimateEditable), record.Priority, record.Completed,
			record.Position, false, record.CreatedAt, record.UpdatedAt))
	}
	return items
}

func actionItemDAO(id, taskID, title string, description pgtype.Text, dueDate pgtype.Date,
	estimatedMinutes int32, actualMinutes pgtype.Int4, started, estimateEditable bool, priority string, completed bool, position int32,
	deleted bool, createdAt, updatedAt pgtype.Timestamptz) dao.ActionItem {
	return dao.ActionItem{
		ID: id, TaskID: taskID, Title: title, Description: pgTextString(description), DueDate: pgDateUnix(dueDate),
		EstimatedMinutes: int32Pointer(estimatedMinutes), ActualMinutes: pgIntPointer(actualMinutes), Started: started, EstimateEditable: estimateEditable, Priority: dao.Priority{Value: priority},
		Completed: completed, Position: int(position), Deleted: deleted,
		CreatedAt: pgUnix(createdAt), UpdatedAt: pgUnix(updatedAt),
	}
}

func int32Pointer(value int32) *int {
	converted := int(value)
	return &converted
}

func pgTextString(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func sqlcBoolean(value interface{}) bool {
	switch boolean := value.(type) {
	case bool:
		return boolean
	case pgtype.Bool:
		return boolean.Valid && boolean.Bool
	default:
		return false
	}
}

func pgIntPointer(value pgtype.Int4) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int32)
	return &v
}

func pgUnix(value pgtype.Timestamptz) int64 {
	if !value.Valid {
		return 0
	}
	return value.Time.Unix()
}

func pgUnixPointer(value pgtype.Timestamptz) *int64 {
	if !value.Valid {
		return nil
	}
	unix := value.Time.Unix()
	return &unix
}

func pgDateUnix(value pgtype.Date) int64 {
	if !value.Valid {
		return 0
	}
	return value.Time.Unix()
}

func pgDateString(value pgtype.Date) *string {
	if !value.Valid {
		return nil
	}
	date := value.Time.Format("2006-01-02")
	return &date
}

func stringToPgText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func intPointerToPgInt(value *int) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*value), Valid: true}
}

func timeToPgTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: !value.IsZero()}
}

func timePointerToPgTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return timeToPgTime(*value)
}

func timeToPgDate(value time.Time) pgtype.Date {
	return pgtype.Date{Time: value, Valid: !value.IsZero()}
}

func taskPriorityString(priority domain.TaskPriority) string {
	if priority == (domain.TaskPriority{}) {
		return "low"
	}
	return priority.String()
}

func taskStatusString(status domain.TaskStatus) string {
	if status == (domain.TaskStatus{}) {
		return "open"
	}
	return status.String()
}
