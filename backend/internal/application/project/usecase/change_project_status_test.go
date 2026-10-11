package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type projectStatusRepositoryFake struct {
	Repository
	project dao.Project
}

func (f *projectStatusRepositoryFake) LockByUserIDWithPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (dao.Project, error) {
	return f.project, nil
}
func (f *projectStatusRepositoryFake) SetStatusByUserID(_ context.Context, _ domain.UserID, _ domain.ProjectID, status string, _ int32) (dao.Project, error) {
	f.project.Status = status
	f.project.Revision++
	return f.project, nil
}
func (f *projectStatusRepositoryFake) HasPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (bool, error) {
	return true, nil
}

type projectStatusChildrenFake struct {
	ProjectChildrenDeleter
	paused, resumed int
}

func (f *projectStatusChildrenFake) PauseProjectBlueprints(context.Context, string, string, time.Time) error {
	f.paused++
	return nil
}
func (f *projectStatusChildrenFake) ResumeProjectBlueprints(context.Context, string, string, time.Time) error {
	f.resumed++
	return nil
}

type projectStatusUOWFake struct {
	repo     Repository
	children ProjectChildrenDeleter
}

func (f projectStatusUOWFake) Do(ctx context.Context, fn func(context.Context, Repository, ProjectChildrenDeleter) error) error {
	return fn(ctx, f.repo, f.children)
}

func TestChangeProjectStatusRunsScheduleLifecycleInSameUOW(t *testing.T) {
	tests := []struct {
		name       string
		from, to   string
		wantPause  int
		wantResume int
	}{
		{name: "manual completion pauses recurring blueprints", from: "open", to: "done", wantPause: 1},
		{name: "manual reopening resumes project-paused blueprints", from: "done", to: "open", wantResume: 1},
		{name: "other status change does not touch blueprints", from: "pending", to: "in_progress"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &projectStatusRepositoryFake{project: dao.Project{
				ID: "project-1", UserID: "user-1", Type: dao.ProjectType{Value: "work"}, Status: tt.from,
				Title: "Project", Priority: dao.Priority{Value: "medium"}, Revision: 1,
			}}
			children := &projectStatusChildrenFake{}
			progress := &projectProgressSourceFake{}
			uc := NewChangeProjectStatusUseCase(projectStatusUOWFake{repo: repo, children: children}, progress, nil)
			if _, err := uc.Execute(context.Background(), domain.UserID("user-1"), domain.ProjectID("project-1"), 1, tt.to); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if children.paused != tt.wantPause || children.resumed != tt.wantResume {
				t.Fatalf("schedule lifecycle calls = pause:%d resume:%d, want pause:%d resume:%d", children.paused, children.resumed, tt.wantPause, tt.wantResume)
			}
		})
	}
}
