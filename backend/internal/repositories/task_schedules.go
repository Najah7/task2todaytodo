package repositories

import (
	"context"

	domain "github.com/Najah7/task2schedule/internal/domain/task"
	"github.com/Najah7/task2schedule/internal/repositories/sqlc"
)

var _ domain.TaskScheduleRepository = TaskScheduleRepository{}

type TaskScheduleRepository struct {
	queries *sqlc.Queries
}

func NewTaskScheduleRepository(db sqlc.DBTX) *TaskScheduleRepository {
	return &TaskScheduleRepository{
		queries: sqlc.New(db),
	}
}

func (r TaskScheduleRepository) Get(ctx context.Context, id domain.TaskScheduleID) (domain.TaskSchedule, error) {
	record, err := r.queries.GetTaskSchedule(ctx, string(id))
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}
	return recordToTaskSchedule(record)
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
		DueAt:         timeToPgTime(schedule.DueAt),
	})
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}
	return recordToTaskSchedule(record)
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
		DueAt:         timeToPgTime(schedule.DueAt),
	})
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}
	return recordToTaskSchedule(record)
}

func (r TaskScheduleRepository) Delete(ctx context.Context, id domain.TaskScheduleID) error {
	return r.queries.DeleteTaskSchedule(ctx, string(id))
}
