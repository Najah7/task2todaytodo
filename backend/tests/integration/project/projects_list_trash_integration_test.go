//go:build integration

package project_test

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	projectdomain "github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectrepo "github.com/Najah7/task2todaytodo/internal/application/project/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduledomain "github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
)

type projectListIntegrationSeed struct {
	id, title, priority string
	endDate             *string
	createdAt           time.Time
}

func TestProjectListSortKeysetAndSummaryAcrossPages(t *testing.T) {
	pool := projectRepositoryIntegrationPool(t)
	fixture := newProjectRepositoryFixture(t, pool)
	ctx := t.Context()
	if _, err := fixture.tx.Exec(ctx, `UPDATE users SET timezone='UTC' WHERE id=$1`, fixture.ownerID); err != nil {
		t.Fatal(err)
	}
	// Keep one live Project outside selected status to verify tab counts remain
	// independent from the current page and filter.
	if _, err := fixture.tx.Exec(ctx, `UPDATE projects SET status='done' WHERE id=$1`, fixture.projectID); err != nil {
		t.Fatal(err)
	}

	seeds := make([]projectListIntegrationSeed, 0, 45)
	for i := 0; i < 45; i++ {
		id := ulid.Make().String()
		title := "Alpha"
		if i >= 30 {
			title = "Beta"
		}
		priority := "urgent"
		switch {
		case i >= 15 && i < 30:
			priority = "high"
		case i >= 30 && i < 40:
			priority = "medium"
		case i >= 40:
			priority = "low"
		}
		var endDate *string
		switch {
		case i < 25:
			endDate = projectListStringPointer("2026-10-07") // overdue
		case i < 30:
			endDate = projectListStringPointer("2026-10-08") // today
		case i < 35:
			endDate = projectListStringPointer("2026-10-22") // today + 14
		case i < 40:
			endDate = projectListStringPointer("2026-10-23") // outside due soon
		}
		createdAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		if i >= 30 {
			createdAt = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
		}
		seed := projectListIntegrationSeed{id: id, title: title, priority: priority, endDate: endDate, createdAt: createdAt}
		if _, err := fixture.tx.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,end_date,status,created_at,changed_by)
			VALUES($1,$2,'other',$3,$4,$5::date,'open',$6,$2)`, seed.id, fixture.ownerID, seed.title, seed.priority, seed.endDate, seed.createdAt); err != nil {
			t.Fatalf("insert list Project %d: %v", i, err)
		}
		seeds = append(seeds, seed)
	}
	seedIDs := projectListSeedIDs(seeds)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=ANY($1::text[])`, seedIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=ANY($1::text[])`, seedIDs)
	})
	fixture.commit(t)

	repo := projectrepo.NewProjectRepository(pool)
	list := projectusecase.NewListProjectsUseCase(repo, repo, nil)
	asOf := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	for _, sortBy := range []string{"title", "end_date", "created_at"} {
		for _, order := range []string{"asc", "desc"} {
			t.Run(sortBy+"/"+order, func(t *testing.T) {
				ordered := projectListExpectedOrder(seeds, sortBy, order)
				request := projectusecase.ProjectListRequest{Size: 20, Status: "open", SortBy: sortBy, SortOrder: order, AsOf: asOf}
				firstPage, err := list.ExecuteFilteredPage(ctx, projectdomain.UserID(fixture.ownerID), request)
				if err != nil {
					t.Fatalf("first page: %v", err)
				}
				assertProjectListPageIDs(t, firstPage.Items, projectListIDs(ordered[:20]))
				if firstPage.Next == nil || firstPage.Next.ID != ordered[19].id || firstPage.Previous != nil {
					t.Fatalf("first page cursors next=%#v previous=%#v", firstPage.Next, firstPage.Previous)
				}
				assertProjectListSummary(t, firstPage.Summary, 45)

				request.Anchor = firstPage.Next
				secondPage, err := list.ExecuteFilteredPage(ctx, projectdomain.UserID(fixture.ownerID), request)
				if err != nil {
					t.Fatalf("next page: %v", err)
				}
				assertProjectListPageIDs(t, secondPage.Items, projectListIDs(ordered[20:40]))
				if secondPage.Previous == nil || secondPage.Previous.ID != ordered[20].id || secondPage.Next == nil || secondPage.Next.ID != ordered[39].id {
					t.Fatalf("second page cursors previous=%#v next=%#v", secondPage.Previous, secondPage.Next)
				}
				assertProjectListSummary(t, secondPage.Summary, 45)

				request.Anchor = secondPage.Next
				thirdPage, err := list.ExecuteFilteredPage(ctx, projectdomain.UserID(fixture.ownerID), request)
				if err != nil {
					t.Fatalf("third page: %v", err)
				}
				assertProjectListPageIDs(t, thirdPage.Items, projectListIDs(ordered[40:]))
				if thirdPage.Previous == nil || thirdPage.Previous.ID != ordered[40].id || thirdPage.Next != nil {
					t.Fatalf("third page cursors previous=%#v next=%#v", thirdPage.Previous, thirdPage.Next)
				}

				request.Anchor = thirdPage.Previous
				secondFromLastPage, err := list.ExecuteFilteredPage(ctx, projectdomain.UserID(fixture.ownerID), request)
				if err != nil {
					t.Fatalf("previous from third page: %v", err)
				}
				assertProjectListPageIDs(t, secondFromLastPage.Items, projectListIDs(ordered[20:40]))

				request.Anchor = secondPage.Previous
				previousPage, err := list.ExecuteFilteredPage(ctx, projectdomain.UserID(fixture.ownerID), request)
				if err != nil {
					t.Fatalf("previous page: %v", err)
				}
				assertProjectListPageIDs(t, previousPage.Items, projectListIDs(ordered[:20]))
				if previousPage.Previous != nil || previousPage.Next == nil || previousPage.Next.ID != ordered[19].id {
					t.Fatalf("backward page cursors previous=%#v next=%#v", previousPage.Previous, previousPage.Next)
				}

				// Remove the cursor row, then continue using its tuple. The cursor
				// must not depend on finding its Project in the filtered relation.
				var revision int32
				if err := pool.QueryRow(ctx, `SELECT revision FROM projects WHERE id=$1`, firstPage.Next.ID).Scan(&revision); err != nil {
					t.Fatal(err)
				}
				deleteProject := projectusecase.NewDeleteProjectUseCase(projectRepositoryTestUOW{pool}, nil)
				if err := deleteProject.Execute(ctx, projectdomain.UserID(fixture.ownerID), projectdomain.ProjectID(firstPage.Next.ID), revision); err != nil {
					t.Fatalf("delete cursor Project %s: %v", firstPage.Next.ID, err)
				}
				request.Anchor = firstPage.Next
				continued, err := list.ExecuteFilteredPage(ctx, projectdomain.UserID(fixture.ownerID), request)
				if err != nil {
					t.Fatalf("continue after cursor row deletion: %v", err)
				}
				assertProjectListPageIDs(t, continued.Items, projectListIDs(ordered[20:40]))
				assertProjectListSummaryExcludingSeed(t, continued.Summary, seeds, firstPage.Next.ID)
				if err := pool.QueryRow(ctx, `SELECT revision FROM projects WHERE id=$1`, firstPage.Next.ID).Scan(&revision); err != nil {
					t.Fatal(err)
				}
				if _, err := projectusecase.NewRestoreProjectUseCase(projectRepositoryTestUOW{pool}, repo, nil).Execute(ctx, projectdomain.UserID(fixture.ownerID), projectdomain.ProjectID(firstPage.Next.ID), revision); err != nil {
					t.Fatalf("restore cursor Project after continuation: %v", err)
				}
			})
		}
	}
}

func TestProjectListTrashRequiresDeletePermissionAndReturnsWholeTabCounts(t *testing.T) {
	pool := projectRepositoryIntegrationPool(t)
	fixture := newProjectRepositoryFixture(t, pool)
	ctx := t.Context()
	deleteActor, readActor := fixture.user(t), fixture.user(t)
	deleteRole := fixture.customRole(t, pool)
	readRole := fixture.customRole(t, pool)
	readPermission := projectPermissionID(t, pool, "project", "read", "allow")
	deletePermission := projectPermissionID(t, pool, "project", "delete", "allow")
	for _, grant := range []struct {
		role string
		id   int64
	}{{deleteRole, deletePermission}, {readRole, readPermission}} {
		if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, grant.role, grant.id); err != nil {
			t.Fatal(err)
		}
	}
	trashedProjectID := ulid.Make().String()
	if _, err := fixture.tx.Exec(ctx, `UPDATE users SET timezone='UTC' WHERE id=ANY($1::text[])`, []string{fixture.ownerID, deleteActor, readActor}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ projectID, title string }{
		{fixture.projectID, "Live shared Project"},
		{trashedProjectID, "Trashed shared Project"},
	} {
		if row.projectID != fixture.projectID {
			if _, err := fixture.tx.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other',$3,'low','open',$2)`, row.projectID, fixture.ownerID, row.title); err != nil {
				t.Fatal(err)
			}
		}
		for _, member := range []struct{ userID, role string }{{deleteActor, deleteRole}, {readActor, readRole}} {
			if _, err := fixture.tx.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,$3,$4)`, row.projectID, member.userID, member.role, fixture.ownerID); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, trashedProjectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, trashedProjectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, trashedProjectID)
	})
	fixture.commit(t)

	deleteProject := projectusecase.NewDeleteProjectUseCase(projectRepositoryTestUOW{pool}, nil)
	if err := deleteProject.Execute(ctx, projectdomain.UserID(fixture.ownerID), projectdomain.ProjectID(trashedProjectID), 1); err != nil {
		t.Fatalf("move Project to trash: %v", err)
	}
	repo := projectrepo.NewProjectRepository(pool)
	list := projectusecase.NewListProjectsUseCase(repo, repo, nil)
	asOf := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	request := projectusecase.ProjectListRequest{Size: 20, Status: "open", SortBy: "title", SortOrder: "asc", AsOf: asOf}
	deleteActorLive, err := list.ExecuteFilteredPage(ctx, projectdomain.UserID(deleteActor), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleteActorLive.Items) != 0 || deleteActorLive.Summary.TotalCount != 0 || deleteActorLive.Summary.TrashCount != 1 {
		t.Fatalf("delete-only actor live page rows=%v summary=%+v; want no live read and one accessible trash item", projectListActualIDs(deleteActorLive.Items), deleteActorLive.Summary)
	}
	request.Status, request.Trash = "", true
	deleteActorTrash, err := list.ExecuteFilteredPage(ctx, projectdomain.UserID(deleteActor), request)
	if err != nil {
		t.Fatal(err)
	}
	if got := projectListActualIDs(deleteActorTrash.Items); !reflect.DeepEqual(got, []string{trashedProjectID}) {
		t.Fatalf("delete-only actor trash IDs=%v, want [%s]", got, trashedProjectID)
	}
	if deleteActorTrash.Summary.TotalCount != 1 || deleteActorTrash.Summary.TrashCount != 1 || !deleteActorTrash.Items[0].CanDelete || deleteActorTrash.Items[0].CanUpdate {
		t.Fatalf("delete-only trash summary/permissions = %+v / update=%t delete=%t", deleteActorTrash.Summary, deleteActorTrash.Items[0].CanUpdate, deleteActorTrash.Items[0].CanDelete)
	}

	request.Trash = false
	readActorLive, err := list.ExecuteFilteredPage(ctx, projectdomain.UserID(readActor), request)
	if err != nil {
		t.Fatal(err)
	}
	if got := projectListActualIDs(readActorLive.Items); !reflect.DeepEqual(got, []string{fixture.projectID}) {
		t.Fatalf("read-only actor live IDs=%v, want [%s]", got, fixture.projectID)
	}
	if readActorLive.Summary.TotalCount != 1 || readActorLive.Summary.OpenCount != 1 || readActorLive.Summary.TrashCount != 0 || readActorLive.Items[0].CanUpdate || readActorLive.Items[0].CanDelete {
		t.Fatalf("read-only live summary/permissions = %+v / update=%t delete=%t", readActorLive.Summary, readActorLive.Items[0].CanUpdate, readActorLive.Items[0].CanDelete)
	}
	request.Status, request.Trash = "", true
	readActorTrash, err := list.ExecuteFilteredPage(ctx, projectdomain.UserID(readActor), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(readActorTrash.Items) != 0 || readActorTrash.Summary.TotalCount != 0 || readActorTrash.Summary.TrashCount != 0 {
		t.Fatalf("read-only actor trash rows=%v summary=%+v; want no trash visibility", projectListActualIDs(readActorTrash.Items), readActorTrash.Summary)
	}
}

func TestProjectTrashAndRestoreChangeOnlyParentDeletionState(t *testing.T) {
	pool := projectRepositoryIntegrationPool(t)
	fixture := newProjectRepositoryFixture(t, pool)
	ctx := t.Context()
	taskIDs := []string{ulid.Make().String(), ulid.Make().String()}
	todoIDs := []string{ulid.Make().String(), ulid.Make().String()}
	scheduleIDs := []string{ulid.Make().String(), ulid.Make().String()}
	start := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	for i, taskID := range taskIDs {
		if _, err := fixture.tx.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,status,changed_by) VALUES($1,$2,$3,$2,$4,'open',$2)`, taskID, fixture.ownerID, fixture.projectID, []string{"Individually deleted Task", "Live Task"}[i]); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.tx.Exec(ctx, `INSERT INTO todo_items(id,task_id,title,position,series_id,occurrence_date,timezone) VALUES($1,$2,$3,0,$1,'2026-10-08','UTC')`, todoIDs[i], taskID, "Todo item"); err != nil {
			t.Fatal(err)
		}
	}
	for i, scheduleID := range scheduleIDs {
		if _, err := fixture.tx.Exec(ctx, `INSERT INTO schedules(id,user_id,project_id,assignee_id,title,start_at,end_at,series_id,occurrence_date,timezone,repeat_state,changed_by)
			VALUES($1,$2,$3,$2,$4,$5,$6,$1,'2026-10-08','UTC','one_off',$2)`, scheduleID, fixture.ownerID, fixture.projectID, []string{"Individually deleted Schedule", "Live Schedule"}[i], start.Add(time.Duration(i)*2*time.Hour), start.Add(time.Duration(i)*2*time.Hour+time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM todo_items WHERE id=ANY($1::text[])`, todoIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=ANY($1::text[])`, taskIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE id=ANY($1::text[])`, scheduleIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE id=ANY($1::text[])`, scheduleIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=ANY($1::text[])`, taskIDs)
	})
	fixture.commit(t)

	store := application.NewStore(pool)
	taskDelete := taskusecase.NewDeleteTaskUseCase(application.NewTaskUOW(pool, store.Task, store.Project), nil)
	if err := taskDelete.Execute(ctx, taskdomain.UserID(fixture.ownerID), taskdomain.TaskID(taskIDs[0]), 1); err != nil {
		t.Fatalf("individually delete Task: %v", err)
	}
	scheduleDelete := scheduleusecase.NewDeleteScheduleUseCase(application.NewScheduleUOW(pool, store.Schedule, store.Project), nil)
	if err := scheduleDelete.Execute(ctx, scheduledomain.UserID(fixture.ownerID), scheduledomain.ScheduleID(scheduleIDs[0])); err != nil {
		t.Fatalf("individually delete Schedule: %v", err)
	}

	beforeTasks := projectListReadChildStates(t, pool, "tasks", taskIDs)
	beforeSchedules := projectListReadChildStates(t, pool, "schedules", scheduleIDs)
	var beforeTodoDeleted []bool
	for _, todoID := range todoIDs {
		var deleted bool
		if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM todo_items WHERE id=$1`, todoID).Scan(&deleted); err != nil {
			t.Fatal(err)
		}
		beforeTodoDeleted = append(beforeTodoDeleted, deleted)
	}
	if !beforeTasks[0].deleted || beforeTasks[1].deleted || !beforeSchedules[0].deleted || beforeSchedules[1].deleted || !beforeTodoDeleted[0] || beforeTodoDeleted[1] {
		t.Fatalf("child fixture states not established: Tasks=%+v Schedules=%+v TodoDeleted=%v", beforeTasks, beforeSchedules, beforeTodoDeleted)
	}

	projectUOW := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	deleteProject := projectusecase.NewDeleteProjectUseCase(projectUOW, nil)
	if err := deleteProject.Execute(ctx, projectdomain.UserID(fixture.ownerID), projectdomain.ProjectID(fixture.projectID), 1); err != nil {
		t.Fatalf("trash Project: %v", err)
	}
	var deleted bool
	var status string
	var projectRevision int32
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,status,revision FROM projects WHERE id=$1`, fixture.projectID).Scan(&deleted, &status, &projectRevision); err != nil {
		t.Fatal(err)
	}
	if !deleted || status != "open" || projectRevision != 2 {
		t.Fatalf("trashed Project state deleted=%t status=%q revision=%d; want true/open/2", deleted, status, projectRevision)
	}
	assertProjectListChildStates(t, pool, "after Project trash", "tasks", taskIDs, beforeTasks)
	assertProjectListChildStates(t, pool, "after Project trash", "schedules", scheduleIDs, beforeSchedules)
	assertProjectListTodoDeleted(t, pool, todoIDs, beforeTodoDeleted)

	restored, err := projectusecase.NewRestoreProjectUseCase(projectUOW, store.Project.Projects, nil).Execute(ctx, projectdomain.UserID(fixture.ownerID), projectdomain.ProjectID(fixture.projectID), 2)
	if err != nil {
		t.Fatalf("restore Project: %v", err)
	}
	if restored.Status != "open" || restored.Revision != 3 {
		t.Fatalf("restored Project status=%q revision=%d; want open/3", restored.Status, restored.Revision)
	}
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,status,revision FROM projects WHERE id=$1`, fixture.projectID).Scan(&deleted, &status, &projectRevision); err != nil {
		t.Fatal(err)
	}
	if deleted || status != "open" || projectRevision != 3 {
		t.Fatalf("restored Project state deleted=%t status=%q revision=%d; want false/open/3", deleted, status, projectRevision)
	}
	assertProjectListChildStates(t, pool, "after Project restore", "tasks", taskIDs, beforeTasks)
	assertProjectListChildStates(t, pool, "after Project restore", "schedules", scheduleIDs, beforeSchedules)
	assertProjectListTodoDeleted(t, pool, todoIDs, beforeTodoDeleted)
}

