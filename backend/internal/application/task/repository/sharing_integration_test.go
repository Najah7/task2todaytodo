package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func TestProjectReadPermissionUsesCurrentRolePermissions(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	viewerID := fixture.user(t)
	fixture.addMember(t, viewerID, "viewer")

	projects := NewProjectRepository(fixture.tx)
	project, err := projects.GetByUserID(ctx, domain.UserID(viewerID), domain.ProjectID(fixture.projectID))
	if err != nil || project.ID != fixture.projectID {
		t.Fatalf("viewer project read = %+v, error %v; want shared project", project, err)
	}

	projectReadAllow := sharingPermissionID(t, fixture.tx, "project", "read", "allow")
	projectReadDeny := sharingPermissionID(t, fixture.tx, "project", "read", "deny")
	projectUpdateDeny := sharingPermissionID(t, fixture.tx, "project", "update", "deny")
	taskReadDeny := sharingPermissionID(t, fixture.tx, "task", "read", "deny")

	// A deny for another resource or action must not affect project.read.
	if _, err := fixture.tx.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		VALUES ('viewer', $1), ('viewer', $2)
	`, projectUpdateDeny, taskReadDeny); err != nil {
		t.Fatalf("add unrelated viewer deny rules: %v", err)
	}
	if _, err := projects.GetByUserID(ctx, domain.UserID(viewerID), domain.ProjectID(fixture.projectID)); err != nil {
		t.Fatalf("unrelated action/resource denies blocked project read: %v", err)
	}

	// A matching deny overrides an existing allow, and the policy is read from
	// current database state on each request.
	if _, err := fixture.tx.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		VALUES ('viewer', $1)
	`, projectReadDeny); err != nil {
		t.Fatalf("add matching viewer deny: %v", err)
	}
	if _, err := projects.GetByUserID(ctx, domain.UserID(viewerID), domain.ProjectID(fixture.projectID)); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("viewer read with matching allow and deny = %v, want project not found", err)
	}
	if _, err := projects.GetByUserID(ctx, domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID)); err != nil {
		t.Fatalf("matching member deny blocked owner's implicit read: %v", err)
	}

	if _, err := fixture.tx.Exec(ctx, `
		DELETE FROM role_permissions
		WHERE role_id = 'viewer' AND permission_id = $1
	`, projectReadDeny); err != nil {
		t.Fatalf("remove matching viewer deny: %v", err)
	}
	if _, err := fixture.tx.Exec(ctx, `
		DELETE FROM role_permissions
		WHERE role_id = 'viewer' AND permission_id = $1
	`, projectReadAllow); err != nil {
		t.Fatalf("remove viewer read allow: %v", err)
	}
	if _, err := projects.GetByUserID(ctx, domain.UserID(viewerID), domain.ProjectID(fixture.projectID)); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("viewer read without matching allow = %v, want project not found", err)
	}
	if _, err := fixture.tx.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		VALUES ('viewer', $1)
	`, projectReadAllow); err != nil {
		t.Fatalf("restore viewer read allow: %v", err)
	}
	if _, err := projects.GetByUserID(ctx, domain.UserID(viewerID), domain.ProjectID(fixture.projectID)); err != nil {
		t.Fatalf("viewer read after current allow restored: %v", err)
	}
}

func TestProjectMemberCreateAndUpdatePermissionsAreDistinctAndDynamic(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	editorID := fixture.user(t)
	existingMemberID, newMemberID, deniedNewMemberID := fixture.user(t), fixture.user(t), fixture.user(t)
	memberAdminRoleID := fixture.customRole(t, "project_member")
	fixture.addMember(t, editorID, memberAdminRoleID)
	fixture.addMember(t, existingMemberID, "viewer")
	fixture.commit(t)

	createAllow := sharingPermissionID(t, pool, "project_member", "create", "allow")
	createDeny := sharingPermissionID(t, pool, "project_member", "create", "deny")
	updateAllow := sharingPermissionID(t, pool, "project_member", "update", "allow")
	updateDeny := sharingPermissionID(t, pool, "project_member", "update", "deny")
	grant := func(permissionID int64) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, memberAdminRoleID, permissionID); err != nil {
			t.Fatalf("grant custom project_member role permission %d: %v", permissionID, err)
		}
	}
	upsert := usecase.NewUpsertProjectMemberUseCase(sharingTestUOW{pool: pool}, nil)
	projectID := domain.ProjectID(fixture.projectID)
	actorID := domain.UserID(editorID)

	grant(createAllow)
	if err := upsert.Execute(ctx, actorID, projectID, domain.UserID(newMemberID), "viewer"); err != nil {
		t.Fatalf("editor with project_member.create adds a new member: %v", err)
	}
	if err := upsert.Execute(ctx, actorID, projectID, domain.UserID(existingMemberID), "admin"); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("project_member.create incorrectly allowed changing an existing member role: %v", err)
	}
	assertMemberRole := func(memberID, wantRole string) {
		t.Helper()
		var gotRole string
		if err := pool.QueryRow(ctx, `SELECT role_id FROM project_members WHERE project_id = $1 AND user_id = $2`, fixture.projectID, memberID).Scan(&gotRole); err != nil {
			t.Fatalf("read role for member %s: %v", memberID, err)
		}
		if gotRole != wantRole {
			t.Fatalf("member %s role = %q, want %q", memberID, gotRole, wantRole)
		}
	}
	assertMemberRole(existingMemberID, "viewer")

	grant(updateAllow)
	if err := upsert.Execute(ctx, actorID, projectID, domain.UserID(existingMemberID), "admin"); err != nil {
		t.Fatalf("new project_member.update grant did not take effect: %v", err)
	}
	assertMemberRole(existingMemberID, "admin")

	grant(updateDeny)
	if err := upsert.Execute(ctx, actorID, projectID, domain.UserID(existingMemberID), "viewer"); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("matching project_member.update deny did not override allow: %v", err)
	}
	assertMemberRole(existingMemberID, "admin")

	grant(createDeny)
	if err := upsert.Execute(ctx, actorID, projectID, domain.UserID(deniedNewMemberID), "viewer"); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("matching project_member.create deny did not override allow: %v", err)
	}
	var deniedMemberExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2)`, fixture.projectID, deniedNewMemberID).Scan(&deniedMemberExists); err != nil {
		t.Fatalf("check denied member creation: %v", err)
	}
	if deniedMemberExists {
		t.Fatal("member was added despite a matching project_member.create deny")
	}
}

