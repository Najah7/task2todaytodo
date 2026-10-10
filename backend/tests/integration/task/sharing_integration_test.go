//go:build integration

package task_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	projectrepo "github.com/Najah7/task2todaytodo/internal/application/project/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskrepo "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func sharingPermissionID(t *testing.T, tx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, resource, action, effect string) int64 {
	t.Helper()
	var permissionID int64
	if err := tx.QueryRow(t.Context(), `
		SELECT permission_id
		FROM permissions
		WHERE resource_id = $1 AND action = $2::action AND effect = $3::effect
	`, resource, action, effect).Scan(&permissionID); err != nil {
		t.Fatalf("find %s.%s %s permission: %v", resource, action, effect, err)
	}
	return permissionID
}

func TestTaskListIsAssignedOnly(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	editorID, viewerID := fixture.user(t), fixture.user(t)
	fixture.addMember(t, editorID, "editor")
	fixture.addMember(t, viewerID, "viewer")

	delegatedID := fixture.task(t, editorID, "Assigned to editor")
	ownerTaskID := fixture.task(t, fixture.ownerID, "Still assigned to owner")
	viewerTaskID := fixture.task(t, viewerID, "Assigned to viewer")
	fixture.commit(t)
	tasks := taskrepo.NewTaskRepository(pool)

	ownerTasks, err := tasks.ListByUserIDCursor(ctx, domain.UserID(fixture.ownerID), 10, nil)
	if err != nil {
		t.Fatalf("list owner's assigned tasks: %v", err)
	}
	if len(ownerTasks) != 1 || ownerTasks[0].ID != ownerTaskID {
		t.Fatalf("owner task list = %+v, want only owner-assigned task %s (exclude delegated task %s)", ownerTasks, ownerTaskID, delegatedID)
	}

	personal, err := tasks.ListByUserIDCursor(ctx, domain.UserID(editorID), 10, nil)
	if err != nil {
		t.Fatalf("list editor's assigned tasks: %v", err)
	}
	if len(personal) != 1 || personal[0].ID != delegatedID {
		t.Fatalf("editor task list = %+v, want only delegated task %s (exclude %s)", personal, delegatedID, ownerTaskID)
	}

	viewerTasks, err := tasks.ListByUserIDCursor(ctx, domain.UserID(viewerID), 10, nil)
	if err != nil || len(viewerTasks) != 1 || viewerTasks[0].ID != viewerTaskID {
		t.Fatalf("viewer assigned task list = %+v, error %v; want only viewer-assigned task %s", viewerTasks, err, viewerTaskID)
	}

	if _, err := tasks.GetByUserID(ctx, domain.UserID(viewerID), domain.TaskID(delegatedID)); err != nil {
		t.Fatalf("viewer reads task assigned to editor: %v", err)
	}
	if _, err := usecase.NewAssignTaskUseCase(sharingTestUOW{pool: pool}, nil).Execute(
		ctx, domain.UserID(viewerID), domain.TaskID(viewerTaskID), domain.UserID(editorID), 1,
	); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("viewer assignee task mutation error = %v, want permission denied", err)
	}

	listAssignees := usecase.NewListTaskAssigneesUseCase(tasks, nil)
	if _, err := listAssignees.Execute(ctx, domain.UserID(viewerID), domain.TaskID(viewerTaskID)); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("viewer assignee list error = %v, want permission denied", err)
	}
	missingTaskID := domain.TaskID(ulid.Make().String())
	if _, err := listAssignees.Execute(ctx, domain.UserID(viewerID), missingTaskID); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("missing task assignee list error = %v, want task not found", err)
	}
}

func TestTaskAssignmentUsesItsOwnDynamicPermission(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	editorID, viewerID := fixture.user(t), fixture.user(t)
	assignmentRoleID := fixture.customRole(t, "assignment")
	fixture.addMember(t, editorID, assignmentRoleID)
	fixture.addMember(t, viewerID, "viewer")
	taskID := fixture.task(t, fixture.ownerID, "Assignment permission task")
	fixture.commit(t)

	taskUpdateAllow := sharingPermissionID(t, pool, "task", "update", "allow")
	taskUpdateDeny := sharingPermissionID(t, pool, "task", "update", "deny")
	assignmentAllow := sharingPermissionID(t, pool, "task_assignment", "update", "allow")
	assignmentDeny := sharingPermissionID(t, pool, "task_assignment", "update", "deny")
	if _, err := pool.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`, assignmentRoleID, taskUpdateAllow); err != nil {
		t.Fatalf("remove custom role task.update allow: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		VALUES ($1, $2), ($1, $3)
	`, assignmentRoleID, taskUpdateDeny, assignmentAllow); err != nil {
		t.Fatalf("grant custom role task.update deny and assignment allow: %v", err)
	}
	assign := usecase.NewAssignTaskUseCase(sharingTestUOW{pool: pool}, nil)
	updated, err := assign.Execute(ctx, domain.UserID(editorID), domain.TaskID(taskID), domain.UserID(viewerID), 1)
	if err != nil {
		t.Fatalf("task.update deny blocked task_assignment.update allow: %v", err)
	}
	if updated.AssigneeID != viewerID || updated.Revision != 2 {
		t.Fatalf("task after permitted assignment = assignee %q revision %d, want %q/2", updated.AssigneeID, updated.Revision, viewerID)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, assignmentRoleID, assignmentDeny); err != nil {
		t.Fatalf("grant custom role matching assignment deny: %v", err)
	}
	if _, err := assign.Execute(ctx, domain.UserID(editorID), domain.TaskID(taskID), domain.UserID(fixture.ownerID), 2); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("matching task_assignment.update deny with allow returned %v, want permission denied", err)
	}
	var assigneeID string
	var revision int32
	if err := pool.QueryRow(ctx, `SELECT assignee_id, revision FROM tasks WHERE id = $1`, taskID).Scan(&assigneeID, &revision); err != nil {
		t.Fatalf("read task after denied assignment: %v", err)
	}
	if assigneeID != viewerID || revision != 2 {
		t.Fatalf("denied assignment changed task to assignee %q revision %d; want unchanged %q/2", assigneeID, revision, viewerID)
	}
}