type projectListChildState struct {
	deleted   bool
	revision  int32
	changedBy string
}

func projectListReadChildStates(t *testing.T, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, table string, ids []string) []projectListChildState {
	t.Helper()
	states := make([]projectListChildState, 0, len(ids))
	for _, id := range ids {
		var state projectListChildState
		query := "SELECT deleted_at IS NOT NULL,revision,changed_by FROM " + table + " WHERE id=$1"
		if err := q.QueryRow(t.Context(), query, id).Scan(&state.deleted, &state.revision, &state.changedBy); err != nil {
			t.Fatal(err)
		}
		states = append(states, state)
	}
	return states
}

func assertProjectListChildStates(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, label, table string, ids []string, want []projectListChildState) {
	t.Helper()
	got := projectListReadChildStates(t, pool, table, ids)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s %s states=%+v, want preserved %+v", label, table, got, want)
	}
}

func assertProjectListTodoDeleted(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, ids []string, want []bool) {
	t.Helper()
	got := make([]bool, 0, len(ids))
	for _, id := range ids {
		var deleted bool
		if err := pool.QueryRow(t.Context(), `SELECT deleted_at IS NOT NULL FROM todo_items WHERE id=$1`, id).Scan(&deleted); err != nil {
			t.Fatal(err)
		}
		got = append(got, deleted)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TodoItem deleted states=%v, want preserved %v", got, want)
	}
}

