package domain

import "github.com/Najah7/task2todaytodo/internal/application/shared/status"

var (
	ErrTaskStatusEmpty   = status.ErrEmpty
	ErrTaskStatusInvalid = status.ErrInvalid
)

type TaskStatus = status.Status

func NewTaskStatus(value string) (TaskStatus, error) { return status.New(value) }