func TestOperationOnlyPermissionsDoNotRequireReadOrParentUpdateGrants(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	actorID := fixture.user(t)
	roleID := fixture.customRole(t, "operation_only")
	fixture.addMember(t, actorID, roleID)
	taskID := fixture.task(t, fixture.ownerID, "Operation-only task")
	actionItemID := fixture.actionItem(t, taskID)
	if _, err := fixture.tx.Exec(ctx, `UPDATE action_items SET completed = true WHERE id = $1`, actionItemID); err != nil {
		t.Fatalf("complete operation-only actionItem fixture: %v", err)
	}
	fixture.commit(t)

	for _, grant := range []struct{ resource, action string }{
		{"task", "update"},
		{"action_item", "update"},
	} {
		permissionID := sharingPermissionID(t, pool, grant.resource, grant.action, "allow")
		if _, err := pool.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, roleID, permissionID); err != nil {
			t.Fatalf("grant operation-only %s.%s: %v", grant.resource, grant.action, err)
		}
	}

	taskRepo := taskrepo.NewTaskRepository(pool)
	actor := domain.UserID(actorID)
	task := domain.TaskID(taskID)
	if _, err := taskRepo.GetByUserID(ctx, actor, task); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("task.read without allow = %v, want hidden task", err)
	}
	newTaskTitle := "Updated without task.read permission"
	if _, err := usecase.NewUpdateTaskUseCase(sharingTestUOW{pool: pool}, nil).Execute(ctx, usecase.UpdateTaskInput{
		UserID: actor, TaskID: task, ExpectedRevision: 1,
		Title: usecase.PatchField[string]{Present: true, Value: &newTaskTitle},
	}); err != nil {
		t.Fatalf("task.update with no task.read grant: %v", err)
	}

	taskUpdateAllow := sharingPermissionID(t, pool, "task", "update", "allow")
	if _, err := pool.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`, roleID, taskUpdateAllow); err != nil {
		t.Fatalf("remove task.update grant before child-only operation: %v", err)
	}
	today := time.Now().UTC().Truncate(time.Minute)
	newActionItemTitle := "Updated without action_item.read or task.update permission"
	if _, err := usecase.NewUpdateActionItemUseCase(sharingTestUOW{pool: pool}, nil).ExecuteOccurrence(
		ctx, actor, task, domain.ActionItemID(actionItemID), today.Format("2006-01-02"), "current",
		usecase.PatchField[string]{Present: true, Value: &newActionItemTitle}, usecase.PatchField[string]{}, usecase.PatchField[time.Time]{},
	); err != nil {
		t.Fatalf("action_item.update with no read or task.update grant: %v", err)
	}
	if err := usecase.NewDeleteTaskUseCase(sharingTestUOW{pool: pool}, nil).Execute(ctx, actor, task, 2); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("task.delete without read/delete allow = %v, want hidden task", err)
	}

	var taskTitle, actionTitle, taskStatus string
	var taskRevision int32
	if err := pool.QueryRow(ctx, `SELECT title, status, revision FROM tasks WHERE id = $1`, taskID).Scan(&taskTitle, &taskStatus, &taskRevision); err != nil {
		t.Fatalf("read task after operation-only child update: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT title FROM action_items WHERE id = $1`, actionItemID).Scan(&actionTitle); err != nil {
		t.Fatalf("read updated actionItem title: %v", err)
	}
	if taskTitle != newTaskTitle || actionTitle != newActionItemTitle || taskStatus != "open" || taskRevision != 3 {
		t.Fatalf("operation-only persisted state task=%q actionItem=%q status=%q revision=%d; want updated titles, open/3", taskTitle, actionTitle, taskStatus, taskRevision)
	}
}