func projectListExpectedOrder(seeds []projectListIntegrationSeed, sortBy, order string) []projectListIntegrationSeed {
	ordered := append([]projectListIntegrationSeed(nil), seeds...)
	weights := map[string]int{"urgent": 100, "high": 50, "medium": 25, "low": 10, "someday": 0}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		comparison := 0
		switch sortBy {
		case "title":
			comparison = strings.Compare(strings.ToLower(a.title), strings.ToLower(b.title))
		case "end_date":
			if (a.endDate == nil) != (b.endDate == nil) {
				return b.endDate == nil
			}
			if a.endDate != nil && b.endDate != nil {
				comparison = strings.Compare(*a.endDate, *b.endDate)
			}
		case "created_at":
			if a.createdAt.Before(b.createdAt) {
				comparison = -1
			} else if a.createdAt.After(b.createdAt) {
				comparison = 1
			}
		}
		if comparison != 0 {
			if order == "desc" {
				return comparison > 0
			}
			return comparison < 0
		}
		if sortBy != "created_at" && weights[a.priority] != weights[b.priority] {
			return weights[a.priority] > weights[b.priority]
		}
		if order == "desc" && sortBy == "created_at" {
			return a.id > b.id
		}
		return a.id < b.id
	})
	return ordered
}

func assertProjectListSummary(t *testing.T, summary dao.ProjectListSummary, total int64) {
	t.Helper()
	if summary.TotalCount != total || summary.OpenCount != 45 || summary.DoneCount != 1 || summary.DueSoonCount != 10 || summary.OverdueCount != 25 || summary.Today != "2026-10-08" || summary.Timezone != "UTC" {
		t.Errorf("project summary=%+v; want total=%d open=45 done=1 dueSoon=10 overdue=25 today=2026-10-08 UTC", summary, total)
	}
}

