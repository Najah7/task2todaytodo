//go:build integration

package task_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskrepo "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/oklog/ulid/v2"
)

func TestActionItemChildTransitionsUseUnfilteredCountWhenReadDenied(t *testing.T) {
	pool := integrationPool(t)
	fixture := newSharingFixture(t, pool)
	actorID := fixture.user(t)
	roleID := fixture.customRole(t, "action_item_create_delete_no_read")
	fixture.addMember(t, actorID, roleID)
	for _, permission := range [][2]string{
		{"project", "read"}, {"task", "read"}, {"task", "update"},
		{"action_item", "create"}, {"action_item", "delete"},
	} {
		permissionID := sharingPermissionID(t, pool, permission[0], permission[1], "allow")
		if _, err := pool.Exec(t.Context(), `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, roleID, permissionID); err != nil {
			t.Fatalf("grant %s.%s: %v", permission[0], permission[1], err)
		}
	}
	denyReadID := sharingPermissionID(t, pool, "action_item", "read", "deny")
	if _, err := pool.Exec(t.Context(), `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, roleID, denyReadID); err != nil {
		t.Fatalf("deny action_item.read: %v", err)
	}
	taskID := fixture.task(t, actorID, "Hidden child count")
	existingItemID := fixture.actionItem(t, taskID)
	fixture.commit(t)

	ctx := t.Context()
	repository := taskrepo.NewActionItemRepository(pool)
	visible, err := repository.ListByTask(ctx, domain.UserID(actorID), domain.TaskID(taskID))
	if err != nil {
		t.Fatalf("list hidden ActionItems: %v", err)
	}
	if len(visible) != 0 {
		t.Fatalf("actor can read %d ActionItems despite action_item.read deny", len(visible))
	}
	projection, err := repository.ReadTaskListProjection(ctx, domain.UserID(actorID), []string{taskID})
	if err != nil {
		t.Fatalf("read source capability projection: %v", err)
	}
	if len(projection.ActionItemsByTask[taskID]) != 0 || !projection.TasksWithChildren[taskID] {
		t.Fatalf("source projection exposed hidden children or lost true child fact: %+v", projection)
	}

	uow := sharingTestUOW{pool: pool}
	create := usecase.NewCreateActionItemUseCase(uow, nil)
	estimate := 15
	created, err := create.Execute(ctx, usecase.CreateActionItemInput{
		ID: domain.ActionItemID(ulid.Make().String()), UserID: domain.UserID(actorID), TaskID: domain.TaskID(taskID),
		Title: "Create despite hidden existing child", EstimatedMinutes: &estimate,
	})
	if err != nil {
		t.Fatalf("create second child with create permission and hidden existing child: %v", err)
	}
	if created.TaskID != taskID {
		t.Fatalf("created ActionItem belongs to %s; want %s", created.TaskID, taskID)
	}

	delete := usecase.NewDeleteActionItemUseCase(uow, nil)
	if err := delete.Execute(ctx, domain.UserID(actorID), domain.TaskID(taskID), domain.ActionItemID(created.ID)); err != nil {
		t.Fatalf("delete one child when another hidden child remains: %v", err)
	}
	err = delete.Execute(ctx, domain.UserID(actorID), domain.TaskID(taskID), domain.ActionItemID(existingItemID))
	if !errors.Is(err, usecase.ErrTaskManualEstimateRequired) {
		t.Fatalf("delete final hidden child error=%v; want positive manual-estimate requirement", err)
	}
	var childCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM action_items WHERE task_id=$1 AND deleted_at IS NULL`, taskID).Scan(&childCount); err != nil {
		t.Fatal(err)
	}
	if childCount != 1 {
		t.Errorf("rejected final delete changed hidden child count to %d; want 1", childCount)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM tasks WHERE id=$1`, taskID)
	})
}