func TestSharedTaskCreationUsesProjectOwnerAndRecordsMemberActor(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	editorID := fixture.user(t)
	fixture.addMember(t, editorID, "editor")
	fixture.commit(t)

	taskID := ulid.Make().String()
	created, err := usecase.NewCreateTaskInProjectUseCase(sharingTestUOW{pool: pool}, nil).Execute(ctx, usecase.CreateTaskInProjectInput{
		ID:        domain.TaskID(taskID),
		UserID:    domain.UserID(editorID),
		ProjectID: domain.ProjectID(fixture.projectID),
		Title:     "Created by shared editor",
	})
	if err != nil {
		t.Fatalf("create task as shared editor: %v", err)
	}
	if created.UserID != fixture.ownerID || created.AssigneeID != fixture.ownerID || created.ChangedBy != editorID || created.Revision != 1 {
		t.Fatalf("shared create result owner=%q assignee=%q actor=%q revision=%d; want project owner, owner assignment, editor actor, revision 1", created.UserID, created.AssigneeID, created.ChangedBy, created.Revision)
	}
	var snapshotOwner, snapshotAssignee, snapshotActor string
	if err := pool.QueryRow(ctx, `
		SELECT user_id, assignee_id, changed_by
		FROM task_revisions
		WHERE id = $1 AND revision = 1
	`, taskID).Scan(&snapshotOwner, &snapshotAssignee, &snapshotActor); err != nil {
		t.Fatalf("read initial shared task snapshot: %v", err)
	}
	if snapshotOwner != fixture.ownerID || snapshotAssignee != fixture.ownerID || snapshotActor != editorID {
		t.Fatalf("shared task snapshot owner=%q assignee=%q actor=%q; want owner, owner, editor", snapshotOwner, snapshotAssignee, snapshotActor)
	}
}

func TestSharedActionItemReadWriteAndDeletePermissions(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	viewerID, editorID, adminID := fixture.user(t), fixture.user(t), fixture.user(t)
	fixture.addMember(t, viewerID, "viewer")
	fixture.addMember(t, editorID, "editor")
	fixture.addMember(t, adminID, "admin")
	taskID := fixture.task(t, viewerID, "Shared child task")
	initialActionItemID := fixture.actionItem(t, taskID)
	fixture.commit(t)

	uow := sharingTestUOW{pool: pool}
	today := time.Now().UTC().Truncate(time.Minute)
	date := today.Format("2006-01-02")
	pageRequest := usecase.CursorPageRequest{Size: 10, FromDate: date}
	actionItemList := usecase.NewListActionItemsUseCase(uow, nil)
	viewerActionItems, err := actionItemList.Execute(ctx, domain.UserID(viewerID), domain.TaskID(taskID))
	if err != nil || len(viewerActionItems) != 1 || viewerActionItems[0].ID != initialActionItemID {
		t.Fatalf("viewer actionItem list = %+v, error %v; want initial shared actionItem", viewerActionItems, err)
	}
	viewerActionItemPage, err := actionItemList.ExecutePage(ctx, domain.UserID(viewerID), domain.TaskID(taskID), pageRequest)
	if err != nil || len(viewerActionItemPage.Items) != 1 || viewerActionItemPage.Items[0].ID != initialActionItemID {
		t.Fatalf("viewer actionItem page = %+v, error %v; want initial shared actionItem", viewerActionItemPage, err)
	}

	timezones := sharingTimezoneReader{}
	viewerCreate := usecase.NewCreateActionItemUseCase(uow, timezones, nil)
	if _, err := viewerCreate.Execute(ctx, usecase.CreateActionItemInput{
		ID: domain.ActionItemID(ulid.Make().String()), UserID: domain.UserID(viewerID), TaskID: domain.TaskID(taskID), Title: "Viewer write must fail",
	}); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("viewer assignee create-actionItem error = %v, want permission denied", err)
	}

	createdActionItem, err := usecase.NewCreateActionItemUseCase(uow, timezones, nil).Execute(ctx, usecase.CreateActionItemInput{
		ID: domain.ActionItemID(ulid.Make().String()), UserID: domain.UserID(editorID), TaskID: domain.TaskID(taskID),
		Title: "Editor created actionItem", DueDate: today,
	})
	if err != nil {
		t.Fatalf("create actionItem as shared editor: %v", err)
	}

	updatedActionItemTitle := "Editor updated actionItem"
	updatedActionItem, err := usecase.NewUpdateActionItemUseCase(uow, nil).ExecuteOccurrence(
		ctx, domain.UserID(editorID), domain.TaskID(taskID), domain.ActionItemID(createdActionItem.SeriesID), createdActionItem.OccurrenceDate, "current",
		usecase.PatchField[string]{Present: true, Value: &updatedActionItemTitle}, usecase.PatchField[string]{}, usecase.PatchField[time.Time]{},
	)
	if err != nil || updatedActionItem.Title != updatedActionItemTitle {
		t.Fatalf("edit shared actionItem = %+v, error %v; want title %q", updatedActionItem, err, updatedActionItemTitle)
	}

	deleteActionItem := usecase.NewDeleteActionItemUseCase(uow, nil)
	if err := deleteActionItem.Execute(ctx, domain.UserID(editorID), domain.TaskID(taskID), domain.ActionItemID(createdActionItem.ID)); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("editor actionItem deletion error = %v, want permission denied", err)
	}
	if err := deleteActionItem.Execute(ctx, domain.UserID(adminID), domain.TaskID(taskID), domain.ActionItemID(createdActionItem.ID)); err != nil {
		t.Fatalf("admin actionItem deletion: %v", err)
	}
}

