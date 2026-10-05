package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.TaskScheduleRepository = TaskScheduleRepository{}

type TaskScheduleRepository struct {
	queries *sqlc.Queries
}

func NewTaskScheduleRepository(db sqlc.DBTX) *TaskScheduleRepository {
	return &TaskScheduleRepository{
		queries: sqlc.New(db),
	}
}

func (r *TaskScheduleRepository) WithTx(tx pgx.Tx) *TaskScheduleRepository {
	return &TaskScheduleRepository{
		queries: r.queries.WithTx(tx),
	}
}

func (r TaskScheduleRepository) Get(ctx context.Context, id domain.TaskScheduleID) (dao.TaskSchedule, error) {
	record, err := r.queries.GetTaskSchedule(ctx, string(id))
	if err != nil {
		return dao.TaskSchedule{}, err
	}
	return recordToTaskScheduleRow(record), nil
}

func (r TaskScheduleRepository) GetByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) (dao.TaskSchedule, error) {
	record, err := r.queries.GetTaskScheduleByTaskAndUserID(ctx, sqlc.GetTaskScheduleByTaskAndUserIDParams{
		ID:       string(id),
		TaskID:   string(taskID),
		ActorKey: string(userID),
	})
	if err != nil {
		return dao.TaskSchedule{}, taskScheduleNotFoundError(err)
	}
	return recordToTaskScheduleByTaskAndUserIDRow(record), nil
}

func (r TaskScheduleRepository) GetForCommand(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID, capability shared.Capability) (dao.TaskSchedule, error) {
	record, err := r.queries.GetTaskScheduleByTaskAndUserIDForCommand(ctx, sqlc.GetTaskScheduleByTaskAndUserIDForCommandParams{
		ID:         string(id),
		TaskID:     string(taskID),
		ActorID:    string(userID),
		ResourceID: string(capability.Resource),
		Action:     sqlc.PermissionAction(capability.Action),
	})
	if err != nil {
		return dao.TaskSchedule{}, taskScheduleNotFoundError(err)
	}
	schedule := taskScheduleDAO(record.ID, record.TaskID, record.Title, record.Description, record.Location, record.IntervalWeeks, record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException, sqlcBoolean(record.Deleted), record.Frequencies, record.StartAt, record.EndAt, record.CreatedAt, record.UpdatedAt, record.Completed)
	if record.RepeatState.Valid {
		schedule.RepeatState = record.RepeatState.String
	}
	schedule.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
	return schedule, nil
}

func (r TaskScheduleRepository) ListByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TaskSchedule, error) {
	records, err := r.queries.ListTaskSchedulesByTaskAndUserID(ctx, sqlc.ListTaskSchedulesByTaskAndUserIDParams{
		TaskID:   string(taskID),
		ActorKey: string(userID),
	})
	if err != nil {
		return nil, err
	}
	return recordsToTaskSchedulesByTaskAndUserIDRows(records), nil
}

