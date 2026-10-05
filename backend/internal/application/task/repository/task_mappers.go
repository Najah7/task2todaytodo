package repository

import (
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/jackc/pgx/v5/pgtype"
)

func recordToProject(record sqlc.Project) dao.Project {
	return dao.Project{
		ID:              record.ID,
		UserID:          record.UserID,
		Type:            dao.ProjectType{Value: record.Type},
		Title:           record.Title,
		Goal:            pgTextString(record.Goal),
		Description:     pgTextString(record.Description),
		Progress:        0,
		Priority:        dao.Priority{Value: record.Priority},
		StartDate:       pgDateString(record.StartDate),
		EndDate:         pgDateString(record.EndDate),
		CreatedAt:       pgUnix(record.CreatedAt),
		UpdatedAt:       pgUnix(record.UpdatedAt),
		Revision:        record.Revision,
		DeletedAt:       pgUnixPointer(record.DeletedAt),
		ChangedBy:       record.ChangedBy,
		CursorCreatedAt: record.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
	}
}

func recordToTask(record sqlc.Task) dao.Task {
	return dao.Task{
		ID:               record.ID,
		UserID:           record.UserID,
		ProjectID:        pgTextString(record.ProjectID),
		AssigneeID:       record.AssigneeID,
		Title:            record.Title,
		Description:      pgTextString(record.Description),
		DueDate:          pgDateUnix(record.DueDate),
		EstimatedMinutes: pgIntPointer(record.EstimatedMinutes),
		ActualMinutes:    pgIntPointer(record.ActualMinutes),
		Progress:         0,
		Priority:         dao.Priority{Value: record.Priority},
		Status:           dao.TaskStatus{Value: record.Status},
		CreatedAt:        pgUnix(record.CreatedAt),
		UpdatedAt:        pgUnix(record.UpdatedAt),
		Revision:         record.Revision,
		DeletedAt:        pgUnixPointer(record.DeletedAt),
		ChangedBy:        record.ChangedBy,
		CursorCreatedAt:  record.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
	}
}

func recordsToTasks(records []sqlc.Task) []dao.Task {
	tasks := make([]dao.Task, 0, len(records))
	for _, record := range records {
		tasks = append(tasks, recordToTask(record))
	}
	return tasks
}

func recordToTaskScheduleRow(record sqlc.GetTaskScheduleRow) dao.TaskSchedule {
	schedule := taskScheduleDAO(
		record.ID,
		record.TaskID,
		record.Title,
		record.Description,
		record.Location,
		0,
		record.SeriesID,
		record.OccurrenceDate,
		record.Timezone,
		record.IsException,
		sqlcBoolean(record.Deleted),
		record.Frequencies,
		record.StartAt,
		record.EndAt,
		record.CreatedAt,
		record.UpdatedAt,
		record.Completed,
	)
	if record.RepeatState.Valid {
		schedule.RepeatState = record.RepeatState.String
	}
	schedule.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
	return schedule
}

func recordToTaskScheduleByTaskAndUserIDRow(record sqlc.GetTaskScheduleByTaskAndUserIDRow) dao.TaskSchedule {
	schedule := taskScheduleDAO(
		record.ID,
		record.TaskID,
		record.Title,
		record.Description,
		record.Location,
		record.IntervalWeeks,
		record.SeriesID,
		record.OccurrenceDate,
		record.Timezone,
		record.IsException,
		sqlcBoolean(record.Deleted),
		record.Frequencies,
		record.StartAt,
		record.EndAt,
		record.CreatedAt,
		record.UpdatedAt,
		record.Completed,
	)
	if record.RepeatState.Valid {
		schedule.RepeatState = record.RepeatState.String
	}
	schedule.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
	return schedule
}

func recordToCreatedTaskScheduleByTaskAndUserIDRow(record sqlc.CreateTaskScheduleByTaskAndUserIDRow) dao.TaskSchedule {
	return taskScheduleDAO(
		record.ID,
		record.TaskID,
		record.Title,
		record.Description,
		record.Location,
		record.IntervalWeeks,
		record.SeriesID,
		record.OccurrenceDate,
		record.Timezone,
		record.IsException,
		sqlcBoolean(record.Deleted),
		record.Frequencies,
		record.StartAt,
		record.EndAt,
		record.CreatedAt,
		record.UpdatedAt,
		record.Completed,
	)
}

func recordToUpdatedTaskScheduleByTaskAndUserIDRow(record sqlc.UpdateTaskScheduleByTaskAndUserIDRow) dao.TaskSchedule {
	return taskScheduleDAO(
		record.ID,
		record.TaskID,
		record.Title,
		record.Description,
		record.Location,
		record.IntervalWeeks,
		record.SeriesID,
		record.OccurrenceDate,
		record.Timezone,
		record.IsException,
		record.Deleted,
		record.Frequencies,
		record.StartAt,
		record.EndAt,
		record.CreatedAt,
		record.UpdatedAt,
		record.Completed,
	)
}

