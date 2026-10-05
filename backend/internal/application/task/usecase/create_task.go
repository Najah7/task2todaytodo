package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
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

type createTaskRepository interface {
	Create(ctx context.Context, task domain.Task) (dao.Task, error)
}

type CreateTaskUseCase struct {
	repo   createTaskRepository
	logger logging.Logger
}

func NewCreateTaskUseCase(repo createTaskRepository, logger logging.Logger) *CreateTaskUseCase {
	return &CreateTaskUseCase{logger: logging.OrNop(logger), repo: repo}
}

func (uc *CreateTaskUseCase) Execute(ctx context.Context, input CreateTaskInput) (output dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CreateTaskUseCase.Execute", err) }()

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

	return uc.repo.Create(ctx, task)

}
