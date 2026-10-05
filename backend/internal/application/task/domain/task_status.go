package domain

import "errors"

var (
	ErrTaskStatusEmpty   = errors.New("task status cannot be empty")
	ErrTaskStatusInvalid = errors.New("invalid task status")
)

const (
	taskStatusOpen            = "open"
	taskStatusPending         = "pending"
	taskStatusWaitingOnOthers = "waiting_on_others"
	taskStatusInProgress      = "in_progress"
	taskStatusDone            = "done"
)

type TaskStatus struct {
	Value   string
	Label   string
	LabelJp string
}

var taskStatuses = map[string]TaskStatus{
	taskStatusOpen:            {Value: taskStatusOpen, Label: "Open", LabelJp: "オープン"},
	taskStatusPending:         {Value: taskStatusPending, Label: "Pending", LabelJp: "保留"},
	taskStatusWaitingOnOthers: {Value: taskStatusWaitingOnOthers, Label: "Waiting on others", LabelJp: "他者待ち"},
	taskStatusInProgress:      {Value: taskStatusInProgress, Label: "In progress", LabelJp: "進行中"},
	taskStatusDone:            {Value: taskStatusDone, Label: "Done", LabelJp: "完了"},
}

func NewTaskStatus(status string) (TaskStatus, error) {
	s, ok := taskStatuses[status]
	if !ok {
		s = TaskStatus{Value: status}
	}
	if err := s.validate(); err != nil {
		return TaskStatus{}, err
	}
	return s, nil
}

func (s TaskStatus) validate() error {
	if s.String() == "" {
		return ErrTaskStatusEmpty
	}

	if _, ok := taskStatuses[s.Value]; ok {
		return nil
	}
	return ErrTaskStatusInvalid
}

func (s TaskStatus) String() string {
	return s.Value
}

func (s TaskStatus) IsOpen() bool {
	return s.Value == taskStatusOpen
}

func (s TaskStatus) IsPending() bool {
	return s.Value == taskStatusPending
}

func (s TaskStatus) IsWaitingOnOthers() bool {
	return s.Value == taskStatusWaitingOnOthers
}

func (s TaskStatus) IsInProgress() bool {
	return s.Value == taskStatusInProgress
}

func (s TaskStatus) IsDone() bool {
	return s.Value == taskStatusDone
}
