package domain

import "errors"

var (
	ErrTaskStatusEmpty   = errors.New("task status cannot be empty")
	ErrTaskStatusInvalid = errors.New("invalid task status")
)

type TaskStatus struct {
	Value   string
	Label   string
	LabelJp string
}

var taskStatuses = map[string]TaskStatus{
	"open":              {Value: "open", Label: "Open", LabelJp: "オープン"},
	"pending":           {Value: "pending", Label: "Pending", LabelJp: "保留"},
	"waiting_on_others": {Value: "waiting_on_others", Label: "Waiting on others", LabelJp: "他者待ち"},
	"in_progress":       {Value: "in_progress", Label: "In progress", LabelJp: "進行中"},
	"done":              {Value: "done", Label: "Done", LabelJp: "完了"},
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
