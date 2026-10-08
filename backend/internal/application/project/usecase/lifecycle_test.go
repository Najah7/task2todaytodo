package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type projectLifecycleRepositoryFake struct {
	status    string
	statusErr error
	lockErr   error
	setTo     string
	setCalls  int
}

func (f *projectLifecycleRepositoryFake) LockProjectForLifecycle(context.Context, domain.ProjectID) error {
	return f.lockErr
}
func (f *projectLifecycleRepositoryFake) ReadProjectStatusForLifecycle(context.Context, domain.ProjectID) (string, error) {
	return f.status, f.statusErr
}
func (f *projectLifecycleRepositoryFake) SetStatusForLifecycle(_ context.Context, _ domain.UserID, _ domain.ProjectID, status string) error {
	f.setCalls++
	f.setTo = status
	return nil
}

type projectLifecycleProgressFake struct{ sources dao.ProjectProgressSources }

func (f projectLifecycleProgressFake) ReadProjectProgressSources(context.Context, []string, time.Time) (dao.ProjectProgressSources, error) {
	return f.sources, nil
}

func TestProjectLifecycleLockParentTranslatesMissingProject(t *testing.T) {
	repo := &projectLifecycleRepositoryFake{lockErr: domain.ErrProjectNotFound}
	uc := NewProjectLifecycleUseCase(repo, projectLifecycleProgressFake{})
	if err := uc.LockParent(context.Background(), "project-1"); !errors.Is(err, shared.ErrProjectUnavailable) {
		t.Fatalf("LockParent error = %v, want shared unavailable sentinel", err)
	}
}

func TestProjectLifecycleReadStatusUsesStatusOnlyPortAndTranslatesMissingProject(t *testing.T) {
	repo := &projectLifecycleRepositoryFake{status: "done", statusErr: domain.ErrProjectNotFound}
	uc := NewProjectLifecycleUseCase(repo, projectLifecycleProgressFake{})
	if _, err := uc.ReadStatus(context.Background(), "project-1"); !errors.Is(err, shared.ErrProjectUnavailable) {
		t.Fatalf("ReadStatus error = %v, want shared unavailable sentinel", err)
	}
	repo.statusErr = nil
	status, err := uc.ReadStatus(context.Background(), "project-1")
	if err != nil || status != "done" {
		t.Fatalf("ReadStatus = (%q, %v), want done without progress capture", status, err)
	}
}

func TestProjectLifecycleReconcilesCountsOnlyOnEligibleTransitions(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		before     shared.ProjectWorkState
		after      dao.ProjectProgressSources
		wantStatus string
	}{
		{
			name:   "done reopens when unfinished work grows",
			status: "done",
			before: shared.ProjectWorkState{Status: "done", Eligible: 1, Completed: 1},
			after: dao.ProjectProgressSources{Tasks: []dao.ProjectTaskProgress{
				{TaskID: "task-done", Done: true}, {TaskID: "task-open", Done: false},
			}},
			wantStatus: "open",
		},
		{
			name:       "empty project can become done when completed work is attached",
			status:     "open",
			before:     shared.ProjectWorkState{Status: "open"},
			after:      dao.ProjectProgressSources{Tasks: []dao.ProjectTaskProgress{{TaskID: "task-done", Done: true}}},
			wantStatus: "done",
		},
		{
			name:       "unrelated count-neutral change does not reopen done project",
			status:     "done",
			before:     shared.ProjectWorkState{Status: "done", Eligible: 1, Completed: 1},
			after:      dao.ProjectProgressSources{Tasks: []dao.ProjectTaskProgress{{TaskID: "task-done", Done: true}}},
			wantStatus: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &projectLifecycleRepositoryFake{status: tt.status}
			uc := NewProjectLifecycleUseCase(repo, projectLifecycleProgressFake{sources: tt.after})
			if err := uc.ReconcileWorkState(context.Background(), "actor", "project-1", tt.before, time.Now()); err != nil {
				t.Fatalf("ReconcileWorkState: %v", err)
			}
			if repo.setTo != tt.wantStatus || repo.setCalls != boolInt(tt.wantStatus != "") {
				t.Fatalf("status writes = %d to %q, want %q", repo.setCalls, repo.setTo, tt.wantStatus)
			}
		})
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
