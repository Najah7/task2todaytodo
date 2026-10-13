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
	status       string
	statusErr    error
	lockErr      error
	setTo        string
	setCalls     int
	ownerID      domain.UserID
	projectCount int
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
func (*projectLifecycleRepositoryFake) LockUserForPlanLimit(context.Context, domain.UserID) error {
	return nil
}
func (f *projectLifecycleRepositoryFake) LockProjectOwnerForPlanLimit(context.Context, domain.ProjectID) (domain.UserID, error) {
	if f.ownerID == "" {
		return "owner-1", nil
	}
	return f.ownerID, nil
}
func (f *projectLifecycleRepositoryFake) CountActiveOwnedProjects(context.Context, domain.UserID) (int, error) {
	return f.projectCount, nil
}

type projectLifecycleProgressFake struct{ sources dao.ProjectProgressSources }

func (f projectLifecycleProgressFake) ReadProjectProgressSources(context.Context, []string, time.Time) (dao.ProjectProgressSources, error) {
	return f.sources, nil
}

type projectLifecycleScheduleFake struct {
	paused, resumed int
	actor, project  string
	at              time.Time
}

func (f *projectLifecycleScheduleFake) PauseProjectRules(_ context.Context, actor, project string, at time.Time) error {
	f.paused++
	f.actor, f.project, f.at = actor, project, at
	return nil
}
func (f *projectLifecycleScheduleFake) ResumeProjectRules(_ context.Context, actor, project string, at time.Time) error {
	f.resumed++
	f.actor, f.project, f.at = actor, project, at
	return nil
}

type projectLifecycleScheduleNoop struct{}

func (projectLifecycleScheduleNoop) PauseProjectRules(context.Context, string, string, time.Time) error {
	return nil
}
func (projectLifecycleScheduleNoop) ResumeProjectRules(context.Context, string, string, time.Time) error {
	return nil
}

func TestProjectLifecycleLockParentTranslatesMissingProject(t *testing.T) {
	repo := &projectLifecycleRepositoryFake{lockErr: domain.ErrProjectNotFound}
	uc := NewProjectLifecycleUseCase(repo, projectLifecycleProgressFake{}, projectLifecycleScheduleNoop{})
	if err := uc.LockParent(context.Background(), "project-1"); !errors.Is(err, shared.ErrProjectUnavailable) {
		t.Fatalf("LockParent error = %v, want shared unavailable sentinel", err)
	}
}

func TestProjectLifecycleReadStatusUsesStatusOnlyPortAndTranslatesMissingProject(t *testing.T) {
	repo := &projectLifecycleRepositoryFake{status: "done", statusErr: domain.ErrProjectNotFound}
	uc := NewProjectLifecycleUseCase(repo, projectLifecycleProgressFake{}, projectLifecycleScheduleNoop{})
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
				{TaskDone: true, TotalActionItems: 1, CompletedActionItems: 1}, {TaskDone: false, TotalActionItems: 1},
			}},
			wantStatus: "open",
		},
		{
			name:       "empty project can become done when completed work is attached",
			status:     "open",
			before:     shared.ProjectWorkState{Status: "open"},
			after:      dao.ProjectProgressSources{Tasks: []dao.ProjectTaskProgress{{TaskDone: true, TotalActionItems: 1, CompletedActionItems: 1}}},
			wantStatus: "done",
		},
		{
			name:       "done project reopens when persisted unfinished schedule is attached",
			status:     "done",
			before:     shared.ProjectWorkState{Status: "done", Eligible: 1, Completed: 1},
			after:      dao.ProjectProgressSources{Schedules: []dao.ProjectScheduleProgress{{ProjectID: "project-1", Total: 2, Completed: 1}}},
			wantStatus: "open",
		},
		{
			name:       "project completes when its persisted schedule completes",
			status:     "open",
			before:     shared.ProjectWorkState{Status: "open", Eligible: 1},
			after:      dao.ProjectProgressSources{Schedules: []dao.ProjectScheduleProgress{{ProjectID: "project-1", Total: 1, Completed: 1}}},
			wantStatus: "done",
		},
		{
			name:       "unrelated count-neutral change does not reopen done project",
			status:     "done",
			before:     shared.ProjectWorkState{Status: "done", Eligible: 1, Completed: 1},
			after:      dao.ProjectProgressSources{Tasks: []dao.ProjectTaskProgress{{TaskDone: true, TotalActionItems: 1, CompletedActionItems: 1}}},
			wantStatus: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &projectLifecycleRepositoryFake{status: tt.status}
			uc := NewProjectLifecycleUseCase(repo, projectLifecycleProgressFake{sources: tt.after}, projectLifecycleScheduleNoop{})
			if err := uc.ReconcileWorkState(context.Background(), "actor", "project-1", tt.before, time.Now()); err != nil {
				t.Fatalf("ReconcileWorkState: %v", err)
			}
			if repo.setTo != tt.wantStatus || repo.setCalls != boolInt(tt.wantStatus != "") {
				t.Fatalf("status writes = %d to %q, want %q", repo.setCalls, repo.setTo, tt.wantStatus)
			}
		})
	}
}