func TestSharedEditorCanAttachOnlyProjectOwnerTags(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	editorID, viewerID := fixture.user(t), fixture.user(t)
	fixture.addMember(t, editorID, "editor")
	fixture.addMember(t, viewerID, "viewer")
	taskID := fixture.task(t, fixture.ownerID, "Shared task with owner tags")
	ownerTagID, editorTagID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.tx.Exec(ctx, `
		INSERT INTO tags (id, user_id, name)
		VALUES ($1, $2, 'Owner tag'), ($3, $4, 'Editor tag')
	`, ownerTagID, fixture.ownerID, editorTagID, editorID); err != nil {
		t.Fatalf("insert owner and editor tags: %v", err)
	}
	fixture.commit(t)

	uow := sharingTestUOW{pool: pool}
	addTag := usecase.NewAddTagToTaskUseCase(uow, nil)
	removeTag := usecase.NewRemoveTagFromTaskUseCase(uow, nil)
	actorID, taskDomainID := domain.UserID(editorID), domain.TaskID(taskID)
	if err := addTag.Execute(ctx, actorID, taskDomainID, ownerTagID); err != nil {
		t.Fatalf("shared editor attaches project owner's task tag: %v", err)
	}
	if err := addTag.Execute(ctx, actorID, taskDomainID, editorTagID); !errors.Is(err, usecase.ErrTaskTagAssignmentNotOwned) {
		t.Fatalf("shared editor attaches own tag to owner's task = %v, want ownership error", err)
	}
	if err := addTag.Execute(ctx, domain.UserID(viewerID), taskDomainID, ownerTagID); err == nil {
		t.Fatal("viewer successfully attached a tag to a shared task")
	}
	var assignmentCount int32
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM task_tag_assignments WHERE task_id = $1`, taskID).Scan(&assignmentCount); err != nil {
		t.Fatalf("count shared task tag assignments: %v", err)
	}
	if assignmentCount != 1 {
		t.Fatalf("shared task has %d tag assignments after allowed and rejected adds, want only owner tag", assignmentCount)
	}
	if err := removeTag.Execute(ctx, actorID, taskDomainID, ownerTagID); err != nil {
		t.Fatalf("shared editor removes project owner's task tag: %v", err)
	}
	if err := removeTag.Execute(ctx, domain.UserID(viewerID), taskDomainID, ownerTagID); err == nil {
		t.Fatal("viewer successfully removed a tag from a shared task")
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM task_tag_assignments WHERE task_id = $1`, taskID).Scan(&assignmentCount); err != nil {
		t.Fatalf("count tag assignments after allowed removal: %v", err)
	}
	if assignmentCount != 0 {
		t.Fatalf("shared task has %d tag assignments after editor removal, want zero", assignmentCount)
	}
}

type sharingTimezoneReader struct{}

func (sharingTimezoneReader) GetTimezone(context.Context, string) (string, error) {
	return "UTC", nil
}

func TestTaskRevisionSnapshotsRecordInitialAndLatestActor(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	actorID := fixture.user(t)
	fixture.addMember(t, actorID, "editor")
	taskID := fixture.task(t, actorID, "Initial title")

	var currentRevision, snapshotRevision int32
	var currentChangedBy, snapshotChangedBy string
	if err := fixture.tx.QueryRow(ctx, `SELECT revision, changed_by FROM tasks WHERE id = $1`, taskID).Scan(&currentRevision, &currentChangedBy); err != nil {
		t.Fatalf("read initial task: %v", err)
	}
	if err := fixture.tx.QueryRow(ctx, `SELECT revision, changed_by FROM task_revisions WHERE id = $1 AND revision = 1`, taskID).Scan(&snapshotRevision, &snapshotChangedBy); err != nil {
		t.Fatalf("read initial task snapshot: %v", err)
	}
	if currentRevision != 1 || snapshotRevision != 1 || currentChangedBy != fixture.ownerID || snapshotChangedBy != fixture.ownerID {
		t.Fatalf("initial task revision/current actor = %d/%q snapshot = %d/%q; want revision 1 and owner %q", currentRevision, currentChangedBy, snapshotRevision, snapshotChangedBy, fixture.ownerID)
	}

	if _, err := fixture.tx.Exec(ctx, `UPDATE tasks SET title = 'Edited task', changed_by = $2 WHERE id = $1`, taskID, actorID); err != nil {
		t.Fatalf("edit task as member: %v", err)
	}
	var revision, snapshots int32
	var title, changedBy, latestTitle, latestChangedBy string
	var updatedAt, latestChangedAt time.Time
	if err := fixture.tx.QueryRow(ctx, `SELECT revision, title, changed_by, updated_at FROM tasks WHERE id = $1`, taskID).Scan(&revision, &title, &changedBy, &updatedAt); err != nil {
		t.Fatalf("read latest task: %v", err)
	}
	if err := fixture.tx.QueryRow(ctx, `
		SELECT count(*)::integer,
		       (array_agg(title ORDER BY revision DESC))[1],
		       (array_agg(changed_by ORDER BY revision DESC))[1],
		       (array_agg(changed_at ORDER BY revision DESC))[1]
		FROM task_revisions WHERE id = $1
	`, taskID).Scan(&snapshots, &latestTitle, &latestChangedBy, &latestChangedAt); err != nil {
		t.Fatalf("read latest task snapshot: %v", err)
	}
	if revision != 2 || snapshots != 2 || title != latestTitle || changedBy != actorID || latestChangedBy != actorID || !updatedAt.Equal(latestChangedAt) {
		t.Fatalf("latest task = revision %d title %q actor %q at %s; snapshots=%d latest snapshot=%q by %q at %s; want revision 2, member actor, matching update time", revision, title, changedBy, updatedAt, snapshots, latestTitle, latestChangedBy, latestChangedAt)
	}
}

