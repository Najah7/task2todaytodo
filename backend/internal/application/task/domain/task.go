package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrTaskIDEmpty                 = errors.New("task ID cannot be empty")
	ErrTaskUserIDEmpty             = errors.New("task user ID cannot be empty")
	ErrTaskAssigneeEmpty           = errors.New("task assignee ID cannot be empty")
	ErrTaskTitleEmpty              = errors.New("task title cannot be empty")
	ErrTaskEstimatedMinutesInvalid = errors.New("task estimated minutes must be greater than or equal to 0")
	ErrTaskActualMinutesInvalid    = errors.New("task actual minutes must be greater than or equal to 0")
	ErrTaskProgressInvalid         = errors.New("task progress must be between 0 and 100")
)

type UserID string
type ProjectID string
type TaskID string

type Task struct {
	ID               TaskID
	UserID           UserID
	AssigneeID       UserID
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
		ID:         id,
		UserID:     userID,
		AssigneeID: userID,
		Title:      title,
		Priority:   taskPriorities["low"],
		Status:     taskStatuses[taskStatusOpen],
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
	if priority == (TaskPriority{}) {
		priority = taskPriorities["low"]
	}
	if status == (TaskStatus{}) {
		status = taskStatuses[taskStatusOpen]
	}
	task := Task{
		ID:               id,
		UserID:           userID,
		AssigneeID:       userID,
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
	assigneeID UserID,
	createdAt time.Time,
	updatedAt time.Time,
) (Task, error) {
	task := Task{
		ID:               id,
		UserID:           userID,
		AssigneeID:       assigneeID,
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

func (t Task) Start() Task {
	return t.withStatus(taskStatusInProgress)
}

func (t Task) Hold() Task {
	return t.withStatus(taskStatusPending)
}

func (t Task) Wait() Task {
	return t.withStatus(taskStatusWaitingOnOthers)
}

func (t Task) Complete() Task {
	return t.withStatus(taskStatusDone)
}

func (t Task) Reopen() Task {
	return t.withStatus(taskStatusOpen)
}

// AssignTo returns a task assigned to a non-empty user ID. Project membership
// eligibility is checked by the application repository inside its transaction.
func (t Task) AssignTo(assigneeID UserID) (Task, error) {
	if assigneeID == "" {
		return t, ErrTaskAssigneeEmpty
	}
	t.AssigneeID = assigneeID
	return t, nil
}

func (t Task) withStatus(status string) Task {
	t.Status = taskStatuses[status]
	return t
}

func (t Task) Validate() error {
	if t.ID == "" {
		return ErrTaskIDEmpty
	}
	if t.UserID == "" {
		return ErrTaskUserIDEmpty
	}
	if t.AssigneeID == "" {
		return ErrTaskAssigneeEmpty
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
