package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/shared"
)

var (
	ErrTaskIDEmpty                 = errors.New("task ID cannot be empty")
	ErrTaskUserIDEmpty             = errors.New("task user ID cannot be empty")
	ErrTaskTitleEmpty              = errors.New("task title cannot be empty")
	ErrTaskEstimatedMinutesInvalid = errors.New("task estimated minutes must be greater than or equal to 0")
	ErrTaskActualMinutesInvalid    = errors.New("task actual minutes must be greater than or equal to 0")
	ErrTaskProgressInvalid         = errors.New("task progress must be between 0 and 100")
)

type UserID shared.ID
type ProjectID shared.ID
type TaskID shared.ID

type Task struct {
	ID               TaskID
	UserID           UserID
	ProjectID        ProjectID
	Title            string
	Description      string
	DueDate          time.Time
	EstimatedMinutes *int
	ActualMinutes    *int
	Progress         int
	Priority         TaskPriority
	Status           TaskStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewTask(
	id TaskID,
	userID UserID,
	title string,
) (Task, error) {
	task := Task{
		ID:     id,
		UserID: userID,
		Title:  title,
	}
	return task, task.Validate()
}

func NewTaskWithDetails(
	id TaskID,
	userID UserID,
	projectID ProjectID,
	title string,
	description string,
	dueDate time.Time,
	estimatedMinutes *int,
	actualMinutes *int,
	progress int,
	priority TaskPriority,
	status TaskStatus,
) (Task, error) {
	task := Task{
		ID:               id,
		UserID:           userID,
		ProjectID:        projectID,
		Title:            title,
		Description:      description,
		DueDate:          dueDate,
		EstimatedMinutes: estimatedMinutes,
		ActualMinutes:    actualMinutes,
		Progress:         progress,
		Priority:         priority,
		Status:           status,
	}
	return task, task.Validate()
}

func NewExistingTask(
	id TaskID,
	userID UserID,
	projectID ProjectID,
	title string,
	description string,
	dueDate time.Time,
	estimatedMinutes *int,
	actualMinutes *int,
	progress int,
	priority TaskPriority,
	status TaskStatus,
	createdAt time.Time,
	updatedAt time.Time,
) (Task, error) {
	task := Task{
		ID:               id,
		UserID:           userID,
		ProjectID:        projectID,
		Title:            title,
		Description:      description,
		DueDate:          dueDate,
		EstimatedMinutes: estimatedMinutes,
		ActualMinutes:    actualMinutes,
		Progress:         progress,
		Priority:         priority,
		Status:           status,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}
	if err := task.Validate(); err != nil {
		return NewZeroTask(), err
	}

	return task, nil
}

func NewZeroTask() Task {
	return Task{}
}

func (t Task) IsZero() bool {
	return t.ID == ""
}

func (t Task) Validate() error {
	if t.ID == "" {
		return ErrTaskIDEmpty
	}
	if t.UserID == "" {
		return ErrTaskUserIDEmpty
	}
	if strings.TrimSpace(t.Title) == "" {
		return ErrTaskTitleEmpty
	}
	if t.EstimatedMinutes != nil && *t.EstimatedMinutes < 0 {
		return ErrTaskEstimatedMinutesInvalid
	}
	if t.ActualMinutes != nil && *t.ActualMinutes < 0 {
		return ErrTaskActualMinutesInvalid
	}
	if t.Progress < 0 || t.Progress > 100 {
		return ErrTaskProgressInvalid
	}
	if t.Priority != (TaskPriority{}) {
		if err := t.Priority.validate(); err != nil {
			return err
		}
	}
	if t.Status != (TaskStatus{}) {
		if err := t.Status.validate(); err != nil {
			return err
		}
	}

	return nil
}