func sharingPermissionID(t *testing.T, tx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, resource, action, effect string) int64 {
	t.Helper()
	var permissionID int64
	if err := tx.QueryRow(t.Context(), `
		SELECT permission_id
		FROM permissions
		WHERE resource_id = $1 AND action = $2::permission_action AND effect = $3::permission_effect
	`, resource, action, effect).Scan(&permissionID); err != nil {
		t.Fatalf("find %s.%s %s permission: %v", resource, action, effect, err)
	}
	return permissionID
}

func TestTaskListIsAssignedOnlyWhileProjectTaskListIncludesAuthorizedTasks(t *testing.T) {
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
	tasks := NewTaskRepository(pool)

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

	projectTasks, err := tasks.ListByProjectAndUserIDCursor(ctx, domain.UserID(viewerID), domain.ProjectID(fixture.projectID), 10, nil)
	if err != nil {
		t.Fatalf("list viewer's authorized project tasks: %v", err)
	}
	if len(projectTasks) != 3 {
		t.Fatalf("viewer project task list has %d tasks, want all three regardless of assignee", len(projectTasks))
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
	todoID, scheduleID := fixture.addChildren(t, taskID)
	if _, err := fixture.tx.Exec(ctx, `UPDATE todo_items SET completed = true WHERE id = $1`, todoID); err != nil {
		t.Fatalf("complete operation-only todo fixture: %v", err)
	}
	fixture.commit(t)

	var taskUpdateAllow int64
	for _, grant := range []struct{ resource, action string }{
		{"project", "update"},
		{"task", "update"},
		{"todo_item", "update"},
		{"task_schedule", "delete"},
	} {
		permissionID := sharingPermissionID(t, pool, grant.resource, grant.action, "allow")
		if grant.resource == "task" && grant.action == "update" {
			taskUpdateAllow = permissionID
		}
		if _, err := pool.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, roleID, permissionID); err != nil {
			t.Fatalf("grant operation-only %s.%s: %v", grant.resource, grant.action, err)
		}
	}

	projectRepo := NewProjectRepository(pool)
	taskRepo := NewTaskRepository(pool)
	actor := domain.UserID(actorID)
	project := domain.ProjectID(fixture.projectID)
	task := domain.TaskID(taskID)
	if _, err := projectRepo.GetByUserID(ctx, actor, project); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("project.read without allow = %v, want hidden project", err)
	}
	if _, err := taskRepo.GetByUserID(ctx, actor, task); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("task.read without allow = %v, want hidden task", err)
	}
	newProjectTitle := "Updated without read permission"
	if _, err := usecase.NewUpdateProjectUseCase(projectRepo, taskRepo, nil).Execute(
		ctx, actor, project, 1, usecase.PatchField[string]{Present: true, Value: &newProjectTitle}, usecase.PatchField[string]{},
		usecase.PatchField[string]{}, usecase.PatchField[string]{}, usecase.PatchField[string]{}, usecase.PatchField[time.Time]{}, usecase.PatchField[time.Time]{},
	); err != nil {
		t.Fatalf("project.update with no project.read grant: %v", err)
	}

	newTaskTitle := "Updated without task.read permission"
	if _, err := usecase.NewUpdateTaskUseCase(taskRepo, taskRepo, nil).Execute(
		ctx, actor, task, 1, usecase.PatchField[string]{Present: true, Value: &newTaskTitle}, usecase.PatchField[string]{},
		usecase.PatchField[time.Time]{}, usecase.PatchField[int]{}, usecase.PatchField[int]{},
	); err != nil {
		t.Fatalf("task.update with no task.read grant: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`, roleID, taskUpdateAllow); err != nil {
		t.Fatalf("remove task.update grant before child-only operations: %v", err)
	}

	today := time.Now().UTC().Truncate(time.Minute)
	newTodoTitle := "Updated without todo_item.read permission"
	if _, err := usecase.NewUpdateTodoItemUseCase(sharingTestUOW{pool: pool}, nil).ExecuteOccurrence(
		ctx, actor, task, domain.TodoItemID(todoID), today.Format("2006-01-02"), "current",
		usecase.PatchField[string]{Present: true, Value: &newTodoTitle}, usecase.PatchField[string]{}, usecase.PatchField[time.Time]{},
	); err != nil {
		t.Fatalf("todo_item.update with no read or task.update grant: %v", err)
	}

	newScheduleTitle := "This schedule update must be denied"
	if _, err := usecase.NewUpdateTaskScheduleUseCase(sharingTestUOW{pool: pool}, nil).ExecuteOccurrence(
		ctx, actor, task, domain.TaskScheduleID(scheduleID), today.Format("2006-01-02"), "current",
		usecase.PatchField[string]{Present: true, Value: &newScheduleTitle}, usecase.PatchField[string]{}, usecase.PatchField[string]{},
	); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("task_schedule.update without read/update allow = %v, want hidden task", err)
	}
	if err := usecase.NewDeleteTaskUseCase(sharingTestUOW{pool: pool}, nil).Execute(ctx, actor, task, 2); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("task.delete without read/delete allow = %v, want hidden task", err)
	}
	if err := usecase.NewDeleteTaskScheduleUseCase(sharingTestUOW{pool: pool}, nil).Execute(ctx, actor, task, domain.TaskScheduleID(scheduleID)); err != nil {
		t.Fatalf("task_schedule.delete without read grant: %v", err)
	}

	var projectTitle, taskTitle, todoTitle, scheduleTitle, taskStatus string
	var scheduleDeleted bool
	if err := pool.QueryRow(ctx, `SELECT title FROM projects WHERE id = $1`, fixture.projectID).Scan(&projectTitle); err != nil {
		t.Fatalf("read updated project title: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT title FROM tasks WHERE id = $1`, taskID).Scan(&taskTitle); err != nil {
		t.Fatalf("read updated task title: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT title FROM todo_items WHERE id = $1`, todoID).Scan(&todoTitle); err != nil {
		t.Fatalf("read updated todo title: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT title, deleted_at IS NOT NULL FROM task_schedules WHERE id = $1`, scheduleID).Scan(&scheduleTitle, &scheduleDeleted); err != nil {
		t.Fatalf("read schedule after operation-only delete: %v", err)
	}
	var taskRevision int32
	if err := pool.QueryRow(ctx, `SELECT status, revision FROM tasks WHERE id = $1`, taskID).Scan(&taskStatus, &taskRevision); err != nil {
		t.Fatalf("read task after operation-only child delete: %v", err)
	}
	if projectTitle != newProjectTitle || taskTitle != newTaskTitle || todoTitle != newTodoTitle || scheduleTitle != "Schedule child" || !scheduleDeleted {
		t.Fatalf("operation-only persisted state project=%q task=%q todo=%q schedule=%q deleted=%t", projectTitle, taskTitle, todoTitle, scheduleTitle, scheduleDeleted)
	}
	if taskStatus != "done" || taskRevision != 3 {
		t.Fatalf("task after schedule deletion status=%q revision=%d, want derived done/revision 3 under task_schedule.delete grant", taskStatus, taskRevision)
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

func TestSharedChildReadWriteAndDeletePermissions(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	viewerID, editorID, adminID := fixture.user(t), fixture.user(t), fixture.user(t)
	fixture.addMember(t, viewerID, "viewer")
	fixture.addMember(t, editorID, "editor")
	fixture.addMember(t, adminID, "admin")
	taskID := fixture.task(t, viewerID, "Shared child task")
	initialTodoID, initialScheduleID := fixture.addChildren(t, taskID)
	fixture.commit(t)

	uow := sharingTestUOW{pool: pool}
	today := time.Now().UTC().Truncate(time.Minute)
	date := today.Format("2006-01-02")
	pageRequest := usecase.CursorPageRequest{Size: 10, FromDate: date}
	todoList := usecase.NewListTodoItemsUseCase(uow, nil)
	scheduleList := usecase.NewListTaskSchedulesUseCase(uow, nil)
	viewerTodos, err := todoList.Execute(ctx, domain.UserID(viewerID), domain.TaskID(taskID))
	if err != nil || len(viewerTodos) != 1 || viewerTodos[0].ID != initialTodoID {
		t.Fatalf("viewer todo list = %+v, error %v; want initial shared todo", viewerTodos, err)
	}
	viewerTodoPage, err := todoList.ExecutePage(ctx, domain.UserID(viewerID), domain.TaskID(taskID), pageRequest)
	if err != nil || len(viewerTodoPage.Items) != 1 || viewerTodoPage.Items[0].ID != initialTodoID {
		t.Fatalf("viewer todo page = %+v, error %v; want initial shared todo", viewerTodoPage, err)
	}
	viewerSchedules, err := scheduleList.Execute(ctx, domain.UserID(viewerID), domain.TaskID(taskID))
	if err != nil || len(viewerSchedules) != 1 || viewerSchedules[0].ID != initialScheduleID {
		t.Fatalf("viewer schedule list = %+v, error %v; want initial shared schedule", viewerSchedules, err)
	}
	viewerSchedulePage, err := scheduleList.ExecutePage(ctx, domain.UserID(viewerID), domain.TaskID(taskID), pageRequest)
	if err != nil || len(viewerSchedulePage.Items) != 1 || viewerSchedulePage.Items[0].ID != initialScheduleID {
		t.Fatalf("viewer schedule page = %+v, error %v; want initial shared schedule", viewerSchedulePage, err)
	}

	timezones := sharingTimezoneReader{}
	viewerCreate := usecase.NewCreateTodoItemUseCase(uow, timezones, nil)
	if _, err := viewerCreate.Execute(ctx, usecase.CreateTodoItemInput{
		ID: domain.TodoItemID(ulid.Make().String()), UserID: domain.UserID(viewerID), TaskID: domain.TaskID(taskID), Title: "Viewer write must fail",
	}); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("viewer assignee create-todo error = %v, want permission denied", err)
	}
	viewerScheduleCreate := usecase.NewCreateTaskScheduleUseCase(uow, timezones, nil)
	if _, err := viewerScheduleCreate.Execute(ctx, usecase.CreateTaskScheduleInput{
		ID: domain.TaskScheduleID(ulid.Make().String()), UserID: domain.UserID(viewerID), TaskID: domain.TaskID(taskID),
		Title: "Viewer write must fail", StartAt: today.Add(time.Hour), EndAt: today.Add(2 * time.Hour),
	}); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("viewer assignee create-schedule error = %v, want permission denied", err)
	}

	createdTodo, err := usecase.NewCreateTodoItemUseCase(uow, timezones, nil).Execute(ctx, usecase.CreateTodoItemInput{
		ID: domain.TodoItemID(ulid.Make().String()), UserID: domain.UserID(editorID), TaskID: domain.TaskID(taskID),
		Title: "Editor created todo", DueDate: today,
	})
	if err != nil {
		t.Fatalf("create todo as shared editor: %v", err)
	}
	createdSchedule, err := usecase.NewCreateTaskScheduleUseCase(uow, timezones, nil).Execute(ctx, usecase.CreateTaskScheduleInput{
		ID: domain.TaskScheduleID(ulid.Make().String()), UserID: domain.UserID(editorID), TaskID: domain.TaskID(taskID),
		Title: "Editor created schedule", StartAt: today.Add(time.Hour), EndAt: today.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create schedule as shared editor: %v", err)
	}

	updatedTodoTitle, updatedScheduleTitle := "Editor updated todo", "Editor updated schedule"
	updatedTodo, err := usecase.NewUpdateTodoItemUseCase(uow, nil).ExecuteOccurrence(
		ctx, domain.UserID(editorID), domain.TaskID(taskID), domain.TodoItemID(createdTodo.SeriesID), createdTodo.OccurrenceDate, "current",
		usecase.PatchField[string]{Present: true, Value: &updatedTodoTitle}, usecase.PatchField[string]{}, usecase.PatchField[time.Time]{},
	)
	if err != nil || updatedTodo.Title != updatedTodoTitle {
		t.Fatalf("edit shared todo = %+v, error %v; want title %q", updatedTodo, err, updatedTodoTitle)
	}
	updatedSchedule, err := usecase.NewUpdateTaskScheduleUseCase(uow, nil).ExecuteOccurrence(
		ctx, domain.UserID(editorID), domain.TaskID(taskID), domain.TaskScheduleID(createdSchedule.SeriesID), createdSchedule.OccurrenceDate, "current",
		usecase.PatchField[string]{Present: true, Value: &updatedScheduleTitle}, usecase.PatchField[string]{}, usecase.PatchField[string]{},
	)
	if err != nil || updatedSchedule.Title != updatedScheduleTitle {
		t.Fatalf("edit shared schedule = %+v, error %v; want title %q", updatedSchedule, err, updatedScheduleTitle)
	}

	deleteTodo := usecase.NewDeleteTodoItemUseCase(uow, nil)
	if err := deleteTodo.Execute(ctx, domain.UserID(editorID), domain.TaskID(taskID), domain.TodoItemID(createdTodo.ID)); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("editor todo deletion error = %v, want permission denied", err)
	}
	deleteSchedule := usecase.NewDeleteTaskScheduleUseCase(uow, nil)
	if err := deleteSchedule.Execute(ctx, domain.UserID(editorID), domain.TaskID(taskID), domain.TaskScheduleID(createdSchedule.ID)); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("editor schedule deletion error = %v, want permission denied", err)
	}
	if err := deleteTodo.Execute(ctx, domain.UserID(adminID), domain.TaskID(taskID), domain.TodoItemID(createdTodo.ID)); err != nil {
		t.Fatalf("admin todo deletion: %v", err)
	}
	if err := deleteSchedule.Execute(ctx, domain.UserID(adminID), domain.TaskID(taskID), domain.TaskScheduleID(createdSchedule.ID)); err != nil {
		t.Fatalf("admin schedule deletion: %v", err)
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
		INSERT INTO task_tags (id, user_id, name)
		VALUES ($1, $2, 'Owner tag'), ($3, $4, 'Editor tag')
	`, ownerTagID, fixture.ownerID, editorTagID, editorID); err != nil {
		t.Fatalf("insert owner and editor tags: %v", err)
	}
	fixture.commit(t)

	uow := sharingTestUOW{pool: pool}
	addTag := usecase.NewAddTagToTaskUseCase(uow, nil)
	removeTag := usecase.NewRemoveTagFromTaskUseCase(uow, nil)
	actorID, taskDomainID := domain.UserID(editorID), domain.TaskID(taskID)
	if err := addTag.Execute(ctx, actorID, taskDomainID, domain.TaskTagID(ownerTagID)); err != nil {
		t.Fatalf("shared editor attaches project owner's task tag: %v", err)
	}
	if err := addTag.Execute(ctx, actorID, taskDomainID, domain.TaskTagID(editorTagID)); !errors.Is(err, usecase.ErrTaskTagNotFound) {
		t.Fatalf("shared editor attaches own tag to owner's task = %v, want tag not found", err)
	}
	if err := addTag.Execute(ctx, domain.UserID(viewerID), taskDomainID, domain.TaskTagID(ownerTagID)); err == nil {
		t.Fatal("viewer successfully attached a tag to a shared task")
	}
	var assignmentCount int32
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM task_tag_assignments WHERE task_id = $1`, taskID).Scan(&assignmentCount); err != nil {
		t.Fatalf("count shared task tag assignments: %v", err)
	}
	if assignmentCount != 1 {
		t.Fatalf("shared task has %d tag assignments after allowed and rejected adds, want only owner tag", assignmentCount)
	}
	if err := removeTag.Execute(ctx, actorID, taskDomainID, domain.TaskTagID(ownerTagID)); err != nil {
		t.Fatalf("shared editor removes project owner's task tag: %v", err)
	}
	if err := removeTag.Execute(ctx, domain.UserID(viewerID), taskDomainID, domain.TaskTagID(ownerTagID)); err == nil {
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

func (sharingTimezoneReader) GetTimezone(context.Context, domain.UserID) (string, error) {
	return "UTC", nil
}

func TestProjectAndTaskRevisionSnapshotsRecordInitialAndLatestActor(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	actorID := fixture.user(t)
	fixture.addMember(t, actorID, "editor")
	taskID := fixture.task(t, actorID, "Initial title")

	assertInitialRevision := func(table, snapshotTable, id, ownerID string) {
		t.Helper()
		var currentRevision, snapshotRevision int32
		var currentChangedBy, snapshotChangedBy string
		if err := fixture.tx.QueryRow(ctx, `SELECT revision, changed_by FROM `+table+` WHERE id = $1`, id).Scan(&currentRevision, &currentChangedBy); err != nil {
			t.Fatalf("read initial %s: %v", table, err)
		}
		if err := fixture.tx.QueryRow(ctx, `SELECT revision, changed_by FROM `+snapshotTable+` WHERE id = $1 AND revision = 1`, id).Scan(&snapshotRevision, &snapshotChangedBy); err != nil {
			t.Fatalf("read initial %s snapshot: %v", table, err)
		}
		if currentRevision != 1 || snapshotRevision != 1 || currentChangedBy != ownerID || snapshotChangedBy != ownerID {
			t.Fatalf("initial %s revision/current actor = %d/%q snapshot = %d/%q; want revision 1 and owner %q", table, currentRevision, currentChangedBy, snapshotRevision, snapshotChangedBy, ownerID)
		}
	}
	assertInitialRevision("projects", "project_revisions", fixture.projectID, fixture.ownerID)
	assertInitialRevision("tasks", "task_revisions", taskID, fixture.ownerID)

	if _, err := fixture.tx.Exec(ctx, `UPDATE projects SET title = 'Edited project', changed_by = $2 WHERE id = $1`, fixture.projectID, actorID); err != nil {
		t.Fatalf("edit project as member: %v", err)
	}
	if _, err := fixture.tx.Exec(ctx, `UPDATE tasks SET title = 'Edited task', changed_by = $2 WHERE id = $1`, taskID, actorID); err != nil {
		t.Fatalf("edit task as member: %v", err)
	}

	for _, resource := range []struct{ table, snapshotTable, id string }{
		{table: "projects", snapshotTable: "project_revisions", id: fixture.projectID},
		{table: "tasks", snapshotTable: "task_revisions", id: taskID},
	} {
		var revision, snapshots int32
		var title, changedBy, latestTitle, latestChangedBy string
		var updatedAt, latestChangedAt time.Time
		if err := fixture.tx.QueryRow(ctx, `SELECT revision, title, changed_by, updated_at FROM `+resource.table+` WHERE id = $1`, resource.id).Scan(&revision, &title, &changedBy, &updatedAt); err != nil {
			t.Fatalf("read latest %s: %v", resource.table, err)
		}
		if err := fixture.tx.QueryRow(ctx, `
			SELECT count(*)::integer,
			       (array_agg(title ORDER BY revision DESC))[1],
			       (array_agg(changed_by ORDER BY revision DESC))[1],
			       (array_agg(changed_at ORDER BY revision DESC))[1]
			FROM `+resource.snapshotTable+` WHERE id = $1
		`, resource.id).Scan(&snapshots, &latestTitle, &latestChangedBy, &latestChangedAt); err != nil {
			t.Fatalf("read latest %s snapshot: %v", resource.table, err)
		}
		if revision != 2 || snapshots != 2 || title != latestTitle || changedBy != actorID || latestChangedBy != actorID || !updatedAt.Equal(latestChangedAt) {
			t.Errorf("latest %s = revision %d title %q actor %q at %s; snapshots=%d latest snapshot=%q by %q at %s; want revision 2, member actor, matching update time", resource.table, revision, title, changedBy, updatedAt, snapshots, latestTitle, latestChangedBy, latestChangedAt)
		}
	}
}

func TestRemovingMemberReassignsTheirTasksAndRejectsReassignment(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	memberID := fixture.user(t)
	fixture.addMember(t, memberID, "editor")
	taskID := fixture.task(t, memberID, "Assigned before removal")
	fixture.commit(t)

	uow := sharingTestUOW{pool: pool}
	remove := usecase.NewDeleteProjectMemberUseCase(uow, nil)
	if err := remove.Execute(ctx, domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID), domain.UserID(memberID)); err != nil {
		t.Fatalf("remove project member: %v", err)
	}

	tasks := NewTaskRepository(pool)
	updated, err := tasks.GetByUserID(ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID))
	if err != nil {
		t.Fatalf("read reassigned task: %v", err)
	}
	if updated.AssigneeID != fixture.ownerID || updated.Revision != 2 || updated.ChangedBy != fixture.ownerID {
		t.Fatalf("removed member task = assignee %q revision %d changed_by %q; want owner fallback at revision 2", updated.AssigneeID, updated.Revision, updated.ChangedBy)
	}
	var snapshotAssignee, snapshotActor string
	if err := pool.QueryRow(ctx, `
		SELECT assignee_id, changed_by FROM task_revisions WHERE id = $1 AND revision = 2
	`, taskID).Scan(&snapshotAssignee, &snapshotActor); err != nil {
		t.Fatalf("read member-removal task snapshot: %v", err)
	}
	if snapshotAssignee != fixture.ownerID || snapshotActor != fixture.ownerID {
		t.Fatalf("member-removal snapshot assignee=%q actor=%q; want owner and owner", snapshotAssignee, snapshotActor)
	}

	assign := usecase.NewAssignTaskUseCase(uow, nil)
	if _, err := assign.Execute(ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID), domain.UserID(memberID), updated.Revision); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("assign to removed member error = %v, want permission denied", err)
	}
	unchanged, err := tasks.GetByUserID(ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID))
	if err != nil || unchanged.AssigneeID != fixture.ownerID || unchanged.Revision != updated.Revision {
		t.Fatalf("task after rejected re-assignment = %+v, error %v; want owner and unchanged revision", unchanged, err)
	}
}

