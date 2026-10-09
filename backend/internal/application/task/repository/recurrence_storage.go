package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/oklog/ulid/v2"
)

func (r ActionItemRepository) SetActionItemRecurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, anchor time.Time, intervalWeeks int, frequencies []dao.TaskFrequency) error {
	params := sqlc.SetActionItemRecurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
		FrequencyAnchorDate: timeToPgDate(anchor), IntervalWeeks: int32(intervalWeeks),
	}
	rows, err := r.queries.SetActionItemRecurrenceByTaskAndUserID(ctx, params)
	if err != nil {
		return err
	}
	if rows == 0 {
		return usecase.ErrActionItemNotFound
	}
	owner := sqlc.ReplaceActionItemFrequenciesByTaskAndUserIDParams{SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID)}
	if err := r.queries.ReplaceActionItemFrequenciesByTaskAndUserID(ctx, owner); err != nil {
		return err
	}
	return r.queries.CreateActionItemFrequenciesByTaskAndUserID(ctx, sqlc.CreateActionItemFrequenciesByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), Frequencies: taskFrequencyValues(frequencies),
	})
}

func (r ActionItemRepository) StopActionItemRecurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID) error {
	rows, err := r.queries.StopActionItemRecurrenceByTaskAndUserID(ctx, sqlc.StopActionItemRecurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return usecase.ErrActionItemNotFound
	}
	return r.queries.ClearActionItemFrequenciesByTaskAndUserID(ctx, sqlc.ClearActionItemFrequenciesByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	})
}

func (r ActionItemRepository) UpdateActionItemSeriesTemplate(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID, snapshotID domain.ActionItemID, item domain.ActionItem) (dao.ActionItem, error) {
	owner := sqlc.SnapshotActionItemRootOccurrenceByTaskAndUserIDParams{
		ID: string(snapshotID), SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID),
	}
	if err := r.queries.SnapshotActionItemRootOccurrenceByTaskAndUserID(ctx, owner); err != nil {
		return dao.ActionItem{}, err
	}
	rows, err := r.queries.UpdateActionItemSeriesTemplateByTaskAndUserID(ctx, sqlc.UpdateActionItemSeriesTemplateByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), Title: item.Title,
		Description: stringToPgText(item.Description), DueDate: timeToPgDate(item.DueDate), Position: int32(item.Position),
	})
	if err != nil {
		return dao.ActionItem{}, err
	}
	if rows == 0 {
		return dao.ActionItem{}, usecase.ErrActionItemNotFound
	}
	return r.GetForOwnedTask(ctx, userID, taskID, seriesID)
}

func (r ActionItemRepository) UpsertActionItemOverride(ctx context.Context, userID domain.UserID, item domain.ActionItem) (string, error) {
	return r.queries.UpsertActionItemOverrideByTaskAndUserID(ctx, sqlc.UpsertActionItemOverrideByTaskAndUserIDParams{
		ID: string(item.ID), TaskID: string(item.TaskID), UserID: string(userID), SeriesID: string(item.SeriesID),
		Title: item.Title, Description: stringToPgText(item.Description), DueDate: timeToPgDate(item.DueDate),
		Completed: item.Completed, Position: int32(item.Position), OccurrenceDate: timeToPgDate(item.OccurrenceDate),
		Timezone: item.Timezone, Deleted: item.Deleted,
	})
}

func (r ActionItemRepository) ListActionItemSkippedOccurrences(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID) ([]int64, error) {
	rows, err := r.queries.ListActionItemSkippedOccurrencesByTaskAndUserID(ctx, sqlc.ListActionItemSkippedOccurrencesByTaskAndUserIDParams{
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

func (r ActionItemRepository) ListActionItemSkippedOccurrencesForCapability(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, capability shared.Capability) ([]int64, error) {
	rows, err := r.queries.ListActionItemSkippedOccurrencesForCapabilityByTaskAndUserID(ctx, sqlc.ListActionItemSkippedOccurrencesForCapabilityByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), ActorID: string(userID),
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
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

func (r ActionItemRepository) SetActionItemSkippedOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, occurrenceDate time.Time, skipped bool) error {
	date := timeToPgDate(occurrenceDate)
	owner := sqlc.SkipActionItemOccurrenceByTaskAndUserIDParams{
		ID: ulid.Make().String(), SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), OccurrenceDate: date,
	}
	if skipped {
		_, err := r.queries.SkipActionItemOccurrenceByTaskAndUserID(ctx, owner)
		return err
	}
	restore := sqlc.RestoreEditedActionItemOccurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), OccurrenceDate: date,
	}
	if err := r.queries.RestoreEditedActionItemOccurrenceByTaskAndUserID(ctx, restore); err != nil {
		return err
	}
	return r.queries.DeleteSkippedActionItemOccurrenceByTaskAndUserID(ctx, sqlc.DeleteSkippedActionItemOccurrenceByTaskAndUserIDParams{
		SeriesID: string(seriesID), TaskID: string(taskID), UserID: string(userID), OccurrenceDate: date,
	})
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