func recordsToTaskSchedulesForTaskRows(records []sqlc.ListTaskSchedulesByTaskForUserRow) []dao.TaskSchedule {
	schedules := make([]dao.TaskSchedule, 0, len(records))
	for _, record := range records {
		schedules = append(schedules, taskScheduleDAOWithoutRecurrence(
			record.ID,
			record.TaskID,
			record.Title,
			record.Description,
			record.Location,
			record.IntervalWeeks,
			record.Frequencies,
			record.StartAt,
			record.EndAt,
			record.CreatedAt,
			record.UpdatedAt,
			record.Completed,
		))
	}
	return schedules
}

func recordToCreatedTaskScheduleOccurrenceByTaskAndUserIDRow(record sqlc.CreateTaskScheduleOccurrenceByTaskAndUserIDRow) dao.TaskSchedule {
	return taskScheduleDAO(
		record.ID,
		record.TaskID,
		record.Title,
		record.Description,
		record.Location,
		record.IntervalWeeks,
		record.SeriesID,
		record.OccurrenceDate,
		record.Timezone,
		record.IsException,
		sqlcBoolean(record.Deleted),
		record.Frequencies,
		record.StartAt,
		record.EndAt,
		record.CreatedAt,
		record.UpdatedAt,
		record.Completed,
	)
}

func recordsToTaskSchedulesByTaskAndUserIDRows(records []sqlc.ListTaskSchedulesByTaskAndUserIDRow) []dao.TaskSchedule {
	schedules := make([]dao.TaskSchedule, 0, len(records))
	for _, record := range records {
		schedule := taskScheduleDAO(
			record.ID,
			record.TaskID,
			record.Title,
			record.Description,
			record.Location,
			record.IntervalWeeks,
			record.SeriesID,
			record.OccurrenceDate,
			record.Timezone,
			record.IsException,
			sqlcBoolean(record.Deleted),
			record.Frequencies,
			record.StartAt,
			record.EndAt,
			record.CreatedAt,
			record.UpdatedAt,
			record.Completed,
		)
		if record.RepeatState.Valid {
			schedule.RepeatState = record.RepeatState.String
		}
		schedule.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
		schedules = append(schedules, schedule)
	}
	return schedules
}

func recordsToActiveTaskScheduleSeriesByUserIDRows(records []sqlc.ListActiveTaskScheduleSeriesByUserIDRow) []dao.TaskSchedule {
	schedules := make([]dao.TaskSchedule, 0, len(records))
	for _, record := range records {
		schedule := taskScheduleDAO(
			record.ID,
			record.TaskID,
			record.Title,
			record.Description,
			record.Location,
			record.IntervalWeeks,
			record.SeriesID,
			record.OccurrenceDate,
			record.Timezone,
			record.IsException,
			sqlcBoolean(record.Deleted),
			record.Frequencies,
			record.StartAt,
			record.EndAt,
			record.CreatedAt,
			record.UpdatedAt,
			record.Completed,
		)
		if record.RepeatState.Valid {
			schedule.RepeatState = record.RepeatState.String
		}
		schedule.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
		schedules = append(schedules, schedule)
	}
	return schedules
}

func taskScheduleDAO(
	id string,
	taskID string,
	title string,
	description pgtype.Text,
	location pgtype.Text,
	intervalWeeks int32,
	seriesID string,
	occurrenceDate pgtype.Date,
	timezone string,
	isException bool,
	deleted bool,
	frequencies []string,
	startAt pgtype.Timestamptz,
	endAt pgtype.Timestamptz,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
	completed ...bool,
) dao.TaskSchedule {
	date := pgDateString(occurrenceDate)
	occurrenceDateString := ""
	if date != nil {
		occurrenceDateString = *date
	}
	completedValue := false
	if len(completed) > 0 {
		completedValue = completed[0]
	}
	repeatState := ""
	frequencyAnchorDate := int64(0)
	if id == seriesID {
		repeatState = "one_off"
		if intervalWeeks > 0 {
			repeatState = "active"
			frequencyAnchorDate = pgDateUnix(occurrenceDate)
		}
	}
	return dao.TaskSchedule{
		ID:                  id,
		TaskID:              taskID,
		Title:               title,
		Description:         pgTextString(description),
		Location:            pgTextString(location),
		IntervalWeeks:       int(intervalWeeks),
		RepeatState:         repeatState,
		FrequencyAnchorDate: frequencyAnchorDate,
		SeriesID:            seriesID,
		OccurrenceDate:      occurrenceDateString,
		Timezone:            timezone,
		IsException:         isException,
		Completed:           completedValue,
		Deleted:             deleted,
		Frequencies:         taskFrequenciesDAO(frequencies),
		StartAt:             pgUnix(startAt),
		EndAt:               pgUnix(endAt),
		CreatedAt:           pgUnix(createdAt),
		UpdatedAt:           pgUnix(updatedAt),
		CursorStartAt:       startAt.Time.UTC().Format(time.RFC3339Nano),
	}
}