func TestProjectLifecyclePausesAndResumesRulesWithAutomaticStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		before     shared.ProjectWorkState
		after      dao.ProjectProgressSources
		wantStatus string
		wantPause  int
		wantResume int
	}{
		{name: "persisted schedule completion pauses", status: "open", before: shared.ProjectWorkState{Status: "open", Eligible: 1}, after: dao.ProjectProgressSources{Schedules: []dao.ProjectScheduleProgress{{ProjectID: "project-1", Total: 1, Completed: 1}}}, wantStatus: "done", wantPause: 1},
		{name: "persisted unfinished schedule resumes", status: "done", before: shared.ProjectWorkState{Status: "done", Eligible: 1, Completed: 1}, after: dao.ProjectProgressSources{Schedules: []dao.ProjectScheduleProgress{{ProjectID: "project-1", Total: 1}}}, wantStatus: "open", wantResume: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &projectLifecycleRepositoryFake{status: tt.status}
			schedules := &projectLifecycleScheduleFake{}
			at := time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC)
			uc := NewProjectLifecycleUseCase(repo, projectLifecycleProgressFake{sources: tt.after}, schedules)
			if err := uc.ReconcileWorkState(context.Background(), "actor-1", "project-1", tt.before, at); err != nil {
				t.Fatalf("ReconcileWorkState: %v", err)
			}
			if repo.setTo != tt.wantStatus || schedules.paused != tt.wantPause || schedules.resumed != tt.wantResume {
				t.Fatalf("status=%q pause=%d resume=%d; want status=%q pause=%d resume=%d", repo.setTo, schedules.paused, schedules.resumed, tt.wantStatus, tt.wantPause, tt.wantResume)
			}
			if (tt.wantPause+tt.wantResume) > 0 && (schedules.actor != "actor-1" || schedules.project != "project-1" || !schedules.at.Equal(at)) {
				t.Fatalf("schedule lifecycle args = (%q,%q,%v), want actor/project/time", schedules.actor, schedules.project, schedules.at)
			}
		})
	}
}

func TestProjectLifecycleBlocksFreePlanAutomaticReopenAtProjectLimit(t *testing.T) {
	repo := &projectLifecycleRepositoryFake{status: "done", projectCount: maxFreeActiveProjects}
	sources := dao.ProjectProgressSources{Tasks: []dao.ProjectTaskProgress{{TaskDone: false, TotalActionItems: 1}}}
	uc := NewProjectLifecycleUseCaseWithAccess(repo, projectLifecycleProgressFake{sources: sources}, projectLifecycleScheduleNoop{}, nil, freeProjectPlanAccess{})
	err := uc.ReconcileWorkState(context.Background(), "actor-1", "project-1", shared.ProjectWorkState{Status: "done", Eligible: 1, Completed: 1}, time.Now())
	if !errors.Is(err, ErrFreeProjectLimitReached) {
		t.Fatalf("ReconcileWorkState error = %v, want Free Project limit", err)
	}
	if repo.setCalls != 0 {
		t.Fatalf("Project status writes = %d, want none", repo.setCalls)
	}
}

type freeProjectPlanAccess struct{}

func (freeProjectPlanAccess) GetPlanAccess(context.Context, string) (shared.PlanAccess, error) {
	return shared.PlanAccessFree, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
