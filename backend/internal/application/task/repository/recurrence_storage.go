package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/oklog/ulid/v2"
)

func (r TodoItemRepository) SetTodoItemRecurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, anchor time.Time, intervalWeeks int, frequencies []dao.TaskFrequency) error {
	params := sqlc.SetTodoItemRecurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
		FrequencyAnchorDate: timeToPgDate(anchor), IntervalWeeks: int32(intervalWeeks),
	}
	rows, err := r.queries.SetTodoItemRecurrenceByTaskAndUserID(ctx, params)
	if err != nil {
		return err
	}
	if rows == 0 {
		return usecase.ErrTodoItemNotFound
	}
	owner := sqlc.ReplaceTodoItemFrequenciesByTaskAndUserIDParams{SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID)}
	if err := r.queries.ReplaceTodoItemFrequenciesByTaskAndUserID(ctx, owner); err != nil {
		return err
	}
	return r.queries.CreateTodoItemFrequenciesByTaskAndUserID(ctx, sqlc.CreateTodoItemFrequenciesByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), Frequencies: taskFrequencyValues(frequencies),
	})
}

func (r TodoItemRepository) StopTodoItemRecurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID) error {
	rows, err := r.queries.StopTodoItemRecurrenceByTaskAndUserID(ctx, sqlc.StopTodoItemRecurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return usecase.ErrTodoItemNotFound
	}
	return r.queries.ClearTodoItemFrequenciesByTaskAndUserID(ctx, sqlc.ClearTodoItemFrequenciesByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	})
}

func (r TodoItemRepository) UpdateTodoItemSeriesTemplate(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID, snapshotID domain.TodoItemID, item domain.TodoItem) (dao.TodoItem, error) {
	owner := sqlc.SnapshotTodoItemRootOccurrenceByTaskAndUserIDParams{
		ID: string(snapshotID), SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	}
	if err := r.queries.SnapshotTodoItemRootOccurrenceByTaskAndUserID(ctx, owner); err != nil {
		return dao.TodoItem{}, err
	}
	rows, err := r.queries.UpdateTodoItemSeriesTemplateByTaskAndUserID(ctx, sqlc.UpdateTodoItemSeriesTemplateByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), Title: item.Title,
		Description: stringToPgText(item.Description), DueDate: timeToPgDate(item.DueDate), Position: int32(item.Position),
	})
	if err != nil {
		return dao.TodoItem{}, err
	}
	if rows == 0 {
		return dao.TodoItem{}, usecase.ErrTodoItemNotFound
	}
	return r.GetForOwnedTask(ctx, userID, taskID, seriesID)
}

func (r TodoItemRepository) UpsertTodoItemOverride(ctx context.Context, userID domain.UserID, item domain.TodoItem) (string, error) {
	return r.queries.UpsertTodoItemOverrideByTaskAndUserID(ctx, sqlc.UpsertTodoItemOverrideByTaskAndUserIDParams{
		ID: string(item.ID), TaskID: string(item.TaskID), UserID: string(userID), SeriesID: string(item.SeriesID),
		Title: item.Title, Description: stringToPgText(item.Description), DueDate: timeToPgDate(item.DueDate),
		Completed: item.Completed, Position: int32(item.Position), OccurrenceDate: timeToPgDate(item.OccurrenceDate),
		Timezone: item.Timezone, Deleted: item.Deleted,
	})
}

func (r TodoItemRepository) ListTodoItemSkippedOccurrences(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID) ([]int64, error) {
	rows, err := r.queries.ListTodoItemSkippedOccurrencesByTaskAndUserID(ctx, sqlc.ListTodoItemSkippedOccurrencesByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	})
	if err != nil {
		return nil, err
	}
	dates := make([]int64, 0, len(rows))
	for _, occurrenceDate := range rows {
		dates = append(dates, pgDateUnix(occurrenceDate))
	}
	return dates, nil
}

