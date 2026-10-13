package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type planLimitActionItems struct {
	ActionItemRepository
	projectCount     int
	projectlessCount int
	taskCount        int
}

func (items planLimitActionItems) CountForProjectPlanLimit(context.Context, string) (int, error) {
	return items.projectCount, nil
}
func (items planLimitActionItems) CountProjectlessForUserPlanLimit(context.Context, domain.UserID) (int, error) {
	return items.projectlessCount, nil
}
func (items planLimitActionItems) CountForTaskPlanLimit(context.Context, domain.TaskID) (int, error) {
	return items.taskCount, nil
}

type planLimitRepositories struct {
	taskProgressTestRepositories
	items ActionItemRepository
}

func (repos planLimitRepositories) ActionItems() ActionItemRepository { return repos.items }

func TestCheckNewActionItemLimitCapsProjectAndProjectlessItems(t *testing.T) {
	tests := []struct {
		name  string
		task  dao.Task
		items planLimitActionItems
	}{
		{name: "project", task: dao.Task{UserID: "user-1", ProjectID: "project-1"}, items: planLimitActionItems{projectCount: maxFreeActionItems}},
		{name: "projectless", task: dao.Task{UserID: "user-1"}, items: planLimitActionItems{projectlessCount: maxFreeActionItems}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkNewActionItemLimit(context.Background(), planLimitRepositories{items: tt.items}, nil, tt.task)
			if !errors.Is(err, ErrFreeActionItemLimitReached) {
				t.Fatalf("checkNewActionItemLimit() error = %v, want Free ActionItem limit", err)
			}
		})
	}
}

func TestCheckActionItemDestinationLimitAllowsEmptyTaskIntoOverLimitDestination(t *testing.T) {
	repos := planLimitRepositories{items: planLimitActionItems{projectCount: maxFreeActionItems + 1}}
	err := checkActionItemDestinationLimit(context.Background(), repos, nil, "user-1", "task-1", "project-1")
	if err != nil {
		t.Fatalf("checkActionItemDestinationLimit() error = %v, want count-neutral move to succeed", err)
	}
}
