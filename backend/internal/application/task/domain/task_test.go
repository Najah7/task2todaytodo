package domain

import (
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	task, err := NewTask("task-1", "user-1", "Write tests")
	if err != nil {
		t.Fatalf("NewTask() error = %v", err)
	}

	if task.ID != "task-1" || task.UserID != "user-1" || task.Title != "Write tests" {
		t.Errorf("task = %+v, want required values to be set", task)
	}
	if task.ProjectID != "" || task.Description != "" || task.Progress != 0 {
		t.Errorf("task = %+v, want only required values to be set", task)
	}
	if task.Priority.String() != "low" || task.Status.String() != "open" {
		t.Errorf("task defaults = %q/%q, want low/open", task.Priority.String(), task.Status.String())
	}
}

func TestNewTaskWithDetails(t *testing.T) {
	estimated := 30
	actual := 10
	priority := mustTaskPriority(t, "low")
	status := mustTaskStatus(t, "open")
	dueDate := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)

	task, err := NewTaskWithDetails("task-1", "user-1", "project-1", "Write tests", "Domain entity tests", dueDate, &estimated, &actual, 20, priority, status)
	if err != nil {
		t.Fatalf("NewTaskWithDetails() error = %v", err)
	}

	if task.ProjectID != "project-1" || task.DueDate != dueDate || task.ManualEstimatedMinutes == nil || *task.ManualEstimatedMinutes != estimated || task.ActualMinutes == nil || *task.ActualMinutes != actual {
		t.Errorf("task = %+v, want details to be set", task)
	}
}

func TestNewTaskValidation(t *testing.T) {
	tests := []struct {
		name    string
		id      TaskID
		userID  UserID
		title   string
		wantErr error
	}{
		{name: "empty ID", userID: "user-1", title: "Task", wantErr: ErrTaskIDEmpty},
		{name: "empty user ID", id: "task-1", title: "Task", wantErr: ErrTaskUserIDEmpty},
		{name: "blank title", id: "task-1", userID: "user-1", title: " ", wantErr: ErrTaskTitleEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTask(tt.id, tt.userID, tt.title)
			assertTaskDomainErrorIs(t, err, tt.wantErr)
			if got.ID != tt.id || got.UserID != tt.userID || got.Title != tt.title {
				t.Errorf("task = %+v, want input values to be preserved", got)
			}
		})
	}
}

func TestNewTaskWithDetailsValidation(t *testing.T) {
	priority := mustTaskPriority(t, "low")
	status := mustTaskStatus(t, "open")
	negative := -1

	tests := []struct {
		name             string
		id               TaskID
		userID           UserID
		title            string
		estimatedMinutes *int
		actualMinutes    *int
		progress         int
		priority         TaskPriority
		status           TaskStatus
		wantErr          error
	}{
		{name: "negative estimated minutes", id: "task-1", userID: "user-1", title: "Task", estimatedMinutes: &negative, progress: 0, priority: priority, status: status, wantErr: ErrTaskEstimatedMinutesInvalid},
		{name: "negative actual minutes", id: "task-1", userID: "user-1", title: "Task", actualMinutes: &negative, progress: 0, priority: priority, status: status, wantErr: ErrTaskActualMinutesInvalid},
		{name: "negative progress", id: "task-1", userID: "user-1", title: "Task", progress: -1, priority: priority, status: status, wantErr: ErrTaskProgressInvalid},
		{name: "progress too high", id: "task-1", userID: "user-1", title: "Task", progress: 101, priority: priority, status: status, wantErr: ErrTaskProgressInvalid},
		{name: "invalid priority", id: "task-1", userID: "user-1", title: "Task", progress: 0, priority: TaskPriority{Value: "blocked"}, status: status, wantErr: ErrTaskPriorityInvalid},
		{name: "invalid status", id: "task-1", userID: "user-1", title: "Task", progress: 0, priority: priority, status: TaskStatus{Value: "archived"}, wantErr: ErrTaskStatusInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTaskWithDetails(tt.id, tt.userID, "", tt.title, "", time.Time{}, tt.estimatedMinutes, tt.actualMinutes, tt.progress, tt.priority, tt.status)
			assertTaskDomainErrorIs(t, err, tt.wantErr)
			if got.ID != tt.id || got.UserID != tt.userID || got.Title != tt.title {
				t.Errorf("task = %+v, want input values to be preserved", got)
			}
		})
	}
}

func TestTaskStatusChanges(t *testing.T) {
	statusValues := []string{"open", "pending", "waiting_on_others", "in_progress", "done"}
	operations := []struct {
		name       string
		wantStatus string
		change     func(Task) Task
	}{
		{name: "start", wantStatus: "in_progress", change: Task.Start},
		{name: "hold", wantStatus: "pending", change: Task.Hold},
		{name: "wait", wantStatus: "waiting_on_others", change: Task.Wait},
		{name: "complete", wantStatus: "done", change: Task.Complete},
		{name: "reopen", wantStatus: "open", change: Task.Reopen},
	}

	for _, currentStatus := range statusValues {
		for _, operation := range operations {
			t.Run(currentStatus+"/"+operation.name, func(t *testing.T) {
				current := mustTaskStatus(t, currentStatus)
				task, err := NewTaskWithDetails(
					"task-1", "user-1", "project-1", "Task", "Description",
					time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC), nil, nil, 25,
					mustTaskPriority(t, "high"), current,
				)
				if err != nil {
					t.Fatalf("NewTaskWithDetails() error = %v", err)
				}

				updated := operation.change(task)
				if updated.Status.String() != operation.wantStatus {
					t.Errorf("status = %q, want %q", updated.Status.String(), operation.wantStatus)
				}
				if updated.ID != task.ID || updated.UserID != task.UserID || updated.ProjectID != task.ProjectID ||
					updated.Title != task.Title || updated.Description != task.Description || updated.DueDate != task.DueDate ||
					updated.Progress != task.Progress || updated.Priority != task.Priority ||
					updated.CreatedAt != task.CreatedAt || updated.UpdatedAt != task.UpdatedAt {
					t.Errorf("updated task = %+v, want other fields preserved from %+v", updated, task)
				}
				if task.Status.String() != currentStatus {
					t.Errorf("original task status = %q, want unchanged %q", task.Status.String(), currentStatus)
				}

				repeated := operation.change(updated)
				if repeated != updated {
					t.Errorf("repeated %s() = %+v, want same task %+v", operation.name, repeated, updated)
				}
			})
		}
	}
}

func mustTaskPriority(t *testing.T, value string) TaskPriority {
	t.Helper()
	priority, err := NewTaskPriority(value)
	if err != nil {
		t.Fatalf("NewTaskPriority() error = %v", err)
	}
	return priority
}

func mustTaskStatus(t *testing.T, value string) TaskStatus {
	t.Helper()
	status, err := NewTaskStatus(value)
	if err != nil {
		t.Fatalf("NewTaskStatus() error = %v", err)
	}
	return status
}