func TestConcurrentAssignmentAndMemberRemovalCannotLeaveRemovedAssignee(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	memberID := fixture.user(t)
	fixture.addMember(t, memberID, "viewer")
	taskID := fixture.task(t, fixture.ownerID, "Race task")
	fixture.commit(t)

	uow := sharingTestUOW{pool: pool}
	assign := usecase.NewAssignTaskUseCase(uow, nil)
	remove := usecase.NewDeleteProjectMemberUseCase(uow, nil)
	raceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	start := make(chan struct{})
	assignResult := make(chan error, 1)
	removeResult := make(chan error, 1)
	go func() {
		<-start
		_, err := assign.Execute(raceCtx, domain.UserID(fixture.ownerID), domain.TaskID(taskID), domain.UserID(memberID), 1)
		assignResult <- err
	}()
	go func() {
		<-start
		removeResult <- remove.Execute(raceCtx, domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID), domain.UserID(memberID))
	}()
	close(start)
	var assignmentErr, removalErr error
	for received := 0; received < 2; received++ {
		select {
		case assignmentErr = <-assignResult:
		case removalErr = <-removeResult:
		case <-raceCtx.Done():
			t.Fatalf("assignment/removal race exceeded deadline: %v", raceCtx.Err())
		}
	}
	if removalErr != nil {
		t.Fatalf("remove member during assignment: %v", removalErr)
	}
	if assignmentErr != nil && !errors.Is(assignmentErr, usecase.ErrPermissionDenied) {
		t.Fatalf("assignment/removal race assignment error = %v; want success or permission denied after removal", assignmentErr)
	}

	tasks := NewTaskRepository(pool)
	final, err := tasks.GetByUserID(ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID))
	if err != nil {
		t.Fatalf("read task after assignment/removal race: %v", err)
	}
	if final.AssigneeID != fixture.ownerID {
		t.Fatalf("task after assignment/removal race is assigned to %q, want owner %q", final.AssigneeID, fixture.ownerID)
	}
	var membershipExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2)`, fixture.projectID, memberID).Scan(&membershipExists); err != nil {
		t.Fatalf("check removed membership: %v", err)
	}
	if membershipExists {
		t.Fatal("membership still exists after completed removal")
	}
	var snapshotCount int32
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM task_revisions WHERE id = $1`, taskID).Scan(&snapshotCount); err != nil {
		t.Fatalf("count task history after race: %v", err)
	}
	if snapshotCount != final.Revision {
		t.Fatalf("task revision is %d but history has %d snapshots", final.Revision, snapshotCount)
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

	final, err := NewTaskRepository(pool).GetByUserID(ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID))
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
	move := usecase.NewAddTaskToProjectUseCase(uow, NewTaskRepository(pool), nil)
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

