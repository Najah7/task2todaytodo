package usecase

import (
	"context"
	"math"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type CreateTaskInput struct {
	ID               domain.TaskID
	UserID           domain.UserID
	Title            string
	Description      string
	DueDate          time.Time
	EstimatedMinutes *int
	Priority         string
}

type CreateTaskUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewCreateTaskUseCase(uow UOW, logger telemetry.Logger) *CreateTaskUseCase {
	return &CreateTaskUseCase{logger: telemetry.OrNop(logger), uow: uow}
}

func (uc *CreateTaskUseCase) Execute(ctx context.Context, input CreateTaskInput) (output dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CreateTaskUseCase.Execute", err) }()
	if input.EstimatedMinutes == nil {
		return dao.Task{}, ErrTaskManualEstimateRequired
	}
	if *input.EstimatedMinutes < 1 || int64(*input.EstimatedMinutes) > math.MaxInt32 {
		return dao.Task{}, domain.ErrTaskEstimatedMinutesInvalid
	}

	priorityValue := input.Priority
	if priorityValue == "" {
		priorityValue = "low"
	}
	priority, err := domain.NewTaskPriority(priorityValue)
	if err != nil {
		return dao.Task{}, err
	}

	status, err := domain.NewTaskStatus("open")
	if err != nil {
		return dao.Task{}, err
	}

	var task domain.Task
	hasDetails := input.Description != "" || !input.DueDate.IsZero() || input.EstimatedMinutes != nil || input.Priority != ""
	if hasDetails {
		task, err = domain.NewTaskWithDetails(
			input.ID,
			input.UserID,
			"",
			input.Title,
			input.Description,
			input.DueDate,
			input.EstimatedMinutes,
			nil,
			0,
			priority,
			status,
		)
	} else {
		task, err = domain.NewTask(input.ID, input.UserID, input.Title)
	}
	if err != nil {
		return dao.Task{}, err
	}

	asOf := time.Now()
	var created dao.Task
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		created, err = repos.Tasks().Create(ctx, task)
		if err != nil {
			return err
		}
		created.CanUpdate, err = repos.Tasks().HasPermission(ctx, input.UserID, input.ID, shared.TaskUpdate())
		if err != nil {
			return err
		}
		rows, err := EnrichTasksInRepositories(ctx, repos, input.UserID, []dao.Task{created}, asOf)
		if err != nil {
			return err
		}
		created = rows[0]
		return nil
	})
	if err != nil {
		return dao.Task{}, err
	}
	return created, nil

}