func taskScheduleDAOWithoutRecurrence(
	id string,
	taskID string,
	title string,
	description pgtype.Text,
	location pgtype.Text,
	intervalWeeks int32,
	frequencies []string,
	startAt pgtype.Timestamptz,
	endAt pgtype.Timestamptz,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
	completed ...bool,
) dao.TaskSchedule {
	return taskScheduleDAO(id, taskID, title, description, location, intervalWeeks, "", pgtype.Date{}, "", false, false, frequencies, startAt, endAt, createdAt, updatedAt, completed...)
}

func recordToTodoItem(record sqlc.TodoItem) dao.TodoItem {
	return todoItemDAO(
		record.ID,
		record.TaskID,
		record.Title,
		record.Description,
		record.DueDate,
		record.Completed,
		record.Position,
		0,
		nil,
		record.SeriesID,
		record.OccurrenceDate,
		record.Timezone,
		record.IsException,
		record.DeletedAt.Valid,
		record.CreatedAt,
		record.UpdatedAt,
	)
}

func recordToCreatedTodoItemRow(record sqlc.CreateTodoItemRow) dao.TodoItem {
	return todoItemDAO(record.ID, record.TaskID, record.Title, record.Description,
		record.DueDate, record.Completed, record.Position, record.IntervalWeeks, nil,
		record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt)
}

func recordToUpdatedTodoItemRow(record sqlc.UpdateTodoItemRow) dao.TodoItem {
	return todoItemDAO(record.ID, record.TaskID, record.Title, record.Description,
		record.DueDate, record.Completed, record.Position, record.IntervalWeeks, nil,
		record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt)
}

func recordToTodoItemRow(record sqlc.GetTodoItemRow) dao.TodoItem {
	item := todoItemDAO(
		record.ID,
		record.TaskID,
		record.Title,
		record.Description,
		record.DueDate,
		record.Completed,
		record.Position,
		record.IntervalWeeks,
		record.Frequencies,
		record.SeriesID,
		record.OccurrenceDate,
		record.Timezone,
		record.IsException,
		sqlcBoolean(record.Deleted),
		record.CreatedAt,
		record.UpdatedAt,
	)
	if record.RepeatState.Valid {
		item.RepeatState = record.RepeatState.String
	}
	item.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
	return item
}

func recordToTodoItemByTaskAndUserIDRow(record sqlc.GetTodoItemByTaskAndUserIDRow) dao.TodoItem {
	item := todoItemDAO(record.ID, record.TaskID, record.Title, record.Description,
		record.DueDate, record.Completed, record.Position, record.IntervalWeeks, record.Frequencies,
		record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt)
	if record.RepeatState.Valid {
		item.RepeatState = record.RepeatState.String
	}
	item.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
	return item
}

func recordToCreatedTodoItemByTaskAndUserIDRow(record sqlc.CreateTodoItemByTaskAndUserIDRow) dao.TodoItem {
	return todoItemDAO(
		record.ID,
		record.TaskID,
		record.Title,
		record.Description,
		record.DueDate,
		record.Completed,
		record.Position,
		record.IntervalWeeks,
		record.Frequencies,
		record.SeriesID,
		record.OccurrenceDate,
		record.Timezone,
		record.IsException,
		sqlcBoolean(record.Deleted),
		record.CreatedAt,
		record.UpdatedAt,
	)
}

func recordToCreatedTodoItemOccurrenceByTaskAndUserIDRow(record sqlc.CreateTodoItemOccurrenceByTaskAndUserIDRow) dao.TodoItem {
	return todoItemDAO(record.ID, record.TaskID, record.Title, record.Description,
		record.DueDate, record.Completed, record.Position, record.IntervalWeeks, record.Frequencies,
		record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt)
}

func recordToUpdatedTodoItemByTaskAndUserIDRow(record sqlc.UpdateTodoItemByTaskAndUserIDRow) dao.TodoItem {
	return todoItemDAO(record.ID, record.TaskID, record.Title, record.Description,
		record.DueDate, record.Completed, record.Position, record.IntervalWeeks, record.Frequencies,
		record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt)
}