func TestDeleteTaskSoftDeletesTodoItemsAndSchedules(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	taskID := fixture.task(t, fixture.ownerID, "Task to delete")
	todoID, scheduleID := fixture.addChildren(t, taskID)
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
	assertDeleted("todo_items", todoID)
	assertDeleted("task_schedules", scheduleID)

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
	if err := usecase.NewDeleteTodoItemUseCase(sharingTestUOW{pool: pool}, nil).Execute(
		ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID), domain.TodoItemID(todoID),
	); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("child mutation under deleted task error = %v, want task not found", err)
	}
}

func TestDeleteProjectSoftDeletesEveryTaskAndChild(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	memberID := fixture.user(t)
	fixture.addMember(t, memberID, "viewer")
	taskIDs := []string{
		fixture.task(t, fixture.ownerID, "Owner task"),
		fixture.task(t, memberID, "Delegated task"),
	}
	todoIDs, scheduleIDs := make([]string, 0, len(taskIDs)), make([]string, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		todoID, scheduleID := fixture.addChildren(t, taskID)
		todoIDs, scheduleIDs = append(todoIDs, todoID), append(scheduleIDs, scheduleID)
	}
	fixture.commit(t)

	remove := usecase.NewDeleteProjectUseCase(sharingTestUOW{pool: pool}, nil)
	if err := remove.Execute(ctx, domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID), 1); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	assertDeleted := func(table, id string) {
		t.Helper()
		var deleted bool
		if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM `+table+` WHERE id = $1`, id).Scan(&deleted); err != nil {
			t.Fatalf("read deleted %s row: %v", table, err)
		}
		if !deleted {
			t.Errorf("%s %s remains active after project deletion", table, id)
		}
	}
	assertDeleted("projects", fixture.projectID)
	for index, taskID := range taskIDs {
		assertDeleted("tasks", taskID)
		assertDeleted("todo_items", todoIDs[index])
		assertDeleted("task_schedules", scheduleIDs[index])
		if err := usecase.NewDeleteTaskScheduleUseCase(sharingTestUOW{pool: pool}, nil).Execute(
			ctx, domain.UserID(fixture.ownerID), domain.TaskID(taskID), domain.TaskScheduleID(scheduleIDs[index]),
		); !errors.Is(err, usecase.ErrTaskNotFound) {
			t.Errorf("child mutation under task deleted with project error = %v, want task not found", err)
		}
		var revision, snapshots int32
		if err := pool.QueryRow(ctx, `SELECT revision FROM tasks WHERE id = $1`, taskID).Scan(&revision); err != nil {
			t.Fatalf("read cascaded task revision: %v", err)
		}
		if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM task_revisions WHERE id = $1`, taskID).Scan(&snapshots); err != nil {
			t.Fatalf("count cascaded task snapshots: %v", err)
		}
		if revision != 2 || snapshots != 2 {
			t.Errorf("cascaded task %s revision/history = %d/%d, want 2/2", taskID, revision, snapshots)
		}
	}
	var projectRevision, projectSnapshots int32
	if err := pool.QueryRow(ctx, `SELECT revision FROM projects WHERE id = $1`, fixture.projectID).Scan(&projectRevision); err != nil {
		t.Fatalf("read deleted project revision: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM project_revisions WHERE id = $1`, fixture.projectID).Scan(&projectSnapshots); err != nil {
		t.Fatalf("count project snapshots: %v", err)
	}
	if projectRevision != 2 || projectSnapshots != 2 {
		t.Fatalf("deleted project revision/history = %d/%d, want 2/2", projectRevision, projectSnapshots)
	}
	if _, err := NewProjectRepository(pool).GetByUserID(ctx, domain.UserID(memberID), domain.ProjectID(fixture.projectID)); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("shared viewer read of deleted project error = %v, want not found", err)
	}
	if err := remove.Execute(ctx, domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID), projectRevision); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("second project mutation error = %v, want deleted project not found", err)
	}
}