// ListByTaskForOccurrenceProjection includes deleted rows so callers can suppress tombstoned virtual dates.
func (r TaskScheduleRepository) ListByTaskForOccurrenceProjection(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TaskSchedule, error) {
	records, err := r.queries.ListTaskSchedulesForOccurrenceProjectionByTaskAndUserID(ctx, sqlc.ListTaskSchedulesForOccurrenceProjectionByTaskAndUserIDParams{
		TaskID:   string(taskID),
		ActorKey: string(userID),
	})
	if err != nil {
		return nil, err
	}
	schedules := make([]dao.TaskSchedule, 0, len(records))
	for _, row := range records {
		schedule := taskScheduleDAO(row.ID, row.TaskID, row.Title, row.Description, row.Location, row.IntervalWeeks, row.SeriesID, row.OccurrenceDate, row.Timezone, row.IsException, sqlcBoolean(row.Deleted), row.Frequencies, row.StartAt, row.EndAt, row.CreatedAt, row.UpdatedAt, row.Completed)
		if row.RepeatState.Valid {
			schedule.RepeatState = row.RepeatState.String
		}
		schedule.FrequencyAnchorDate = pgDateUnix(row.FrequencyAnchorDate)
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

func (r TaskScheduleRepository) ListByTaskForOccurrenceCommand(ctx context.Context, userID domain.UserID, taskID domain.TaskID, capability shared.Capability) ([]dao.TaskSchedule, error) {
	records, err := r.queries.ListTaskSchedulesForOccurrenceCommandByTaskAndUserID(ctx, sqlc.ListTaskSchedulesForOccurrenceCommandByTaskAndUserIDParams{
		TaskID:     string(taskID),
		ActorID:    string(userID),
		ResourceID: string(capability.Resource),
		Action:     sqlc.PermissionAction(capability.Action),
	})
	if err != nil {
		return nil, err
	}
	schedules := make([]dao.TaskSchedule, 0, len(records))
	for _, row := range records {
		schedule := taskScheduleDAO(row.ID, row.TaskID, row.Title, row.Description, row.Location, row.IntervalWeeks, row.SeriesID, row.OccurrenceDate, row.Timezone, row.IsException, sqlcBoolean(row.Deleted), row.Frequencies, row.StartAt, row.EndAt, row.CreatedAt, row.UpdatedAt, row.Completed)
		if row.RepeatState.Valid {
			schedule.RepeatState = row.RepeatState.String
		}
		schedule.FrequencyAnchorDate = pgDateUnix(row.FrequencyAnchorDate)
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

func (r TaskScheduleRepository) ListByTaskAndUserIDCursor(ctx context.Context, userID domain.UserID, taskID domain.TaskID, limit int, anchor *usecase.CursorAnchor) ([]dao.TaskSchedule, error) {
	arg := sqlc.ListTaskSchedulesByTaskAndUserIDCursorPageParams{TaskID: string(taskID), ActorKey: string(userID), PageLimit: int32(limit)}
	if anchor != nil {
		at, err := time.Parse(time.RFC3339Nano, anchor.At)
		if err != nil {
			return nil, usecase.ErrInvalidTaskPage
		}
		arg.CursorAt = pgtype.Timestamptz{Time: at, Valid: true}
		arg.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	records, err := r.queries.ListTaskSchedulesByTaskAndUserIDCursorPage(ctx, arg)
	if err != nil {
		return nil, err
	}
	schedules := make([]dao.TaskSchedule, 0, len(records))
	for _, row := range records {
		s := taskScheduleDAO(row.ID, row.TaskID, row.Title, row.Description, row.Location, row.IntervalWeeks, row.SeriesID, row.OccurrenceDate, row.Timezone, row.IsException, sqlcBoolean(row.Deleted), row.Frequencies, row.StartAt, row.EndAt, row.CreatedAt, row.UpdatedAt, row.Completed)
		if row.RepeatState.Valid {
			s.RepeatState = row.RepeatState.String
		}
		s.FrequencyAnchorDate = pgDateUnix(row.FrequencyAnchorDate)
		schedules = append(schedules, s)
	}
	return schedules, nil
}

// ListActiveSeriesByUserID returns active recurrence roots for daily refill.
func (r TaskScheduleRepository) ListActiveSeriesByUserID(ctx context.Context, userID domain.UserID) ([]dao.TaskSchedule, error) {
	records, err := r.queries.ListActiveTaskScheduleSeriesByUserID(ctx, string(userID))
	if err != nil {
		return nil, err
	}
	return recordsToActiveTaskScheduleSeriesByUserIDRows(records), nil
}

func (r TaskScheduleRepository) Create(ctx context.Context, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	var err error
	schedule, err = domain.NewTaskScheduleWithRecurrence(schedule)
	if err != nil {
		return dao.TaskSchedule{}, err
	}
	record, err := r.queries.CreateTaskSchedule(ctx, sqlc.CreateTaskScheduleParams{
		ID:             string(schedule.ID),
		TaskID:         string(schedule.TaskID),
		Title:          schedule.Title,
		Description:    stringToPgText(schedule.Description),
		Location:       stringToPgText(schedule.Location),
		IntervalWeeks:  int32(schedule.IntervalWeeks),
		SeriesID:       taskScheduleSeriesID(schedule),
		OccurrenceDate: taskScheduleOccurrenceDate(schedule),
		Timezone:       taskScheduleTimezone(schedule),
		IsException:    schedule.IsException,
		StartAt:        timeToPgTime(schedule.StartAt),
		EndAt:          timeToPgTime(schedule.EndAt),
	})
	if err != nil {
		return dao.TaskSchedule{}, err
	}
	return taskScheduleDAO(record.ID, record.TaskID, record.Title, record.Description, record.Location, record.IntervalWeeks, record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException, sqlcBoolean(record.Deleted), nil, record.StartAt, record.EndAt, record.CreatedAt, record.UpdatedAt, record.Completed), nil
}

func (r TaskScheduleRepository) CreateByTaskAndUserID(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	var err error
	schedule, err = domain.NewTaskScheduleWithRecurrence(schedule)
	if err != nil {
		return dao.TaskSchedule{}, err
	}
	record, err := r.queries.CreateTaskScheduleByTaskAndUserID(ctx, sqlc.CreateTaskScheduleByTaskAndUserIDParams{
		ID:             string(schedule.ID),
		TaskID:         string(schedule.TaskID),
		UserID:         string(userID),
		Title:          schedule.Title,
		Description:    stringToPgText(schedule.Description),
		Location:       stringToPgText(schedule.Location),
		IntervalWeeks:  int32(schedule.IntervalWeeks),
		SeriesID:       taskScheduleSeriesID(schedule),
		OccurrenceDate: taskScheduleOccurrenceDate(schedule),
		Timezone:       taskScheduleTimezone(schedule),
		IsException:    schedule.IsException,
		StartAt:        timeToPgTime(schedule.StartAt),
		EndAt:          timeToPgTime(schedule.EndAt),
		Frequencies:    taskFrequencyStrings(schedule.Frequencies),
	})
	if err != nil {
		return dao.TaskSchedule{}, taskScheduleTaskNotFoundError(err)
	}
	return recordToCreatedTaskScheduleByTaskAndUserIDRow(record), nil
}

// CreateOccurrenceByTaskAndUserID creates one generated occurrence idempotently.
func (r TaskScheduleRepository) CreateOccurrenceByTaskAndUserID(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	var err error
	schedule, err = domain.NewTaskScheduleWithRecurrence(schedule)
	if err != nil {
		return dao.TaskSchedule{}, err
	}
	record, err := r.queries.CreateTaskScheduleOccurrenceByTaskAndUserID(ctx, sqlc.CreateTaskScheduleOccurrenceByTaskAndUserIDParams{
		ID:             string(schedule.ID),
		TaskID:         string(schedule.TaskID),
		UserID:         string(userID),
		SeriesID:       taskScheduleSeriesID(schedule),
		OccurrenceDate: taskScheduleOccurrenceDate(schedule),
		Timezone:       taskScheduleTimezone(schedule),
		IsException:    schedule.IsException,
		StartAt:        timeToPgTime(schedule.StartAt),
		EndAt:          timeToPgTime(schedule.EndAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Task is no longer eligible or the occurrence was tombstoned already.
			return dao.TaskSchedule{}, nil
		}
		return dao.TaskSchedule{}, taskScheduleTaskNotFoundError(err)
	}
	return recordToCreatedTaskScheduleOccurrenceByTaskAndUserIDRow(record), nil
}

func (r TaskScheduleRepository) Update(ctx context.Context, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	record, err := r.queries.UpdateTaskSchedule(ctx, sqlc.UpdateTaskScheduleParams{
		ID:          string(schedule.ID),
		TaskID:      string(schedule.TaskID),
		Title:       schedule.Title,
		Description: stringToPgText(schedule.Description),
		Location:    stringToPgText(schedule.Location),
		StartAt:     timeToPgTime(schedule.StartAt),
		EndAt:       timeToPgTime(schedule.EndAt),
	})
	if err != nil {
		return dao.TaskSchedule{}, err
	}
	return taskScheduleDAO(record.ID, record.TaskID, record.Title, record.Description, record.Location, record.IntervalWeeks, record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException, sqlcBoolean(record.Deleted), nil, record.StartAt, record.EndAt, record.CreatedAt, record.UpdatedAt, record.Completed), nil
}

func (r TaskScheduleRepository) UpdateByTaskAndUserID(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	record, err := r.queries.UpdateTaskScheduleByTaskAndUserID(ctx, sqlc.UpdateTaskScheduleByTaskAndUserIDParams{
		ID:          string(schedule.ID),
		TaskID:      string(schedule.TaskID),
		UserID:      string(userID),
		Title:       schedule.Title,
		Description: stringToPgText(schedule.Description),
		Location:    stringToPgText(schedule.Location),
		StartAt:     timeToPgTime(schedule.StartAt),
		EndAt:       timeToPgTime(schedule.EndAt),
	})
	if err != nil {
		return dao.TaskSchedule{}, taskScheduleNotFoundError(err)
	}
	return recordToUpdatedTaskScheduleByTaskAndUserIDRow(record), nil
}

func (r TaskScheduleRepository) SetCompletedForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID, completed bool) error {
	rows, err := r.queries.SetTaskScheduleCompletedByTaskAndUserID(ctx, sqlc.SetTaskScheduleCompletedByTaskAndUserIDParams{
		ID: string(id), TaskID: string(taskID), UserID: string(userID), Completed: completed,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrTaskScheduleNotFound
	}
	return nil
}

func (r TaskScheduleRepository) Delete(ctx context.Context, id domain.TaskScheduleID) error {
	return r.queries.DeleteTaskSchedule(ctx, string(id))
}

func (r TaskScheduleRepository) DeleteByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) error {
	_, err := r.queries.TombstoneTaskScheduleByTaskAndUserID(ctx, sqlc.TombstoneTaskScheduleByTaskAndUserIDParams{
		ID:     string(id),
		TaskID: string(taskID),
		UserID: string(userID),
	})
	return taskScheduleNotFoundError(err)
}

func (r TaskScheduleRepository) TombstoneByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) error {
	_, err := r.queries.TombstoneTaskScheduleByTaskAndUserID(ctx, sqlc.TombstoneTaskScheduleByTaskAndUserIDParams{
		ID:     string(id),
		TaskID: string(taskID),
		UserID: string(userID),
	})
	return taskScheduleNotFoundError(err)
}

func (r TaskScheduleRepository) DeleteUneditedFutureBySeries(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, fromAt time.Time) error {
	_, err := r.queries.DeleteUneditedFutureTaskSchedulesBySeries(ctx, sqlc.DeleteUneditedFutureTaskSchedulesBySeriesParams{
		UserID:   string(userID),
		TaskID:   string(taskID),
		SeriesID: string(seriesID),
		FromAt:   timeToPgTime(fromAt),
	})
	return err
}

func (r TaskScheduleRepository) DeleteUneditedFutureByTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, fromAt time.Time) error {
	_, err := r.queries.DeleteUneditedFutureTaskSchedulesByTask(ctx, sqlc.DeleteUneditedFutureTaskSchedulesByTaskParams{
		UserID: string(userID),
		TaskID: string(taskID),
		FromAt: timeToPgTime(fromAt),
	})
	return err
}

func taskScheduleSeriesID(schedule domain.TaskSchedule) string {
	return string(schedule.SeriesID)
}

func taskScheduleOccurrenceDate(schedule domain.TaskSchedule) pgtype.Date {
	return timeToPgDate(schedule.OccurrenceDate)
}

func taskScheduleTimezone(schedule domain.TaskSchedule) string {
	return schedule.Timezone
}

func taskScheduleNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTaskScheduleNotFound
	}
	return err
}

func taskScheduleTaskNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTaskScheduleTaskNotFound
	}
	return err
}