func TestConcurrentTaskRevisionWritersProduceOneWinnerAndOneConflict(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	firstMemberID, secondMemberID := fixture.user(t), fixture.user(t)
	fixture.addMember(t, firstMemberID, "viewer")
	fixture.addMember(t, secondMemberID, "editor")
	taskID := fixture.task(t, fixture.ownerID, "CAS task")
	fixture.commit(t)

	assign := usecase.NewAssignTaskUseCase(sharingTestUOW{pool: pool}, nil)
	raceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	start := make(chan struct{})
	type result struct {
		assigneeID string
		err        error
	}
	results := make(chan result, 2)
	for _, assigneeID := range []string{firstMemberID, secondMemberID} {
		assigneeID := assigneeID
		go func() {
			<-start
			_, err := assign.Execute(raceCtx, domain.UserID(fixture.ownerID), domain.TaskID(taskID), domain.UserID(assigneeID), 1)
			results <- result{assigneeID: assigneeID, err: err}
		}()
	}
	close(start)

	winners := make([]string, 0, 1)
	conflicts := 0
	for received := 0; received < 2; received++ {
		var got result
		select {
		case got = <-results:
		case <-raceCtx.Done():
			t.Fatalf("task revision writer race exceeded deadline: %v", raceCtx.Err())
		}
		if got.err == nil {
			winners = append(winners, got.assigneeID)
			continue
		}
		if errors.Is(got.err, usecase.ErrRevisionConflict) {
			conflicts++
			continue
		}
		t.Fatalf("assignment to %s returned unexpected error: %v", got.assigneeID, got.err)
	}
	if len(winners) != 1 || conflicts != 1 {
		t.Fatalf("assignment race produced %d winners and %d revision conflicts; want one each", len(winners), conflicts)
	}

	final, err := taskrepo.NewTaskRepository(pool).GetByUserID(ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID))
	if err != nil {
		t.Fatalf("read task after revision race: %v", err)
	}
	if final.Revision != 2 || final.AssigneeID != winners[0] || final.ChangedBy != fixture.ownerID {
		t.Fatalf("task after revision race = revision %d assignee %q changed_by %q; want winner %q at revision 2", final.Revision, final.AssigneeID, final.ChangedBy, winners[0])
	}
	var snapshots int32
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM task_revisions WHERE id = $1`, taskID).Scan(&snapshots); err != nil {
		t.Fatalf("count task revisions after race: %v", err)
	}
	if snapshots != 2 {
		t.Fatalf("task history has %d snapshots, want exactly initial + one successful update", snapshots)
	}
}

func TestConcurrentTaskAssignmentAndProjectMoveKeepAssigneeEligible(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	memberID := fixture.user(t)
	fixture.addMember(t, memberID, "editor")
	targetProjectID := fixture.addProject(t, "Move target")
	taskID := fixture.task(t, fixture.ownerID, "Assignment and move race")
	fixture.commit(t)

	uow := sharingTestUOW{pool: pool}
	assign := usecase.NewAssignTaskUseCase(uow, nil)
	move := usecase.NewAddTaskToProjectUseCase(uow, taskrepo.NewTaskRepository(pool), nil)
	raceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	start := make(chan struct{})
	assignmentResult := make(chan error, 1)
	moveResult := make(chan error, 1)
	go func() {
		<-start
		_, err := assign.Execute(raceCtx, domain.UserID(fixture.ownerID), domain.TaskID(taskID), domain.UserID(memberID), 1)
		assignmentResult <- err
	}()
	go func() {
		<-start
		_, err := move.Execute(raceCtx, domain.UserID(fixture.ownerID), domain.ProjectID(targetProjectID), domain.TaskID(taskID), 1)
		moveResult <- err
	}()
	close(start)

	var assignmentErr, moveErr error
	for received := 0; received < 2; received++ {
		select {
		case assignmentErr = <-assignmentResult:
		case moveErr = <-moveResult:
		case <-raceCtx.Done():
			t.Fatalf("assignment/move race exceeded deadline: %v", raceCtx.Err())
		}
	}
	if assignmentErr != nil && !errors.Is(assignmentErr, usecase.ErrRevisionConflict) && !errors.Is(assignmentErr, usecase.ErrPermissionDenied) {
		t.Fatalf("assignment race error = %v; want success, revision conflict, or changed-project eligibility denial", assignmentErr)
	}
	if moveErr != nil && !errors.Is(moveErr, usecase.ErrRevisionConflict) {
		t.Fatalf("project move race error = %v; want success or revision conflict", moveErr)
	}
	if (assignmentErr == nil) == (moveErr == nil) {
		t.Fatalf("race results assignment=%v move=%v; want exactly one successful revision-1 mutation", assignmentErr, moveErr)
	}

	var currentProjectID, currentAssignee, changedBy string
	var revision, historyCount int32
	if err := pool.QueryRow(ctx, `SELECT project_id, assignee_id, changed_by, revision FROM tasks WHERE id = $1`, taskID).Scan(&currentProjectID, &currentAssignee, &changedBy, &revision); err != nil {
		t.Fatalf("read task after assignment/move race: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM task_revisions WHERE id = $1`, taskID).Scan(&historyCount); err != nil {
		t.Fatalf("count task history after assignment/move race: %v", err)
	}
	if currentProjectID == fixture.projectID {
		if assignmentErr != nil || !errors.Is(moveErr, usecase.ErrRevisionConflict) || currentAssignee != memberID {
			t.Fatalf("assignment won with project=%q assignee=%q actor=%q errors assignment=%v move=%v", currentProjectID, currentAssignee, changedBy, assignmentErr, moveErr)
		}
	} else if currentProjectID == targetProjectID {
		if moveErr != nil || assignmentErr == nil || currentAssignee != fixture.ownerID {
			t.Fatalf("move won with project=%q assignee=%q actor=%q errors assignment=%v move=%v", currentProjectID, currentAssignee, changedBy, assignmentErr, moveErr)
		}
	} else {
		t.Fatalf("task project after race = %q, want source %q or target %q", currentProjectID, fixture.projectID, targetProjectID)
	}
	if revision != 2 || historyCount != 2 || changedBy != fixture.ownerID {
		t.Fatalf("task revision/history/actor after race = %d/%d/%q; want 2/2/owner", revision, historyCount, changedBy)
	}
}

