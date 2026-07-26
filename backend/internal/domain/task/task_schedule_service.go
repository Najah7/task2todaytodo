package task

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/domain/shared"
)

type TaskScheduleCreateParams struct {
	UserID          UserID
	TaskID          TaskID
	Title           string
	Description     string
	Location        string
	IntervalWeeks   *int
	FrequencyValues []string
	StartAt         time.Time
	EndAt           time.Time
	DueAt           time.Time
}

type TaskScheduleUpdateParams struct {
	UserID          UserID
	TaskID          TaskID
	ID              TaskScheduleID
	Title           *string
	Description     *string
	Location        *string
	IntervalWeeks   *int
	FrequencyValues *[]string
	StartAt         *time.Time
	EndAt           *time.Time
	DueAt           *time.Time
}

type TaskScheduleService struct {
	repo TaskScheduleRepository
}

func NewTaskScheduleService(repo TaskScheduleRepository) *TaskScheduleService {
	return &TaskScheduleService{
		repo: repo,
	}
}

func (s *TaskScheduleService) Create(ctx context.Context, idGen func() string, params TaskScheduleCreateParams) (TaskSchedule, error) {
	id, err := shared.NewID(idGen())
	if err != nil {
		return NewZeroTaskSchedule(), err
	}

	intervalWeeks := OnceIntervalWeeks
	if params.IntervalWeeks != nil {
		intervalWeeks = *params.IntervalWeeks
	}

	frequencies, err := taskFrequenciesFromValues(params.FrequencyValues)
	if err != nil {
		return NewZeroTaskSchedule(), err
	}

	schedule, err := NewTaskScheduleWithDetails(
		TaskScheduleID(id),
		params.TaskID,
		params.Title,
		params.Description,
		params.Location,
		intervalWeeks,
		frequencies,
		params.StartAt,
		params.EndAt,
		params.DueAt,
	)
	if err != nil {
		return NewZeroTaskSchedule(), err
	}

	return s.repo.CreateByTaskAndUser(ctx, params.UserID, schedule)
}

func (s *TaskScheduleService) Update(ctx context.Context, params TaskScheduleUpdateParams) (TaskSchedule, error) {
	existing, err := s.repo.GetByTaskAndUser(ctx, params.UserID, params.TaskID, params.ID)
	if err != nil {
		return NewZeroTaskSchedule(), err
	}

	title := existing.Title
	if params.Title != nil {
		title = *params.Title
	}

	description := existing.Description
	if params.Description != nil {
		description = *params.Description
	}

	location := existing.Location
	if params.Location != nil {
		location = *params.Location
	}

	intervalWeeks := existing.IntervalWeeks
	if params.IntervalWeeks != nil {
		intervalWeeks = *params.IntervalWeeks
	}

	frequencies := existing.Frequencies
	if params.FrequencyValues != nil {
		frequencies, err = taskFrequenciesFromValues(*params.FrequencyValues)
		if err != nil {
			return NewZeroTaskSchedule(), err
		}
	}

	startAt := existing.StartAt
	if params.StartAt != nil {
		startAt = *params.StartAt
	}

	endAt := existing.EndAt
	if params.EndAt != nil {
		endAt = *params.EndAt
	}

	dueAt := existing.DueAt
	if params.DueAt != nil {
		dueAt = *params.DueAt
	}

	updated, err := NewExistingTaskSchedule(
		existing.ID,
		existing.TaskID,
		title,
		description,
		location,
		intervalWeeks,
		frequencies,
		startAt,
		endAt,
		dueAt,
		existing.CreatedAt,
		existing.UpdatedAt,
	)
	if err != nil {
		return NewZeroTaskSchedule(), err
	}

	return s.repo.UpdateByTaskAndUser(ctx, params.UserID, updated)
}

func (s *TaskScheduleService) Delete(ctx context.Context, userID UserID, taskID TaskID, id TaskScheduleID) error {
	return s.repo.DeleteByTaskAndUser(ctx, userID, taskID, id)
}

func taskFrequenciesFromValues(values []string) (TaskFrequencies, error) {
	frequencies := make(TaskFrequencies, 0, len(values))
	for _, value := range values {
		frequency, err := NewTaskFrequency(value)
		if err != nil {
			return nil, err
		}
		frequencies = append(frequencies, frequency)
	}
	return frequencies, nil
}