func recordToReorderedTodoItemByTaskAndUserIDRow(record sqlc.ReorderTodoItemsByTaskAndUserIDRow) dao.TodoItem {
	return todoItemDAO(record.ID, record.TaskID, record.Title, record.Description,
		record.DueDate, record.Completed, record.Position, record.IntervalWeeks, record.Frequencies,
		record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt)
}

func recordsToTodoItems(records []sqlc.ListTodoItemsByTaskAndUserIDRow) []dao.TodoItem {
	items := make([]dao.TodoItem, 0, len(records))
	for _, record := range records {
		item := todoItemDAO(
			record.ID,
			record.TaskID,
			record.Title,
			record.Description,
			record.DueDate,
			record.Completed,
			record.Position,
			record.IntervalWeeks,
			record.Frequencies,
			record.SeriesID,
			record.OccurrenceDate,
			record.Timezone,
			record.IsException,
			sqlcBoolean(record.Deleted),
			record.CreatedAt,
			record.UpdatedAt,
		)
		if record.RepeatState.Valid {
			item.RepeatState = record.RepeatState.String
		}
		item.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
		items = append(items, item)
	}
	return items
}

func recordsToTodoItemsForTaskRows(records []sqlc.ListTodoItemsByTaskForUserRow) []dao.TodoItem {
	items := make([]dao.TodoItem, 0, len(records))
	for _, record := range records {
		items = append(items, todoItemDAO(
			record.ID,
			record.TaskID,
			record.Title,
			record.Description,
			record.DueDate,
			record.Completed,
			record.Position,
			record.IntervalWeeks,
			record.Frequencies,
			"",
			pgtype.Date{},
			"",
			false,
			false,
			record.CreatedAt,
			record.UpdatedAt,
		))
	}
	return items
}

func todoItemDAO(
	id string,
	taskID string,
	title string,
	description pgtype.Text,
	dueDate pgtype.Date,
	completed bool,
	position int32,
	intervalWeeks int32,
	frequencies []string,
	seriesID string,
	occurrenceDate pgtype.Date,
	timezone string,
	isException bool,
	deleted bool,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
) dao.TodoItem {
	occurrenceDateString := ""
	if date := pgDateString(occurrenceDate); date != nil {
		occurrenceDateString = *date
	}
	repeatState := ""
	frequencyAnchorDate := int64(0)
	if id == seriesID {
		repeatState = "one_off"
		if intervalWeeks > 0 {
			repeatState = "active"
			frequencyAnchorDate = pgDateUnix(occurrenceDate)
		}
	}
	return dao.TodoItem{
		ID:                  id,
		TaskID:              taskID,
		Title:               title,
		Description:         pgTextString(description),
		DueDate:             pgDateUnix(dueDate),
		Completed:           completed,
		Position:            int(position),
		IntervalWeeks:       int(intervalWeeks),
		RepeatState:         repeatState,
		FrequencyAnchorDate: frequencyAnchorDate,
		Frequencies:         taskFrequenciesDAO(frequencies),
		SeriesID:            seriesID,
		OccurrenceDate:      occurrenceDateString,
		Timezone:            timezone,
		IsException:         isException,
		Deleted:             deleted,
		CreatedAt:           pgUnix(createdAt),
		UpdatedAt:           pgUnix(updatedAt),
	}
}

func taskFrequencyStrings(frequencies domain.TaskFrequencies) []string {
	values := make([]string, 0, len(frequencies))
	for _, frequency := range frequencies {
		values = append(values, frequency.String())
	}
	return values
}

func taskFrequenciesDAO(values []string) []dao.TaskFrequency {
	frequencies := make([]dao.TaskFrequency, 0, len(values))
	for _, value := range values {
		frequencies = append(frequencies, dao.TaskFrequency{Value: value})
	}
	return frequencies
}

func recordToTodoList(record sqlc.TodoList) dao.TodoList {
	return dao.TodoList{
		ID:        record.ID,
		UserID:    record.UserID,
		ListDate:  pgDateUnix(record.ListDate),
		CreatedAt: pgUnix(record.CreatedAt),
		UpdatedAt: pgUnix(record.UpdatedAt),
	}
}

func recordsToTodoLists(records []sqlc.TodoList) []dao.TodoList {
	lists := make([]dao.TodoList, 0, len(records))
	for _, record := range records {
		lists = append(lists, recordToTodoList(record))
	}
	return lists
}

func pgTextString(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func sqlcBoolean(value interface{}) bool {
	boolean, _ := value.(bool)
	return boolean
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

func projectDatePointerToPgDate(value *time.Time) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	date := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
	return pgtype.Date{Time: date, Valid: true}
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