func TestDeleteTaskSoftDeletesActionItems(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	taskID := fixture.task(t, fixture.ownerID, "Task to delete")
	actionItemID := fixture.actionItem(t, taskID)
	fixture.commit(t)

	remove := usecase.NewDeleteTaskUseCase(sharingTestUOW{pool: pool}, nil)
	if err := remove.Execute(ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID), 1); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	assertDeleted := func(table, id string) {
		t.Helper()
		var deleted bool
		if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM `+table+` WHERE id = $1`, id).Scan(&deleted); err != nil {
			t.Fatalf("read deleted %s row: %v", table, err)
		}
		if !deleted {
			t.Errorf("%s %s remains active after task deletion", table, id)
		}
	}
	assertDeleted("tasks", taskID)
	assertDeleted("action_items", actionItemID)

	var revision, snapshots int32
	var changedBy string
	if err := pool.QueryRow(ctx, `SELECT revision, changed_by FROM tasks WHERE id = $1`, taskID).Scan(&revision, &changedBy); err != nil {
		t.Fatalf("read deleted task revision: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM task_revisions WHERE id = $1`, taskID).Scan(&snapshots); err != nil {
		t.Fatalf("count task snapshots: %v", err)
	}
	if revision != 2 || snapshots != 2 || changedBy != fixture.ownerID {
		t.Fatalf("deleted task revision/history/actor = %d/%d/%q; want 2/2/owner", revision, snapshots, changedBy)
	}
	if err := remove.Execute(ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID), revision); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("second task mutation error = %v, want deleted task not found", err)
	}
	var afterRetrySnapshots int32
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM task_revisions WHERE id = $1`, taskID).Scan(&afterRetrySnapshots); err != nil {
		t.Fatalf("count task snapshots after rejected mutation: %v", err)
	}
	if afterRetrySnapshots != snapshots {
		t.Fatalf("rejected deleted-task mutation added history: before=%d after=%d", snapshots, afterRetrySnapshots)
	}
	if err := usecase.NewDeleteActionItemUseCase(sharingTestUOW{pool: pool}, nil).Execute(
		ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID), domain.ActionItemID(actionItemID),
	); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("child mutation under deleted task error = %v, want task not found", err)
	}
}

func TestTaskRevisionHistoryAccessTracksMembershipAndDeletion(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	viewerID, removedID, adminID := fixture.user(t), fixture.user(t), fixture.user(t)
	fixture.addMember(t, viewerID, "viewer")
	fixture.addMember(t, removedID, "editor")
	fixture.addMember(t, adminID, "admin")
	taskID := fixture.task(t, fixture.ownerID, "History ACL task")
	fixture.commit(t)

	history := usecase.NewListTaskRevisionsUseCase(taskrepo.NewTaskRepository(pool), nil)
	pageRequest := usecase.CursorPageRequest{Size: 10}
	for _, actorID := range []string{viewerID, removedID, adminID} {
		page, err := history.Execute(ctx, domain.UserID(actorID), domain.TaskID(taskID), pageRequest)
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("read active task history as %s: items=%d error=%v; want initial snapshot", actorID, len(page.Items), err)
		}
	}

	if _, err := pool.Exec(ctx, `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`, fixture.projectID, removedID); err != nil {
		t.Fatalf("remove member before task history check: %v", err)
	}
	if _, err := history.Execute(ctx, domain.UserID(removedID), domain.TaskID(taskID), pageRequest); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("removed member task history error = %v, want task not found", err)
	}

	if err := usecase.NewDeleteTaskUseCase(sharingTestUOW{pool: pool}, nil).Execute(ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID), 1); err != nil {
		t.Fatalf("delete task for history ACL check: %v", err)
	}
	for _, actor := range []struct {
		id          string
		wantHistory bool
	}{
		{id: fixture.ownerID, wantHistory: true},
		{id: adminID, wantHistory: true},
		{id: viewerID, wantHistory: false},
		{id: removedID, wantHistory: false},
	} {
		page, err := history.Execute(ctx, domain.UserID(actor.id), domain.TaskID(taskID), pageRequest)
		if actor.wantHistory {
			if err != nil || len(page.Items) != 2 {
				t.Fatalf("read deleted task history as %s: items=%d error=%v; want two snapshots", actor.id, len(page.Items), err)
			}
			continue
		}
		if !errors.Is(err, usecase.ErrTaskNotFound) {
			t.Errorf("deleted task history access as %s error = %v, want task not found", actor.id, err)
		}
	}
}

type usecaseTaskResult struct {
	userID     string
	assigneeID string
	changedBy  string
}

type sharingFixture struct {
	tx         pgx.Tx
	pool       *pgxpool.Pool
	ownerID    string
	projectID  string
	projectIDs []string
	userIDs    []string
}

func newSharingFixture(t *testing.T, pool *pgxpool.Pool) *sharingFixture {
	t.Helper()
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin sharing test transaction: %v", err)
	}
	fixture := &sharingFixture{
		tx:        tx,
		pool:      pool,
		ownerID:   ulid.Make().String(),
		projectID: ulid.Make().String(),
	}
	fixture.projectIDs = append(fixture.projectIDs, fixture.projectID)
	fixture.userIDs = append(fixture.userIDs, fixture.ownerID)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if fixture.tx != nil {
			_ = fixture.tx.Rollback(cleanupCtx)
		}
		_, _ = pool.Exec(cleanupCtx, `
			DELETE FROM task_revisions
			WHERE id IN (SELECT id FROM tasks WHERE project_id = ANY($1::text[]))
		`, fixture.projectIDs)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_revisions WHERE id = ANY($1::text[])`, fixture.projectIDs)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM tasks WHERE project_id = ANY($1::text[])`, fixture.projectIDs)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_members WHERE project_id = ANY($1::text[])`, fixture.projectIDs)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM projects WHERE id = ANY($1::text[])`, fixture.projectIDs)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::text[])`, fixture.userIDs)
	})

	fixture.insertUser(t, fixture.ownerID)
	if _, err := tx.Exec(ctx, `
		INSERT INTO projects (id, user_id, type, title, priority, changed_by)
		VALUES ($1, $2, 'other', 'Shared project', 'low', $2)
	`, fixture.projectID, fixture.ownerID); err != nil {
		t.Fatalf("insert shared project: %v", err)
	}
	return fixture
}