func TestRevisionHistoryAccessTracksMembershipAndDeletion(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	viewerID, removedID, adminID := fixture.user(t), fixture.user(t), fixture.user(t)
	fixture.addMember(t, viewerID, "viewer")
	fixture.addMember(t, removedID, "editor")
	fixture.addMember(t, adminID, "admin")
	taskID := fixture.task(t, fixture.ownerID, "History ACL task")
	fixture.commit(t)

	projectHistory := usecase.NewListProjectRevisionsUseCase(NewProjectRepository(pool), nil)
	taskHistory := usecase.NewListTaskRevisionsUseCase(NewTaskRepository(pool), nil)
	pageRequest := usecase.CursorPageRequest{Size: 10}
	assertHistoryCount := func(label string, got int, want int) {
		t.Helper()
		if got != want {
			t.Errorf("%s history count = %d, want %d", label, got, want)
		}
	}
	for _, actorID := range []string{viewerID, removedID, adminID} {
		projectPage, err := projectHistory.Execute(ctx, domain.UserID(actorID), domain.ProjectID(fixture.projectID), pageRequest)
		if err != nil {
			t.Fatalf("read active project history as %s: %v", actorID, err)
		}
		taskPage, err := taskHistory.Execute(ctx, domain.UserID(actorID), domain.TaskID(taskID), pageRequest)
		if err != nil {
			t.Fatalf("read active task history as %s: %v", actorID, err)
		}
		assertHistoryCount("active project for "+actorID, len(projectPage.Items), 1)
		assertHistoryCount("active task for "+actorID, len(taskPage.Items), 1)
	}

	removeMember := usecase.NewDeleteProjectMemberUseCase(sharingTestUOW{pool: pool}, nil)
	if err := removeMember.Execute(ctx, domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID), domain.UserID(removedID)); err != nil {
		t.Fatalf("remove member before deletion: %v", err)
	}
	if _, err := projectHistory.Execute(ctx, domain.UserID(removedID), domain.ProjectID(fixture.projectID), pageRequest); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("removed member project history error = %v, want project not found", err)
	}
	if _, err := taskHistory.Execute(ctx, domain.UserID(removedID), domain.TaskID(taskID), pageRequest); !errors.Is(err, usecase.ErrTaskNotFound) {
		t.Fatalf("removed member task history error = %v, want task not found", err)
	}

	removeProject := usecase.NewDeleteProjectUseCase(sharingTestUOW{pool: pool}, nil)
	if err := removeProject.Execute(ctx, domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID), 1); err != nil {
		t.Fatalf("delete project for history ACL check: %v", err)
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
		projectPage, projectErr := projectHistory.Execute(ctx, domain.UserID(actor.id), domain.ProjectID(fixture.projectID), pageRequest)
		taskPage, taskErr := taskHistory.Execute(ctx, domain.UserID(actor.id), domain.TaskID(taskID), pageRequest)
		if actor.wantHistory {
			if projectErr != nil || taskErr != nil {
				t.Fatalf("read deleted history as %s: project error %v, task error %v", actor.id, projectErr, taskErr)
			}
			assertHistoryCount("deleted project for "+actor.id, len(projectPage.Items), 2)
			assertHistoryCount("deleted task for "+actor.id, len(taskPage.Items), 2)
			continue
		}
		if !errors.Is(taskErr, usecase.ErrTaskNotFound) {
			t.Errorf("deleted task history access as %s error = %v, want task not found", actor.id, taskErr)
		}
		if !errors.Is(projectErr, domain.ErrProjectNotFound) {
			t.Errorf("deleted project history access as %s error = %v, want project not found", actor.id, projectErr)
		}
	}
}