func (r TodoItemRepository) ListTodoItemSkippedOccurrencesForCapability(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, capability shared.Capability) ([]int64, error) {
	rows, err := r.queries.ListTodoItemSkippedOccurrencesForCapabilityByTaskAndUserID(ctx, sqlc.ListTodoItemSkippedOccurrencesForCapabilityByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), ActorID: string(userID),
		ResourceID: string(capability.Resource), Action: sqlc.PermissionAction(capability.Action),
	})
	if err != nil {
		return nil, err
	}
	dates := make([]int64, 0, len(rows))
	for _, occurrenceDate := range rows {
		dates = append(dates, pgDateUnix(occurrenceDate))
	}
	return dates, nil
}

func (r TodoItemRepository) SetTodoItemSkippedOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, occurrenceDate time.Time, skipped bool) error {
	date := timeToPgDate(occurrenceDate)
	owner := sqlc.SkipTodoItemOccurrenceByTaskAndUserIDParams{
		ID: ulid.Make().String(), SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), OccurrenceDate: date,
	}
	if skipped {
		_, err := r.queries.SkipTodoItemOccurrenceByTaskAndUserID(ctx, owner)
		return err
	}
	restore := sqlc.RestoreEditedTodoItemOccurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), OccurrenceDate: date,
	}
	if err := r.queries.RestoreEditedTodoItemOccurrenceByTaskAndUserID(ctx, restore); err != nil {
		return err
	}
	return r.queries.DeleteSkippedTodoItemOccurrenceByTaskAndUserID(ctx, sqlc.DeleteSkippedTodoItemOccurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), OccurrenceDate: date,
	})
}

func (r TaskScheduleRepository) SetTaskScheduleRecurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, anchor time.Time, intervalWeeks int, frequencies []dao.TaskFrequency) error {
	params := sqlc.SetTaskScheduleRecurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
		FrequencyAnchorDate: timeToPgDate(anchor), IntervalWeeks: int32(intervalWeeks),
	}
	rows, err := r.queries.SetTaskScheduleRecurrenceByTaskAndUserID(ctx, params)
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrTaskScheduleNotFound
	}
	owner := sqlc.ReplaceTaskScheduleFrequenciesByTaskAndUserIDParams{SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID)}
	if err := r.queries.ReplaceTaskScheduleFrequenciesByTaskAndUserID(ctx, owner); err != nil {
		return err
	}
	return r.queries.CreateTaskScheduleFrequenciesByTaskAndUserID(ctx, sqlc.CreateTaskScheduleFrequenciesByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), Frequencies: taskFrequencyValues(frequencies),
	})
}

func (r TaskScheduleRepository) StopTaskScheduleRecurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID) error {
	rows, err := r.queries.StopTaskScheduleRecurrenceByTaskAndUserID(ctx, sqlc.StopTaskScheduleRecurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrTaskScheduleNotFound
	}
	return r.queries.ClearTaskScheduleFrequenciesByTaskAndUserID(ctx, sqlc.ClearTaskScheduleFrequenciesByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	})
}

func (r TaskScheduleRepository) UpdateTaskScheduleSeriesTemplate(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID, snapshotID domain.TaskScheduleID, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	owner := sqlc.SnapshotTaskScheduleRootOccurrenceByTaskAndUserIDParams{
		ID: string(snapshotID), SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	}
	if err := r.queries.SnapshotTaskScheduleRootOccurrenceByTaskAndUserID(ctx, owner); err != nil {
		return dao.TaskSchedule{}, err
	}
	rows, err := r.queries.UpdateTaskScheduleSeriesTemplateByTaskAndUserID(ctx, sqlc.UpdateTaskScheduleSeriesTemplateByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), Title: schedule.Title,
		Description: stringToPgText(schedule.Description), Location: stringToPgText(schedule.Location),
		StartAt: timeToPgTime(schedule.StartAt), EndAt: timeToPgTime(schedule.EndAt), Timezone: schedule.Timezone,
	})
	if err != nil {
		return dao.TaskSchedule{}, err
	}
	if rows == 0 {
		return dao.TaskSchedule{}, domain.ErrTaskScheduleNotFound
	}
	return r.GetByTaskAndUserID(ctx, userID, taskID, seriesID)
}