func (fixture *sharingFixture) user(t *testing.T) string {
	t.Helper()
	userID := ulid.Make().String()
	fixture.userIDs = append(fixture.userIDs, userID)
	fixture.insertUser(t, userID)
	return userID
}

func (fixture *sharingFixture) insertUser(t *testing.T, userID string) {
	t.Helper()
	if _, err := fixture.tx.Exec(t.Context(), `
		INSERT INTO users (id, first_name, last_name, email, password)
		VALUES ($1, 'Sharing', 'Test', $2, 'not-used')
	`, userID, userID+"@sharing.test"); err != nil {
		t.Fatalf("insert sharing test user: %v", err)
	}
}

func (fixture *sharingFixture) addMember(t *testing.T, userID, roleID string) {
	t.Helper()
	if _, err := fixture.tx.Exec(t.Context(), `
		INSERT INTO project_members (project_id, user_id, role_id, added_by)
		VALUES ($1, $2, $3, $4)
	`, fixture.projectID, userID, roleID, fixture.ownerID); err != nil {
		t.Fatalf("add %s member: %v", roleID, err)
	}
}

func (fixture *sharingFixture) customRole(t *testing.T, purpose string) string {
	t.Helper()
	roleID := "sharing_test_" + purpose + "_" + strings.ToLower(ulid.Make().String())
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO roles (role_id, name) VALUES ($1, $2)`, roleID, roleID); err != nil {
		t.Fatalf("insert isolated sharing test role: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(cleanupCtx, `DELETE FROM project_members WHERE project_id = $1 AND role_id = $2`, fixture.projectID, roleID)
		_, _ = fixture.pool.Exec(cleanupCtx, `DELETE FROM roles WHERE role_id = $1`, roleID)
	})
	return roleID
}

func (fixture *sharingFixture) addProject(t *testing.T, title string) string {
	t.Helper()
	projectID := ulid.Make().String()
	if _, err := fixture.tx.Exec(t.Context(), `
		INSERT INTO projects (id, user_id, type, title, priority, changed_by)
		VALUES ($1, $2, 'other', $3, 'low', $2)
	`, projectID, fixture.ownerID, title); err != nil {
		t.Fatalf("insert additional sharing test project: %v", err)
	}
	fixture.projectIDs = append(fixture.projectIDs, projectID)
	return projectID
}

func (fixture *sharingFixture) task(t *testing.T, assigneeID, title string) string {
	t.Helper()
	taskID := ulid.Make().String()
	if _, err := fixture.tx.Exec(t.Context(), `
		INSERT INTO tasks (id, user_id, project_id, assignee_id, title, changed_by)
		VALUES ($1, $2, $3, $4, $5, $2)
	`, taskID, fixture.ownerID, fixture.projectID, assigneeID, title); err != nil {
		t.Fatalf("insert sharing test task: %v", err)
	}
	return taskID
}

func (fixture *sharingFixture) actionItem(t *testing.T, taskID string) string {
	t.Helper()
	date := time.Now().UTC().Format("2006-01-02")
	actionItemID := ulid.Make().String()
	if _, err := fixture.tx.Exec(t.Context(), `
		INSERT INTO action_items (id, task_id, title, position, series_id, occurrence_date, timezone)
		VALUES ($1, $2, 'ActionItem child', 0, $1, $3::date, 'UTC')
	`, actionItemID, taskID, date); err != nil {
		t.Fatalf("insert actionItem child: %v", err)
	}
	return actionItemID
}

func (fixture *sharingFixture) commit(t *testing.T) {
	t.Helper()
	if fixture.tx == nil {
		t.Fatal("sharing fixture transaction already committed")
	}
	if err := fixture.tx.Commit(t.Context()); err != nil {
		t.Fatalf("commit sharing fixture: %v", err)
	}
	fixture.tx = nil
}

type sharingTestRepositories struct {
	usecase.Repositories
	tasks        *taskrepo.TaskRepository
	taskTags     *taskrepo.TaskTagRepository
	actionItems  *taskrepo.ActionItemRepository
	taskProjects usecase.TaskProjectRepository
	lifecycle    shared.ProjectWorkLifecycle
}

func (repositories sharingTestRepositories) ProjectLifecycle() shared.ProjectWorkLifecycle {
	return repositories.lifecycle
}

type sharingTestTaskProjects struct{ queries *sqlc.Queries }

func (r sharingTestTaskProjects) GetProjectByUserID(ctx context.Context, actorID, projectID string) (usecase.TaskProject, error) {
	return r.GetProjectByUserIDWithPermission(ctx, actorID, projectID, shared.TaskRead())
}

func (r sharingTestTaskProjects) GetProjectByUserIDWithPermission(ctx context.Context, actorID, projectID string, capability shared.Capability) (usecase.TaskProject, error) {
	project, err := r.queries.GetProjectByUserIDForPermission(ctx, sqlc.GetProjectByUserIDForPermissionParams{
		ID: projectID, ActorID: actorID, ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if err != nil {
		return usecase.TaskProject{}, r.projectLookupError(ctx, actorID, projectID, capability, err)
	}
	return usecase.TaskProject{ID: project.ID, OwnerID: project.UserID, DefaultPriority: project.Priority}, nil
}

func (r sharingTestTaskProjects) LockProjectByUserIDWithPermission(ctx context.Context, actorID, projectID string, capability shared.Capability) (usecase.TaskProject, error) {
	project, err := r.queries.LockProjectByUserIDForPermission(ctx, sqlc.LockProjectByUserIDForPermissionParams{
		ID: projectID, UserID: actorID, ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if err != nil {
		return usecase.TaskProject{}, r.projectLookupError(ctx, actorID, projectID, capability, err)
	}
	return usecase.TaskProject{ID: project.ID, OwnerID: project.UserID, DefaultPriority: project.Priority}, nil
}

func (r sharingTestTaskProjects) projectLookupError(ctx context.Context, actorID, projectID string, capability shared.Capability, err error) error {
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	allowed, permissionErr := r.queries.HasProjectPermission(ctx, sqlc.HasProjectPermissionParams{
		ActorID: actorID, ProjectID: projectID, ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(permissionErr, pgx.ErrNoRows) {
		return usecase.ErrTaskProjectNotFound
	}
	if permissionErr != nil {
		return permissionErr
	}
	if !allowed {
		return usecase.ErrPermissionDenied
	}
	return usecase.ErrTaskProjectNotFound
}

func (repositories sharingTestRepositories) TaskProjects() usecase.TaskProjectRepository {
	return repositories.taskProjects
}

func (repositories sharingTestRepositories) Tasks() usecase.TaskRepository {
	return repositories.tasks
}

func (repositories sharingTestRepositories) TaskTags() usecase.TaskTagRepository {
	return repositories.taskTags
}

func (repositories sharingTestRepositories) ActionItems() usecase.ActionItemRepository {
	return repositories.actionItems
}

type sharingTestUOW struct {
	pool *pgxpool.Pool
}

func (uow sharingTestUOW) Do(ctx context.Context, run func(context.Context, usecase.Repositories) error) error {
	tx, err := uow.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repositories := sharingTestRepositories{
		tasks:        taskrepo.NewTaskRepository(tx),
		taskTags:     taskrepo.NewTaskTagRepository(tx),
		actionItems:  taskrepo.NewActionItemRepository(tx),
		taskProjects: sharingTestTaskProjects{queries: sqlc.New(tx)},
	}
	projectStore := projectrepo.NewProjectRepository(tx)
	repositories.lifecycle = projectusecase.NewProjectLifecycleUseCase(projectStore, projectStore)
	if err := run(ctx, repositories); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
