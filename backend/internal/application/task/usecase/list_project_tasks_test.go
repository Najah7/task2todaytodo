package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type listProjectTasksProjectRepositoryFake struct {
	project TaskProject
	err     error
	calls   int
}

func (repo *listProjectTasksProjectRepositoryFake) GetProjectByUserID(_ context.Context, userID, id string) (TaskProject, error) {
	repo.calls++
	if repo.err != nil {
		return TaskProject{}, repo.err
	}
	if repo.project.ID != id || repo.project.OwnerID != userID {
		return TaskProject{}, ErrTaskProjectNotFound
	}
	return repo.project, nil
}

func (repo *listProjectTasksProjectRepositoryFake) GetProjectByUserIDWithPermission(ctx context.Context, userID, id string, _ shared.Capability) (TaskProject, error) {
	return repo.GetProjectByUserID(ctx, userID, id)
}
func (repo *listProjectTasksProjectRepositoryFake) LockProjectByUserIDWithPermission(ctx context.Context, userID, id string, _ shared.Capability) (TaskProject, error) {
	return repo.GetProjectByUserID(ctx, userID, id)
}

type listProjectTasksTaskRepositoryFake struct {
	taskProgressSourceFake
	rows      []dao.Task
	err       error
	userID    domain.UserID
	projectID domain.ProjectID
	limit     int
	anchor    *CursorAnchor
	calls     int
}

func (repo *listProjectTasksTaskRepositoryFake) ListByProjectAndUserIDCursor(_ context.Context, userID domain.UserID, projectID domain.ProjectID, limit int, anchor *CursorAnchor) ([]dao.Task, error) {
	repo.calls++
	repo.userID = userID
	repo.projectID = projectID
	repo.limit = limit
	repo.anchor = anchor
	return repo.rows, repo.err
}

func TestListProjectTasksChecksOwnerAndReturnsCursorPage(t *testing.T) {
	userID := domain.UserID("user-1")
	projectID := domain.ProjectID("project-1")
	projectRepo := &listProjectTasksProjectRepositoryFake{project: TaskProject{ID: string(projectID), OwnerID: string(userID)}}
	rows := []dao.Task{{ID: "new", CursorCreatedAt: "2026-10-04T00:00:00.000002Z"}, {ID: "next", CursorCreatedAt: "2026-10-04T00:00:00.000001Z"}, {ID: "extra"}}
	taskRepo := &listProjectTasksTaskRepositoryFake{rows: rows, taskProgressSourceFake: taskProgressSourceFake{sources: dao.TaskProgressSources{
		Counts: map[string]dao.TaskProgressCounts{
			"new":  {Total: 2, Completed: 1},
			"next": {Total: 4, Completed: 1},
		},
	}}}
	page, err := NewListProjectTasksUseCase(projectRepo, taskRepo, nil).Execute(context.Background(), userID, projectID, CursorPageRequest{Size: 2})
	if err != nil || len(page.Items) != 2 || page.Next == nil || page.Next.ID != "next" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if projectRepo.calls != 1 || taskRepo.calls != 1 || taskRepo.userID != userID || taskRepo.projectID != projectID || taskRepo.limit != 3 {
		t.Fatalf("repos project calls=%d task=%+v", projectRepo.calls, taskRepo)
	}
	if page.Items[0].Progress != 50 || page.Items[1].Progress != 25 || taskRepo.taskProgressSourceFake.calls != 1 {
		t.Fatalf("progress=%d,%d source calls=%d, want 50,25 and one read", page.Items[0].Progress, page.Items[1].Progress, taskRepo.taskProgressSourceFake.calls)
	}
}

func TestListProjectTasksDoesNotQueryChildrenForForeignProject(t *testing.T) {
	projectRepo := &listProjectTasksProjectRepositoryFake{project: TaskProject{OwnerID: "other-user"}}
	taskRepo := &listProjectTasksTaskRepositoryFake{}
	_, err := NewListProjectTasksUseCase(projectRepo, taskRepo, nil).Execute(context.Background(), "user-1", "project-1", CursorPageRequest{Size: 10})
	if !errors.Is(err, ErrTaskProjectNotFound) || taskRepo.calls != 0 {
		t.Fatalf("error=%v child queries=%d", err, taskRepo.calls)
	}
}