func TestConcurrentProjectTaskCreationAndDeletionCannotLeaveActiveTask(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	editorID := fixture.user(t)
	fixture.addMember(t, editorID, "editor")
	fixture.commit(t)

	uow := sharingTestUOW{pool: pool}
	create := usecase.NewCreateTaskInProjectUseCase(uow, nil)
	remove := usecase.NewDeleteProjectUseCase(uow, nil)
	taskID := ulid.Make().String()
	raceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	start := make(chan struct{})
	type createResult struct {
		task usecaseTaskResult
		err  error
	}
	createResultCh := make(chan createResult, 1)
	deleteResultCh := make(chan error, 1)
	go func() {
		<-start
		task, err := create.Execute(raceCtx, usecase.CreateTaskInProjectInput{
			ID: domain.TaskID(taskID), UserID: domain.UserID(editorID),
			ProjectID: domain.ProjectID(fixture.projectID), Title: "Concurrent shared task",
		})
		createResultCh <- createResult{task: usecaseTaskResult{userID: task.UserID, assigneeID: task.AssigneeID, changedBy: task.ChangedBy}, err: err}
	}()
	go func() {
		<-start
		deleteResultCh <- remove.Execute(raceCtx, domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID), 1)
	}()
	close(start)

	var created createResult
	var deleteErr error
	for received := 0; received < 2; received++ {
		select {
		case created = <-createResultCh:
		case deleteErr = <-deleteResultCh:
		case <-raceCtx.Done():
			t.Fatalf("project create/delete race exceeded deadline: %v", raceCtx.Err())
		}
	}
	if deleteErr != nil {
		t.Fatalf("delete project during task creation: %v", deleteErr)
	}
	if created.err != nil && !errors.Is(created.err, domain.ErrProjectNotFound) {
		t.Fatalf("create/delete race creation error = %v; want success or project not found after delete", created.err)
	}

	var projectDeleted bool
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM projects WHERE id = $1`, fixture.projectID).Scan(&projectDeleted); err != nil {
		t.Fatalf("read project after create/delete race: %v", err)
	}
	if !projectDeleted {
		t.Fatal("project remained active after successful delete")
	}
	var activeTasks int32
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM tasks WHERE project_id = $1 AND deleted_at IS NULL`, fixture.projectID).Scan(&activeTasks); err != nil {
		t.Fatalf("count active tasks after project deletion: %v", err)
	}
	if activeTasks != 0 {
		t.Fatalf("deleted project has %d active tasks", activeTasks)
	}
	var taskExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tasks WHERE id = $1)`, taskID).Scan(&taskExists); err != nil {
		t.Fatalf("check concurrent task row: %v", err)
	}
	if created.err == nil {
		if !taskExists || created.task.userID != fixture.ownerID || created.task.assigneeID != fixture.ownerID || created.task.changedBy != editorID {
			t.Fatalf("successful shared create result exists=%t owner=%q assignee=%q actor=%q; want persisted owner task attributed to editor", taskExists, created.task.userID, created.task.assigneeID, created.task.changedBy)
		}
		var taskDeleted bool
		if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM tasks WHERE id = $1`, taskID).Scan(&taskDeleted); err != nil {
			t.Fatalf("read created task after project deletion: %v", err)
		}
		if !taskDeleted {
			t.Fatal("task created before project deletion remained active")
		}
	} else if taskExists {
		t.Fatal("task row exists although task creation was rejected after project deletion")
	}
}

