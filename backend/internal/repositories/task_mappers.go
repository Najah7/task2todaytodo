package repositories

import (
	"time"

	domain "github.com/Najah7/task2schedule/internal/domain/task"
	"github.com/Najah7/task2schedule/internal/repositories/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

func recordToProject(record sqlc.Project) (domain.Project, error) {
	projectType, err := domain.NewProjectType(record.Type)
	if err != nil {
		return domain.NewZeroProject(), err
	}

	return domain.NewExistingProject(
		domain.ProjectID(record.ID),
		domain.UserID(record.UserID),
		projectType,
		record.Title,
		pgTextString(record.Goal),
		pgTextString(record.Description),
		int(record.Progress),
		pgTime(record.StartAt),
		pgTime(record.EndAt),
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordToTask(record sqlc.Task) (domain.Task, error) {
	priority, err := domain.NewTaskPriority(record.Priority)
	if err != nil {
		return domain.NewZeroTask(), err
	}
	status, err := domain.NewTaskStatus(record.Status)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	return domain.NewExistingTask(
		domain.TaskID(record.ID),
		domain.UserID(record.UserID),
		domain.ProjectID(pgTextString(record.ProjectID)),
		record.Title,
		pgTextString(record.Description),
		pgIntPointer(record.EstimatedMinutes),
		pgIntPointer(record.ActualMinutes),
		int(record.Progress),
		priority,
		status,
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordsToTasks(records []sqlc.Task) ([]domain.Task, error) {
	tasks := make([]domain.Task, 0, len(records))
	for _, record := range records {
		task, err := recordToTask(record)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func recordToTaskSchedule(record sqlc.TaskSchedule) (domain.TaskSchedule, error) {
	return domain.NewExistingTaskSchedule(
		domain.TaskScheduleID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		pgTextString(record.Location),
		int(record.IntervalWeeks),
		pgTime(record.StartAt),
		pgTime(record.EndAt),
		pgTime(record.DueAt),
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordToTodoItem(record sqlc.TodoItem) (domain.TodoItem, error) {
	return domain.NewExistingTodoItem(
		domain.TodoItemID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		record.Completed,
		int(record.Position),
		int(record.IntervalWeeks),
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordToTodoList(record sqlc.TodoList) (domain.TodoList, error) {
	return domain.NewExistingTodoList(
		domain.TodoListID(record.ID),
		domain.UserID(record.UserID),
		pgDateTime(record.ListDate),
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordsToTodoLists(records []sqlc.TodoList) ([]domain.TodoList, error) {
	lists := make([]domain.TodoList, 0, len(records))
	for _, record := range records {
		list, err := recordToTodoList(record)
		if err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}
	return lists, nil
}

func pgTextString(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func stringToPgText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func pgIntPointer(value pgtype.Int4) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int32)
	return &v
}

func intPointerToPgInt(value *int) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*value), Valid: true}
}

func pgTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func timeToPgTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: !value.IsZero()}
}

func pgDateTime(value pgtype.Date) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
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
