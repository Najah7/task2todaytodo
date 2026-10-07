//go:build integration

package project_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectrepo "github.com/Najah7/task2todaytodo/internal/application/project/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func projectRepositoryIntegrationPool(t *testing.T) *pgxpool.Pool {
	return testdb.Open(t)
}

type projectRepositoryFixture struct {
	tx        pgx.Tx
	pool      *pgxpool.Pool
	ownerID   string
	projectID string
	users     []string
}

func newProjectRepositoryFixture(t *testing.T, pool *pgxpool.Pool) *projectRepositoryFixture {
	t.Helper()
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin Project fixture: %v", err)
	}
	f := &projectRepositoryFixture{tx: tx, pool: pool, ownerID: ulid.Make().String(), projectID: ulid.Make().String()}
	f.users = append(f.users, f.ownerID)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if f.tx != nil {
			_ = f.tx.Rollback(cleanupCtx)
		}
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_revisions WHERE id = $1`, f.projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_members WHERE project_id = $1`, f.projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM projects WHERE id = $1`, f.projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::text[])`, f.users)
	})
	f.insertUser(t, f.ownerID)
	if _, err := tx.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Project integration','low',$2)`, f.projectID, f.ownerID); err != nil {
		t.Fatalf("insert Project: %v", err)
	}
	return f
}

func (f *projectRepositoryFixture) user(t *testing.T) string {
	t.Helper()
	id := ulid.Make().String()
	f.users = append(f.users, id)
	f.insertUser(t, id)
	return id
}

