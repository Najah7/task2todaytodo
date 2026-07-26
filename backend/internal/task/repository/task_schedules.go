package repository

import (
	"context"
	"errors"

	usecase "github.com/Najah7/task2todaytodo/internal/task/usecase"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	"github.com/jackc/pgx/v5"
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

func (r TaskScheduleRepository) Get(ctx context.Context, id domain.TaskScheduleID) (domain.TaskSchedule, error) {
	record, err := r.queries.GetTaskSchedule(ctx, string(id))
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}
	return recordToTaskScheduleRow(record)
}

func (r TaskScheduleRepository) GetByTaskAndUser(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) (domain.TaskSchedule, error) {
	record, err := r.queries.GetTaskScheduleByTaskAndUser(ctx, sqlc.GetTaskScheduleByTaskAndUserParams{
		ID:     string(id),
		TaskID: string(taskID),
		UserID: string(userID),
	})
	if err != nil {
		return domain.NewZeroTaskSchedule(), taskScheduleNotFoundError(err)
	}
	return recordToTaskScheduleByTaskAndUserRow(record)
}

func (r TaskScheduleRepository) Create(ctx context.Context, schedule domain.TaskSchedule) (domain.TaskSchedule, error) {
	record, err := r.queries.CreateTaskSchedule(ctx, sqlc.CreateTaskScheduleParams{
		ID:            string(schedule.ID),
		TaskID:        string(schedule.TaskID),
		Title:         schedule.Title,
		Description:   stringToPgText(schedule.Description),
		Location:      stringToPgText(schedule.Location),
		IntervalWeeks: int32(schedule.IntervalWeeks),
		StartAt:       timeToPgTime(schedule.StartAt),
		EndAt:         timeToPgTime(schedule.EndAt),
	})
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}
	return recordToTaskSchedule(record)
}

func (r TaskScheduleRepository) CreateByTaskAndUser(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (domain.TaskSchedule, error) {
	record, err := r.queries.CreateTaskScheduleByTaskAndUser(ctx, sqlc.CreateTaskScheduleByTaskAndUserParams{
		ID:            string(schedule.ID),
		TaskID:        string(schedule.TaskID),
		UserID:        string(userID),
		Title:         schedule.Title,
		Description:   stringToPgText(schedule.Description),
		Location:      stringToPgText(schedule.Location),
		IntervalWeeks: int32(schedule.IntervalWeeks),
		StartAt:       timeToPgTime(schedule.StartAt),
		EndAt:         timeToPgTime(schedule.EndAt),
		Frequencies:   taskFrequencyStrings(schedule.Frequencies),
	})
	if err != nil {
		return domain.NewZeroTaskSchedule(), taskScheduleTaskNotFoundError(err)
	}
	return recordToCreatedTaskScheduleByTaskAndUserRow(record)
}

func (r TaskScheduleRepository) Update(ctx context.Context, schedule domain.TaskSchedule) (domain.TaskSchedule, error) {
	record, err := r.queries.UpdateTaskSchedule(ctx, sqlc.UpdateTaskScheduleParams{
		ID:            string(schedule.ID),
		TaskID:        string(schedule.TaskID),
		Title:         schedule.Title,
		Description:   stringToPgText(schedule.Description),
		Location:      stringToPgText(schedule.Location),
		IntervalWeeks: int32(schedule.IntervalWeeks),
		StartAt:       timeToPgTime(schedule.StartAt),
		EndAt:         timeToPgTime(schedule.EndAt),
	})
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}
	return recordToTaskSchedule(record)
}

func (r TaskScheduleRepository) UpdateByTaskAndUser(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (domain.TaskSchedule, error) {
	record, err := r.queries.UpdateTaskScheduleByTaskAndUser(ctx, sqlc.UpdateTaskScheduleByTaskAndUserParams{
		ID:            string(schedule.ID),
		TaskID:        string(schedule.TaskID),
		UserID:        string(userID),
		Title:         schedule.Title,
		Description:   stringToPgText(schedule.Description),
		Location:      stringToPgText(schedule.Location),
		IntervalWeeks: int32(schedule.IntervalWeeks),
		StartAt:       timeToPgTime(schedule.StartAt),
		EndAt:         timeToPgTime(schedule.EndAt),
		Frequencies:   taskFrequencyStrings(schedule.Frequencies),
	})
	if err != nil {
		return domain.NewZeroTaskSchedule(), taskScheduleNotFoundError(err)
	}
	return recordToUpdatedTaskScheduleByTaskAndUserRow(record)
}

func (r TaskScheduleRepository) Delete(ctx context.Context, id domain.TaskScheduleID) error {
	return r.queries.DeleteTaskSchedule(ctx, string(id))
}

func (r TaskScheduleRepository) DeleteByTaskAndUser(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) error {
	_, err := r.queries.DeleteTaskScheduleByTaskAndUser(ctx, sqlc.DeleteTaskScheduleByTaskAndUserParams{
		ID:     string(id),
		TaskID: string(taskID),
		UserID: string(userID),
	})
	return taskScheduleNotFoundError(err)
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
