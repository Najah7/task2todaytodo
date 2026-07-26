package usecase

import (
	"testing"

	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
)

func mustProjectType(t *testing.T, value string) domain.ProjectType {
	t.Helper()
	projectType, err := domain.NewProjectType(value)
	if err != nil {
		t.Fatalf("domain.NewProjectType() error = %v", err)
	}
	return projectType
}

func mustTaskPriority(t *testing.T, value string) domain.TaskPriority {
	t.Helper()
	priority, err := domain.NewTaskPriority(value)
	if err != nil {
		t.Fatalf("domain.NewTaskPriority() error = %v", err)
	}
	return priority
}

func mustTaskStatus(t *testing.T, value string) domain.TaskStatus {
	t.Helper()
	status, err := domain.NewTaskStatus(value)
	if err != nil {
		t.Fatalf("domain.NewTaskStatus() error = %v", err)
	}
	return status
}