func (f *projectRepositoryFixture) insertUser(t *testing.T, id string) {
	t.Helper()
	if _, err := f.tx.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password) VALUES($1,'Project','Test',$2,'unused')`, id, id+"@project-audit.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func (f *projectRepositoryFixture) addMember(t *testing.T, id, role string) {
	t.Helper()
	if _, err := f.tx.Exec(t.Context(), `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,$3,$4)`, f.projectID, id, role, f.ownerID); err != nil {
		t.Fatalf("add Project member: %v", err)
	}
}

func (f *projectRepositoryFixture) customRole(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	role := "project_audit_" + strings.ToLower(ulid.Make().String())
	if _, err := pool.Exec(t.Context(), `INSERT INTO roles(role_id,name) VALUES($1,$1)`, role); err != nil {
		t.Fatalf("create isolated role: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1 AND role_id=$2`, f.projectID, role)
		_, _ = pool.Exec(context.Background(), `DELETE FROM role_permissions WHERE role_id=$1`, role)
		_, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE role_id=$1`, role)
	})
	return role
}

func (f *projectRepositoryFixture) commit(t *testing.T) {
	t.Helper()
	if err := f.tx.Commit(t.Context()); err != nil {
		t.Fatalf("commit Project fixture: %v", err)
	}
	f.tx = nil
}

func projectPermissionID(t *testing.T, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, resource, action, effect string) int64 {
	t.Helper()
	var id int64
	if err := q.QueryRow(t.Context(), `SELECT permission_id FROM permissions WHERE resource_id=$1 AND action=$2::action AND effect=$3::effect`, resource, action, effect).Scan(&id); err != nil {
		t.Fatalf("find %s.%s %s permission: %v", resource, action, effect, err)
	}
	return id
}

type projectRepositoryTestUOW struct{ pool *pgxpool.Pool }

func (u projectRepositoryTestUOW) Do(ctx context.Context, run func(context.Context, projectusecase.Repository, projectusecase.ProjectChildrenDeleter) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := run(ctx, projectrepo.NewProjectRepository(tx), noProjectChildren{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type noProjectChildren struct{}

func (noProjectChildren) DeleteProjectTasks(context.Context, string, string) error     { return nil }
func (noProjectChildren) DeleteProjectSchedules(context.Context, string, string) error { return nil }
func (noProjectChildren) ReassignTasksAfterProjectMemberRemoval(context.Context, string, string, string) error {
	return nil
}
func (noProjectChildren) ReassignSchedulesAfterProjectMemberRemoval(context.Context, string, string, string) error {
	return nil
}

func TestProjectReadPermissionUsesCurrentRolePermissions(t *testing.T) {
	pool := projectRepositoryIntegrationPool(t)
	f := newProjectRepositoryFixture(t, pool)
	viewer := f.user(t)
	roleID := f.customRole(t, pool)
	f.addMember(t, viewer, roleID)
	f.commit(t)
	ctx := t.Context()
	repo := projectrepo.NewProjectRepository(pool)
	readAllow := projectPermissionID(t, pool, "project", "read", "allow")
	readDeny := projectPermissionID(t, pool, "project", "read", "deny")
	updateDeny := projectPermissionID(t, pool, "project", "update", "deny")
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, roleID, readAllow); err != nil {
		t.Fatal(err)
	}

	if got, err := repo.GetByUserID(ctx, domain.UserID(viewer), domain.ProjectID(f.projectID)); err != nil || got.ID != f.projectID {
		t.Fatalf("viewer project read = %+v, error %v; want shared Project", got, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, roleID, updateDeny); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByUserID(ctx, domain.UserID(viewer), domain.ProjectID(f.projectID)); err != nil {
		t.Fatalf("unrelated deny blocked Project read: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, roleID, readDeny); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByUserID(ctx, domain.UserID(viewer), domain.ProjectID(f.projectID)); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("matching deny read error = %v, want not found", err)
	}
	if _, err := repo.GetByUserID(ctx, domain.UserID(f.ownerID), domain.ProjectID(f.projectID)); err != nil {
		t.Fatalf("owner read blocked by member deny: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id=$2`, roleID, readDeny); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id=$2`, roleID, readAllow); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByUserID(ctx, domain.UserID(viewer), domain.ProjectID(f.projectID)); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("read without grant = %v, want not found", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, roleID, readAllow); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByUserID(ctx, domain.UserID(viewer), domain.ProjectID(f.projectID)); err != nil {
		t.Fatalf("read after grant restored: %v", err)
	}
}

func TestProjectUpdateGrantDoesNotRequireProjectRead(t *testing.T) {
	pool := projectRepositoryIntegrationPool(t)
	f := newProjectRepositoryFixture(t, pool)
	actor := f.user(t)
	roleID := f.customRole(t, pool)
	f.addMember(t, actor, roleID)
	f.commit(t)
	ctx := t.Context()
	updateAllow := projectPermissionID(t, pool, "project", "update", "allow")
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, roleID, updateAllow); err != nil {
		t.Fatal(err)
	}
	repo := projectrepo.NewProjectRepository(pool)
	if _, err := repo.GetByUserID(ctx, domain.UserID(actor), domain.ProjectID(f.projectID)); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("project read without read grant = %v, want hidden Project", err)
	}
	typ, _ := domain.NewProjectType("other")
	priority, _ := domain.NewPriority("low")
	project, err := domain.NewProjectWithDetails(domain.ProjectID(f.projectID), domain.UserID(f.ownerID), typ, "Updated without read", "", "", 0, priority, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := repo.UpdateByUserID(ctx, domain.UserID(actor), project, 1)
	if err != nil {
		t.Fatalf("project.update with no project.read grant: %v", err)
	}
	if updated.Title != "Updated without read" || updated.Revision != 2 || updated.ChangedBy != actor {
		t.Fatalf("updated Project = title %q revision %d actor %q; want edited title/revision 2/editor", updated.Title, updated.Revision, updated.ChangedBy)
	}
}

func TestProjectMemberCreateAndUpdatePermissionsAreDistinctAndDynamic(t *testing.T) {
	pool := projectRepositoryIntegrationPool(t)
	f := newProjectRepositoryFixture(t, pool)
	actor, existing, newMember := f.user(t), f.user(t), f.user(t)
	roleID := f.customRole(t, pool)
	f.addMember(t, actor, roleID)
	f.addMember(t, existing, "viewer")
	f.commit(t)

	createAllow := projectPermissionID(t, pool, "project_member", "create", "allow")
	createDeny := projectPermissionID(t, pool, "project_member", "create", "deny")
	updateAllow := projectPermissionID(t, pool, "project_member", "update", "allow")
	updateDeny := projectPermissionID(t, pool, "project_member", "update", "deny")
	grant := func(id int64) {
		t.Helper()
		if _, err := pool.Exec(t.Context(), `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, roleID, id); err != nil {
			t.Fatal(err)
		}
	}
	upsert := projectusecase.NewUpsertProjectMemberUseCase(projectRepositoryTestUOW{pool}, nil)
	grant(createAllow)
	if err := pool.QueryRow(t.Context(), `SELECT role_id FROM project_members WHERE project_id=$1 AND user_id=$2`, f.projectID, actor).Scan(new(string)); err != nil {
		t.Fatal(err)
	}
	if err := upsert.Execute(t.Context(), domain.UserID(actor), domain.ProjectID(f.projectID), domain.UserID(newMember), "viewer"); err != nil {
		t.Fatalf("create grant failed to add member: %v", err)
	}
	if err := upsert.Execute(t.Context(), domain.UserID(actor), domain.ProjectID(f.projectID), domain.UserID(existing), "admin"); !errors.Is(err, projectusecase.ErrPermissionDenied) {
		t.Fatalf("create grant changed existing role: %v", err)
	}
	grant(updateAllow)
	if err := upsert.Execute(t.Context(), domain.UserID(actor), domain.ProjectID(f.projectID), domain.UserID(existing), "admin"); err != nil {
		t.Fatalf("new update grant did not take effect: %v", err)
	}
	grant(updateDeny)
	if err := upsert.Execute(t.Context(), domain.UserID(actor), domain.ProjectID(f.projectID), domain.UserID(existing), "viewer"); !errors.Is(err, projectusecase.ErrPermissionDenied) {
		t.Fatalf("matching update deny did not override allow: %v", err)
	}
	grant(createDeny)
	if err := upsert.Execute(t.Context(), domain.UserID(actor), domain.ProjectID(f.projectID), domain.UserID(ulid.Make().String()), "viewer"); !errors.Is(err, projectusecase.ErrPermissionDenied) {
		t.Fatalf("matching create deny did not override allow: %v", err)
	}
}