func TestProjectDeletionCascadesChildCreatedWhileWaitingForTaskLock(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	ctx := t.Context()
	fixture := newSharingFixture(t, pool)
	editorID := fixture.user(t)
	fixture.addMember(t, editorID, "editor")
	taskID := fixture.task(t, fixture.ownerID, "Child deletion race")
	fixture.commit(t)

	functionName := "sharing_pause_insert_" + strings.ToLower(taskID)
	triggerName := "sharing_pause_trigger_" + strings.ToLower(taskID)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(hashtext(NEW.task_id));
			RETURN NEW;
		END;
		$$
	`, functionName)); err != nil {
		t.Fatalf("create child-insert synchronization function: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON todo_items`, triggerName))
		_, _ = pool.Exec(cleanupCtx, fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, functionName))
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TRIGGER %s BEFORE INSERT ON todo_items
		FOR EACH ROW EXECUTE FUNCTION %s()
	`, triggerName, functionName)); err != nil {
		t.Fatalf("create child-insert synchronization trigger: %v", err)
	}

	lockConnection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire advisory-lock connection: %v", err)
	}
	lockHeld := true
	defer func() {
		if lockHeld {
			_, _ = lockConnection.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, taskID)
		}
		lockConnection.Release()
	}()
	if _, err := lockConnection.Exec(ctx, `SELECT pg_advisory_lock(hashtext($1))`, taskID); err != nil {
		t.Fatalf("hold todo insert advisory lock: %v", err)
	}

	childApp, deleteApp := "sharing-child-"+taskID, "sharing-delete-"+taskID
	uowForChild := sharingTestUOW{pool: pool, applicationName: childApp}
	uowForDelete := sharingTestUOW{pool: pool, applicationName: deleteApp}
	create := usecase.NewCreateTodoItemUseCase(uowForChild, sharingTimezoneReader{}, nil)
	remove := usecase.NewDeleteProjectUseCase(uowForDelete, nil)
	todoID := ulid.Make().String()
	raceCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	createResultCh := make(chan error, 1)
	go func() {
		_, err := create.Execute(raceCtx, usecase.CreateTodoItemInput{
			ID: domain.TodoItemID(todoID), UserID: domain.UserID(editorID), TaskID: domain.TaskID(taskID), Title: "Child committed during delete wait",
		})
		createResultCh <- err
	}()
	waitForApplicationLockWait(t, pool, childApp)

	deleteResultCh := make(chan error, 1)
	go func() {
		deleteResultCh <- remove.Execute(raceCtx, domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID), 1)
	}()
	waitForApplicationLockWait(t, pool, deleteApp)

	if _, err := lockConnection.Exec(ctx, `SELECT pg_advisory_unlock(hashtext($1))`, taskID); err != nil {
		t.Fatalf("release todo insert advisory lock: %v", err)
	}
	lockHeld = false

	for _, operation := range []struct {
		name string
		ch   <-chan error
	}{
		{name: "child creation", ch: createResultCh},
		{name: "project deletion", ch: deleteResultCh},
	} {
		select {
		case err := <-operation.ch:
			if err != nil {
				t.Fatalf("%s during cascade race: %v", operation.name, err)
			}
		case <-raceCtx.Done():
			t.Fatalf("%s did not finish before race deadline: %v", operation.name, raceCtx.Err())
		}
	}

	assertDeleted := func(table, id string) {
		t.Helper()
		var deleted bool
		if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM `+table+` WHERE id = $1`, id).Scan(&deleted); err != nil {
			t.Fatalf("read %s after delete/child race: %v", table, err)
		}
		if !deleted {
			t.Errorf("%s %s remains active after project cascade", table, id)
		}
	}
	assertDeleted("tasks", taskID)
	assertDeleted("todo_items", todoID)
}