func (r TaskScheduleRepository) UpsertTaskScheduleOverride(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (string, error) {
	return r.queries.UpsertTaskScheduleOverrideByTaskAndUserID(ctx, sqlc.UpsertTaskScheduleOverrideByTaskAndUserIDParams{
		ID: string(schedule.ID), TaskID: string(schedule.TaskID), UserID: string(userID), SeriesID: string(schedule.SeriesID),
		Title: schedule.Title, Description: stringToPgText(schedule.Description), Location: stringToPgText(schedule.Location),
		StartAt: timeToPgTime(schedule.StartAt), EndAt: timeToPgTime(schedule.EndAt), OccurrenceDate: timeToPgDate(schedule.OccurrenceDate),
		Timezone: schedule.Timezone, Completed: schedule.Completed, Deleted: schedule.Deleted,
	})
}

func (r TaskScheduleRepository) ListTaskScheduleSkippedOccurrences(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID) ([]int64, error) {
	rows, err := r.queries.ListTaskScheduleSkippedOccurrencesByTaskAndUserID(ctx, sqlc.ListTaskScheduleSkippedOccurrencesByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	})
	if err != nil {
		return nil, err
	}
	dates := make([]int64, 0, len(rows))
	for _, occurrenceDate := range rows {
		dates = append(dates, pgDateUnix(occurrenceDate))
	}
	return dates, nil
}

func (r TaskScheduleRepository) ListTaskScheduleSkippedOccurrencesForCapability(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, capability shared.Capability) ([]int64, error) {
	rows, err := r.queries.ListTaskScheduleSkippedOccurrencesForCapabilityByTaskAndUserID(ctx, sqlc.ListTaskScheduleSkippedOccurrencesForCapabilityByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), ActorID: string(userID),
		ResourceID: string(capability.Resource), Action: sqlc.PermissionAction(capability.Action),
	})
	if err != nil {
		return nil, err
	}
	dates := make([]int64, 0, len(rows))
	for _, occurrenceDate := range rows {
		dates = append(dates, pgDateUnix(occurrenceDate))
	}
	return dates, nil
}

func (r TaskScheduleRepository) SetTaskScheduleSkippedOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, occurrenceDate time.Time, skipped bool) error {
	date := timeToPgDate(occurrenceDate)
	owner := sqlc.SkipTaskScheduleOccurrenceByTaskAndUserIDParams{
		ID: ulid.Make().String(), SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), OccurrenceDate: date,
	}
	if skipped {
		_, err := r.queries.SkipTaskScheduleOccurrenceByTaskAndUserID(ctx, owner)
		return err
	}
	restore := sqlc.RestoreEditedTaskScheduleOccurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), OccurrenceDate: date,
	}
	if err := r.queries.RestoreEditedTaskScheduleOccurrenceByTaskAndUserID(ctx, restore); err != nil {
		return err
	}
	return r.queries.DeleteSkippedTaskScheduleOccurrenceByTaskAndUserID(ctx, sqlc.DeleteSkippedTaskScheduleOccurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), OccurrenceDate: date,
	})
}

func unixToPgDate(value int64) pgtype.Date {
	return pgtype.Date{Time: time.Unix(value, 0).UTC(), Valid: true}
}

func taskFrequenciesFromStrings(values []string) []dao.TaskFrequency {
	frequencies := make([]dao.TaskFrequency, 0, len(values))
	for _, value := range values {
		frequencies = append(frequencies, dao.TaskFrequency{Value: value})
	}
	return frequencies
}

func taskFrequencyValues(frequencies []dao.TaskFrequency) []string {
	values := make([]string, 0, len(frequencies))
	for _, frequency := range frequencies {
		values = append(values, frequency.Value)
	}
	return values
}