func TestProjectRevisionHistoryTracksMembershipAndDeletion(t *testing.T) {
	pool := projectRepositoryIntegrationPool(t)
	f := newProjectRepositoryFixture(t, pool)
	viewer, removed, admin := f.user(t), f.user(t), f.user(t)
	viewerRole, editorRole := f.customRole(t, pool), f.customRole(t, pool)
	f.addMember(t, viewer, viewerRole)
	f.addMember(t, removed, editorRole)
	f.addMember(t, admin, "admin")
	f.commit(t)
	ctx := t.Context()
	projectRead := projectPermissionID(t, pool, "project", "read", "allow")
	projectUpdate := projectPermissionID(t, pool, "project", "update", "allow")
	for _, role := range []string{viewerRole, editorRole} {
		if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, role, projectRead); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, editorRole, projectUpdate); err != nil {
		t.Fatal(err)
	}
	repo := projectrepo.NewProjectRepository(pool)
	history := projectusecase.NewListProjectRevisionsUseCase(repo, nil)
	page := projectusecase.CursorPageRequest{Size: 10}
	var initialRevision, initialSnapshots int32
	if err := pool.QueryRow(ctx, `SELECT p.revision,(SELECT count(*)::integer FROM project_revisions r WHERE r.id=p.id) FROM projects p WHERE p.id=$1`, f.projectID).Scan(&initialRevision, &initialSnapshots); err != nil {
		t.Fatal(err)
	}
	if initialRevision != 1 || initialSnapshots != 1 {
		t.Fatalf("fixture Project revision/history = %d/%d; want 1/1", initialRevision, initialSnapshots)
	}

	for _, actor := range []string{f.ownerID, viewer, removed, admin} {
		got, err := history.Execute(ctx, domain.UserID(actor), domain.ProjectID(f.projectID), page)
		if err != nil || len(got.Items) != 1 || got.Items[0].ChangedBy != f.ownerID {
			t.Fatalf("initial Project history for %s (owner %s) = %+v, error %v; want owner snapshot", actor, f.ownerID, got, err)
		}
	}
	projectType, _ := domain.NewProjectType("other")
	projectPriority, _ := domain.NewPriority("low")
	project, err := domain.NewProjectWithDetails(domain.ProjectID(f.projectID), domain.UserID(f.ownerID), projectType, "Edited by editor", "", "", 0, projectPriority, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := repo.UpdateByUserID(ctx, domain.UserID(removed), project, 1)
	if err != nil || updated.Revision != 2 || updated.ChangedBy != removed {
		t.Fatalf("editor Project update = %+v, error %v; want revision 2 attributed to editor", updated, err)
	}
	removeMember := projectusecase.NewDeleteProjectMemberUseCase(projectRepositoryTestUOW{pool}, nil)
	if err := removeMember.Execute(ctx, domain.UserID(f.ownerID), domain.ProjectID(f.projectID), domain.UserID(removed)); err != nil {
		t.Fatalf("remove history member: %v", err)
	}
	if _, err := history.Execute(ctx, domain.UserID(removed), domain.ProjectID(f.projectID), page); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("removed member history access = %v, want not found", err)
	}
	deleteProject := projectusecase.NewDeleteProjectUseCase(projectRepositoryTestUOW{pool}, nil)
	if err := deleteProject.Execute(ctx, domain.UserID(f.ownerID), domain.ProjectID(f.projectID), 2); err != nil {
		t.Fatalf("delete Project for history check: %v", err)
	}
	for _, actor := range []struct {
		id   string
		want bool
	}{{f.ownerID, true}, {admin, true}, {viewer, false}, {removed, false}} {
		got, err := history.Execute(ctx, domain.UserID(actor.id), domain.ProjectID(f.projectID), page)
		if actor.want {
			if err != nil || len(got.Items) != 3 || got.Items[1].ChangedBy != removed {
				t.Fatalf("deleted Project history for %s = %+v, error %v; want three snapshots with editor update", actor.id, got, err)
			}
		} else if !errors.Is(err, domain.ErrProjectNotFound) {
			t.Errorf("deleted Project history for %s error = %v, want not found", actor.id, err)
		}
	}
}