func waitForApplicationLockWait(t *testing.T, pool *pgxpool.Pool, applicationName string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE application_name = $1 AND wait_event_type = 'Lock'
			)
		`, applicationName).Scan(&waiting); err != nil {
			t.Fatalf("check %s lock wait: %v", applicationName, err)
		}
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("%s did not reach a database lock wait: %v", applicationName, ctx.Err())
		case <-ticker.C:
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

func (fixture *sharingFixture) addChildren(t *testing.T, taskID string) (string, string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Minute)
	date := now.Format("2006-01-02")
	todoID, scheduleID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.tx.Exec(t.Context(), `
		INSERT INTO todo_items (id, task_id, title, position, series_id, occurrence_date, timezone)
		VALUES ($1, $2, 'Todo child', 0, $1, $3::date, 'UTC')
	`, todoID, taskID, date); err != nil {
		t.Fatalf("insert todo child: %v", err)
	}
	if _, err := fixture.tx.Exec(t.Context(), `
		INSERT INTO task_schedules (id, task_id, title, start_at, end_at, series_id, occurrence_date, timezone)
		VALUES ($1, $2, 'Schedule child', $3, $4, $1, $5::date, 'UTC')
	`, scheduleID, taskID, now, now.Add(time.Hour), date); err != nil {
		t.Fatalf("insert schedule child: %v", err)
	}
	return todoID, scheduleID
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
	projects      *ProjectRepository
	tasks         *TaskRepository
	taskTags      *TaskTagRepository
	todoItems     *TodoItemRepository
	taskSchedules *TaskScheduleRepository
}

func (repositories sharingTestRepositories) Projects() usecase.ProjectRepository {
	return repositories.projects
}

func (repositories sharingTestRepositories) Tasks() usecase.TaskRepository {
	return repositories.tasks
}

func (repositories sharingTestRepositories) TaskTags() usecase.TaskTagRepository {
	return repositories.taskTags
}

func (repositories sharingTestRepositories) TodoItems() usecase.TodoItemRepository {
	return repositories.todoItems
}

func (repositories sharingTestRepositories) TaskSchedules() usecase.TaskScheduleRepository {
	return repositories.taskSchedules
}

type sharingTestUOW struct {
	pool            *pgxpool.Pool
	applicationName string
}

func (uow sharingTestUOW) Do(ctx context.Context, run func(context.Context, usecase.Repositories) error) error {
	tx, err := uow.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if uow.applicationName != "" {
		var configured string
		if err := tx.QueryRow(ctx, `SELECT set_config('application_name', $1, true)`, uow.applicationName).Scan(&configured); err != nil {
			return err
		}
	}
	repositories := sharingTestRepositories{
		projects:      NewProjectRepository(tx),
		tasks:         NewTaskRepository(tx),
		taskTags:      NewTaskTagRepository(tx),
		todoItems:     NewTodoItemRepository(tx),
		taskSchedules: NewTaskScheduleRepository(tx),
	}
	if err := run(ctx, repositories); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