func assertProjectListSummaryExcludingSeed(t *testing.T, summary dao.ProjectListSummary, seeds []projectListIntegrationSeed, excludedID string) {
	t.Helper()
	var dueSoon, overdue int64
	for _, seed := range seeds {
		if seed.id == excludedID || seed.endDate == nil {
			continue
		}
		if *seed.endDate < "2026-10-08" {
			overdue++
		} else if *seed.endDate <= "2026-10-22" {
			dueSoon++
		}
	}
	if summary.TotalCount != 44 || summary.OpenCount != 44 || summary.DoneCount != 1 || summary.DueSoonCount != dueSoon || summary.OverdueCount != overdue || summary.TrashCount != 1 || summary.Today != "2026-10-08" || summary.Timezone != "UTC" {
		t.Errorf("summary after cursor deletion=%+v; want total/open=44 done=1 dueSoon=%d overdue=%d trash=1 today=2026-10-08 UTC", summary, dueSoon, overdue)
	}
}

func assertProjectListPageIDs(t *testing.T, projects []dao.Project, want []string) {
	t.Helper()
	got := projectListActualIDs(projects)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Project IDs=%v; want %v", got, want)
	}
}

func projectListActualIDs(projects []dao.Project) []string {
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
	}
	return ids
}

func projectListSeedIDs(seeds []projectListIntegrationSeed) []string {
	ids := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		ids = append(ids, seed.id)
	}
	return ids
}

func projectListIDs(seeds []projectListIntegrationSeed) []string {
	ids := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		ids = append(ids, seed.id)
	}
	return ids
}

func projectListStringPointer(value string) *string { return &value }
