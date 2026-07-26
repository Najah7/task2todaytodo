package repositories

import (
	"time"

	domain "github.com/Najah7/task2todaytodo/internal/domain/task"
	"github.com/Najah7/task2todaytodo/internal/repositories/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

func recordToProject(record sqlc.Project) (domain.Project, error) {
	projectType, err := domain.NewProjectType(record.Type)
	if err != nil {
		return domain.NewZeroProject(), err
	}
	priority, err := domain.NewTaskPriority(record.Priority)
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
		pgDateTime(record.DueDate),
		int(record.Progress),
		priority,
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
		pgDateTime(record.DueDate),
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
	return newTaskSchedule(
		domain.TaskScheduleID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		pgTextString(record.Location),
		int(record.IntervalWeeks),
		nil,
		pgTime(record.StartAt),
		pgTime(record.EndAt),
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordToTaskScheduleRow(record sqlc.GetTaskScheduleRow) (domain.TaskSchedule, error) {
	return newTaskSchedule(
		domain.TaskScheduleID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		pgTextString(record.Location),
		int(record.IntervalWeeks),
		record.Frequencies,
		pgTime(record.StartAt),
		pgTime(record.EndAt),
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordToTaskScheduleByTaskAndUserRow(record sqlc.GetTaskScheduleByTaskAndUserRow) (domain.TaskSchedule, error) {
	return newTaskSchedule(
		domain.TaskScheduleID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		pgTextString(record.Location),
		int(record.IntervalWeeks),
		record.Frequencies,
		pgTime(record.StartAt),
		pgTime(record.EndAt),
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordToCreatedTaskScheduleByTaskAndUserRow(record sqlc.CreateTaskScheduleByTaskAndUserRow) (domain.TaskSchedule, error) {
	return newTaskSchedule(
		domain.TaskScheduleID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		pgTextString(record.Location),
		int(record.IntervalWeeks),
		record.Frequencies,
		pgTime(record.StartAt),
		pgTime(record.EndAt),
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordToUpdatedTaskScheduleByTaskAndUserRow(record sqlc.UpdateTaskScheduleByTaskAndUserRow) (domain.TaskSchedule, error) {
	return newTaskSchedule(
		domain.TaskScheduleID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		pgTextString(record.Location),
		int(record.IntervalWeeks),
		record.Frequencies,
		pgTime(record.StartAt),
		pgTime(record.EndAt),
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordsToTaskSchedulesByTaskAndUserRows(records []sqlc.ListTaskSchedulesByTaskAndUserRow) ([]domain.TaskSchedule, error) {
	schedules := make([]domain.TaskSchedule, 0, len(records))
	for _, record := range records {
		schedule, err := newTaskSchedule(
			domain.TaskScheduleID(record.ID),
			domain.TaskID(record.TaskID),
			record.Title,
			pgTextString(record.Description),
			pgTextString(record.Location),
			int(record.IntervalWeeks),
			record.Frequencies,
			pgTime(record.StartAt),
			pgTime(record.EndAt),
			pgTime(record.CreatedAt),
			pgTime(record.UpdatedAt),
		)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

func recordsToTaskSchedulesForTaskRows(records []sqlc.ListTaskSchedulesByTaskForUserRow) ([]domain.TaskSchedule, error) {
	schedules := make([]domain.TaskSchedule, 0, len(records))
	for _, record := range records {
		schedule, err := newTaskSchedule(
			domain.TaskScheduleID(record.ID),
			domain.TaskID(record.TaskID),
			record.Title,
			pgTextString(record.Description),
			pgTextString(record.Location),
			int(record.IntervalWeeks),
			record.Frequencies,
			pgTime(record.StartAt),
			pgTime(record.EndAt),
			pgTime(record.CreatedAt),
			pgTime(record.UpdatedAt),
		)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

func newTaskSchedule(
	id domain.TaskScheduleID,
	taskID domain.TaskID,
	title string,
	description string,
	location string,
	intervalWeeks int,
	frequencyValues []string,
	startAt time.Time,
	endAt time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) (domain.TaskSchedule, error) {
	frequencies, err := taskFrequenciesFromStrings(frequencyValues)
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}

	return domain.NewExistingTaskSchedule(
		id,
		taskID,
		title,
		description,
		location,
		intervalWeeks,
		frequencies,
		startAt,
		endAt,
		createdAt,
		updatedAt,
	)
}

func recordToTodoItem(record sqlc.TodoItem) (domain.TodoItem, error) {
	return newTodoItem(
		domain.TodoItemID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		pgDateTime(record.DueDate),
		record.Completed,
		int(record.Position),
		int(record.IntervalWeeks),
		nil,
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordToTodoItemRow(record sqlc.GetTodoItemRow) (domain.TodoItem, error) {
	return newTodoItem(
		domain.TodoItemID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		pgDateTime(record.DueDate),
		record.Completed,
		int(record.Position),
		int(record.IntervalWeeks),
		record.Frequencies,
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordToTodoItemByTaskAndUserRow(record sqlc.GetTodoItemByTaskAndUserRow) (domain.TodoItem, error) {
	return newTodoItem(
		domain.TodoItemID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		pgDateTime(record.DueDate),
		record.Completed,
		int(record.Position),
		int(record.IntervalWeeks),
		record.Frequencies,
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordToCreatedTodoItemByTaskAndUserRow(record sqlc.CreateTodoItemByTaskAndUserRow) (domain.TodoItem, error) {
	return newTodoItem(
		domain.TodoItemID(record.ID),
		domain.TaskID(record.TaskID),
		record.Title,
		pgTextString(record.Description),
		pgDateTime(record.DueDate),
		record.Completed,
		int(record.Position),
		int(record.IntervalWeeks),
		record.Frequencies,
		pgTime(record.CreatedAt),
		pgTime(record.UpdatedAt),
	)
}

func recordsToTodoItemsByTaskAndUserRows(records []sqlc.ListTodoItemsByTaskAndUserRow) ([]domain.TodoItem, error) {
	items := make([]domain.TodoItem, 0, len(records))
	for _, record := range records {
		item, err := newTodoItem(
			domain.TodoItemID(record.ID),
			domain.TaskID(record.TaskID),
			record.Title,
			pgTextString(record.Description),
			pgDateTime(record.DueDate),
			record.Completed,
			int(record.Position),
			int(record.IntervalWeeks),
			record.Frequencies,
			pgTime(record.CreatedAt),
			pgTime(record.UpdatedAt),
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func recordsToTodoItemsForTaskRows(records []sqlc.ListTodoItemsByTaskForUserRow) ([]domain.TodoItem, error) {
	items := make([]domain.TodoItem, 0, len(records))
	for _, record := range records {
		item, err := newTodoItem(
			domain.TodoItemID(record.ID),
			domain.TaskID(record.TaskID),
			record.Title,
			pgTextString(record.Description),
			pgDateTime(record.DueDate),
			record.Completed,
			int(record.Position),
			int(record.IntervalWeeks),
			record.Frequencies,
			pgTime(record.CreatedAt),
			pgTime(record.UpdatedAt),
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func newTodoItem(
	id domain.TodoItemID,
	taskID domain.TaskID,
	title string,
	description string,
	dueDate time.Time,
	completed bool,
	position int,
	intervalWeeks int,
	frequencyValues []string,
	createdAt time.Time,
	updatedAt time.Time,
) (domain.TodoItem, error) {
	frequencies, err := taskFrequenciesFromStrings(frequencyValues)
	if err != nil {
		return domain.NewZeroTodoItem(), err
	}

	return domain.NewExistingTodoItem(
		id,
		taskID,
		title,
		description,
		dueDate,
		completed,
		position,
		intervalWeeks,
		frequencies,
		createdAt,
		updatedAt,
	)
}

func taskFrequenciesFromStrings(values []string) (domain.TaskFrequencies, error) {
	frequencies := make(domain.TaskFrequencies, 0, len(values))
	for _, value := range values {
		frequency, err := domain.NewTaskFrequency(value)
		if err != nil {
			return nil, err
		}
		frequencies = append(frequencies, frequency)
	}
	return frequencies, nil
}

func taskFrequencyStrings(frequencies domain.TaskFrequencies) []string {
	values := make([]string, 0, len(frequencies))
	for _, frequency := range frequencies {
		values = append(values, frequency.String())
	}
	return values
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
