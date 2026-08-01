package domain

import "errors"

var (
	ErrTaskPriorityEmpty   = errors.New("task priority cannot be empty")
	ErrTaskPriorityInvalid = errors.New("invalid task priority")
)

type TaskPriority struct {
	Value   string
	Label   string
	LabelJp string
	Weight  int
}

var taskPriorities = map[string]TaskPriority{
	"urgent":  {Value: "urgent", Label: "Urgent", LabelJp: "緊急", Weight: 100},
	"high":    {Value: "high", Label: "High", LabelJp: "高", Weight: 50},
	"medium":  {Value: "medium", Label: "Medium", LabelJp: "中", Weight: 25},
	"low":     {Value: "low", Label: "Low", LabelJp: "低", Weight: 10},
	"someday": {Value: "someday", Label: "Someday", LabelJp: "いつか", Weight: 0},
}

func NewTaskPriority(priority string) (TaskPriority, error) {
	p, ok := taskPriorities[priority]
	if !ok {
		p = TaskPriority{Value: priority}
	}
	if err := p.validate(); err != nil {
		return TaskPriority{}, err
	}
	return p, nil
}

func (p TaskPriority) validate() error {
	if p.String() == "" {
		return ErrTaskPriorityEmpty
	}

	if _, ok := taskPriorities[p.Value]; ok {
		return nil
	}
	return ErrTaskPriorityInvalid
}

func (p TaskPriority) String() string {
	return p.Value
}
