package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/shared"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
)

type TaskScheduleCreateParams struct {
	UserID          domain.UserID
	TaskID          domain.TaskID
	Title           string
	Description     string
	Location        string
	IntervalWeeks   *int
	FrequencyValues []string
	StartAt         time.Time
	EndAt           time.Time
}

type TaskScheduleUpdateParams struct {
	UserID          domain.UserID
	TaskID          domain.TaskID
	ID              domain.TaskScheduleID
	Title           *string
	Description     *string
	Location        *string
	IntervalWeeks   *int
	FrequencyValues *[]string
	StartAt         *time.Time
	EndAt           *time.Time
}

type TaskScheduleService struct {
	repo TaskScheduleRepository
}

func NewTaskScheduleService(repo TaskScheduleRepository) *TaskScheduleService {
	return &TaskScheduleService{
		repo: repo,
	}
}

func (s *TaskScheduleService) Create(ctx context.Context, idGen func() string, params TaskScheduleCreateParams) (domain.TaskSchedule, error) {
	id, err := shared.NewID(idGen())
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}

	intervalWeeks := domain.OnceIntervalWeeks
	if params.IntervalWeeks != nil {
		intervalWeeks = *params.IntervalWeeks
	}

	frequencies, err := taskFrequenciesFromValues(params.FrequencyValues)
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}

	schedule, err := domain.NewTaskScheduleWithDetails(
		domain.TaskScheduleID(id),
		params.TaskID,
		params.Title,
		params.Description,
		params.Location,
		intervalWeeks,
		frequencies,
		params.StartAt,
		params.EndAt,
	)
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}

	return s.repo.CreateByTaskAndUser(ctx, params.UserID, schedule)
}

func (s *TaskScheduleService) Update(ctx context.Context, params TaskScheduleUpdateParams) (domain.TaskSchedule, error) {
	existing, err := s.repo.GetByTaskAndUser(ctx, params.UserID, params.TaskID, params.ID)
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
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
			return domain.NewZeroTaskSchedule(), err
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

	updated, err := domain.NewExistingTaskSchedule(
		existing.ID,
		existing.TaskID,
		title,
		description,
		location,
		intervalWeeks,
		frequencies,
		startAt,
		endAt,
		existing.CreatedAt,
		existing.UpdatedAt,
	)
	if err != nil {
		return domain.NewZeroTaskSchedule(), err
	}

	return s.repo.UpdateByTaskAndUser(ctx, params.UserID, updated)
}

func (s *TaskScheduleService) Delete(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) error {
	return s.repo.DeleteByTaskAndUser(ctx, userID, taskID, id)
}

func taskFrequenciesFromValues(values []string) (domain.TaskFrequencies, error) {
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
