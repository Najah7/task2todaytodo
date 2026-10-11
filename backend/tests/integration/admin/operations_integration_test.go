//go:build integration

package admin_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	admindao "github.com/Najah7/task2todaytodo/internal/application/admin/dao"
	adminrepo "github.com/Najah7/task2todaytodo/internal/application/admin/repository"
	adminusecase "github.com/Najah7/task2todaytodo/internal/application/admin/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	"github.com/Najah7/task2todaytodo/internal/settings"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func TestConcurrentBackfillAcceptReturnsOneActiveOperation(t *testing.T) {
	pool := testdb.Open(t)
	userID := seedUser(t, pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM admin_operations WHERE requested_by=$1`, userID)
	})
	repo := adminrepo.NewOperationRepository(pool)
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			op, created, _, err := repo.CreateOrGetActive(t.Context(), newOperation(userID), ulid.Make().String(), time.Minute)
			results <- result{operation: op, created: created, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var id string
	createdCount := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("CreateOrGetActive() error = %v", result.err)
		}
		if id == "" {
			id = result.operation.ID
		}
		if result.operation.ID != id {
			t.Fatalf("active operation ids differ: %s and %s", id, result.operation.ID)
		}
		if result.created {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("created operations = %d, want 1", createdCount)
	}
}

func TestApplicationCompositionAndCleanup(t *testing.T) {
	testPool := testdb.Open(t)
	conn := testPool.Config().ConnConfig
	app, closeApp, err := application.New(settings.DatabaseSettings{
		User: conn.User, Password: conn.Password, Host: conn.Host,
		Port: strconv.Itoa(int(conn.Port)), Name: conn.Database, SSLMode: "disable",
	}, adapter.NewIDIssuer(), telemetry.Nop())
	if err != nil {
		t.Fatalf("application.New(): %v", err)
	}
	testdb.RegisterShutdown(t, closeApp)

	if app.UseCase.Admin.GetOperation == nil || app.UseCase.Admin.ListOperations == nil || app.UseCase.Admin.RunBackfill == nil {
		t.Fatalf("admin use cases are not fully wired: %+v", app.UseCase.Admin)
	}
	if app.Store.Admin.Operations == nil {
		t.Fatal("admin operation repository is not wired")
	}
	if exists, err := app.Store.Auth.Users.TimezoneExists(t.Context(), "UTC"); err != nil || !exists {
		t.Fatalf("TimezoneExists(UTC) before shutdown = %v, %v; want true, nil", exists, err)
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = closeApp(closeCtx)
	cancel()
	if err != nil {
		t.Fatalf("close application: %v", err)
	}
	if _, err := app.Store.Auth.Users.TimezoneExists(context.Background(), "UTC"); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("TimezoneExists(UTC) after shutdown error = %v; want closed pool error", err)
	}
	// Shutdown is safe to call again from the registered test cleanup.
	if err := closeApp(context.Background()); err != nil {
		t.Fatalf("close application again: %v", err)
	}
}

func TestExpiredLeaseProjectsFailureAndNextRunPersistsIt(t *testing.T) {
	pool := testdb.Open(t)
	userID := seedUser(t, pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM admin_operations WHERE requested_by=$1`, userID)
	})
	repo := adminrepo.NewOperationRepository(pool)
	first, created, token, err := repo.CreateOrGetActive(t.Context(), newOperation(userID), ulid.Make().String(), time.Minute)
	if err != nil || !created {
		t.Fatalf("first CreateOrGetActive() = created %v, err %v", created, err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE admin_operations SET lease_expires_at=now()-interval '1 second' WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Fail(t.Context(), first.ID, token, "interrupted", "safe interrupted detail"); !errors.Is(err, adminusecase.ErrLeaseLost) {
		t.Fatalf("Fail after lease expiration = %v, want ErrLeaseLost", err)
	}
	projected, err := repo.Get(t.Context(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if projected.State != "failed" || projected.ErrorCode != "interrupted" {
		t.Fatalf("expired projection = %+v", projected)
	}
	second, created, _, err := repo.CreateOrGetActive(t.Context(), newOperation(userID), ulid.Make().String(), time.Minute)
	if err != nil || !created || second.ID == first.ID {
		t.Fatalf("next run = %+v, created %v, err %v", second, created, err)
	}
	var state, code, message string
	if err := pool.QueryRow(t.Context(), `SELECT state,error_code,error_message FROM admin_operations WHERE id=$1`, first.ID).Scan(&state, &code, &message); err != nil {
		t.Fatal(err)
	}
	if state != "failed" || code != "interrupted" || message == "" {
		t.Fatalf("persisted stale operation = %s %s %q", state, code, message)
	}
}

func TestOperationReadMapsNullableFieldsForActiveAndTerminalRows(t *testing.T) {
	pool := testdb.Open(t)
	userID := seedUser(t, pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM admin_operations WHERE requested_by=$1`, userID)
	})
	repo := adminrepo.NewOperationRepository(pool)

	active, created, token, err := repo.CreateOrGetActive(t.Context(), newOperation(userID), ulid.Make().String(), time.Minute)
	if err != nil || !created {
		t.Fatalf("create active operation: created=%v err=%v", created, err)
	}
	readActive, err := repo.Get(t.Context(), active.ID)
	if err != nil || readActive.State != "running" || readActive.ErrorCode != "" || readActive.ErrorMessage != "" {
		t.Fatalf("read active operation = %+v, err=%v", readActive, err)
	}
	if err := repo.Succeed(t.Context(), active.ID, token, 3, 7); err != nil {
		t.Fatalf("succeed operation: %v", err)
	}

	failed, created, token, err := repo.CreateOrGetActive(t.Context(), newOperation(userID), ulid.Make().String(), time.Minute)
	if err != nil || !created {
		t.Fatalf("create second operation: created=%v err=%v", created, err)
	}
	if err := repo.Fail(t.Context(), failed.ID, token, "backfill_failed", "safe failure detail"); err != nil {
		t.Fatalf("fail operation: %v", err)
	}
	readFailed, err := repo.Get(t.Context(), failed.ID)
	if err != nil || readFailed.State != "failed" || readFailed.ErrorCode != "backfill_failed" || readFailed.ErrorMessage != "safe failure detail" {
		t.Fatalf("read failed operation = %+v, err=%v", readFailed, err)
	}

	items, err := repo.List(t.Context(), "", 10)
	if err != nil || len(items) != 2 {
		t.Fatalf("list terminal operations = %+v, err=%v", items, err)
	}
	for _, item := range items {
		if item.ID == active.ID && (item.State != "succeeded" || item.Response["generated_schedules"] != 7) {
			t.Errorf("listed success = %+v", item)
		}
		if item.ID == failed.ID && (item.State != "failed" || item.ErrorCode != "backfill_failed") {
			t.Errorf("listed failure = %+v", item)
		}
	}
}

func TestAdminRepositoryChecksSystemCapabilities(t *testing.T) {
	pool := testdb.Open(t)
	ownerID, memberID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	seedUserWithID(t, pool, ownerID)
	seedUserWithID(t, pool, memberID)
	if _, err := pool.Exec(t.Context(), `INSERT INTO projects(id,user_id,title,changed_by) VALUES($1,$2,'Project admin scope',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'admin',$3)`, projectID, memberID, ownerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(ctx, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, memberID})
	})
	operations := adminrepo.NewOperationRepository(pool)
	allowed, err := operations.HasPermission(t.Context(), memberID, shared.AdminJobRun())
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("Project administrator received global admin_job:run permission")
	}
	sysadminID := seedUser(t, pool)
	if _, err := pool.Exec(t.Context(), `INSERT INTO system_user_roles(user_id,role_id) VALUES($1,'sysadmin')`, sysadminID); err != nil {
		t.Fatal(err)
	}
	for _, capability := range []shared.Capability{shared.AdminJobRun(), shared.AdminOperationRead()} {
		allowed, err := operations.HasPermission(t.Context(), sysadminID, capability)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed {
			t.Fatalf("sysadmin lacks capability %+v", capability)
		}
	}
	if _, err := pool.Exec(t.Context(), `UPDATE project_members SET role_id='sysadmin' WHERE project_id=$1 AND user_id=$2`, projectID, memberID); err == nil {
		t.Fatal("Project membership accepted sysadmin role")
	}
}

func TestAdminUseCasesEnforcePermissionsBeforeOperationAccess(t *testing.T) {
	pool := testdb.Open(t)
	ordinaryID, projectAdminID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	seedUserWithID(t, pool, ordinaryID)
	seedUserWithID(t, pool, projectAdminID)
	if _, err := pool.Exec(t.Context(), `INSERT INTO projects(id,user_id,title,changed_by) VALUES($1,$2,'Admin use case scope',$2)`, projectID, ordinaryID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'admin',$3)`, projectID, projectAdminID, ordinaryID); err != nil {
		t.Fatal(err)
	}
	sysadminID := seedUser(t, pool)
	if _, err := pool.Exec(t.Context(), `INSERT INTO system_user_roles(user_id,role_id) VALUES($1,'sysadmin')`, sysadminID); err != nil {
		t.Fatal(err)
	}
	projectCleanup(t, pool, projectID, ordinaryID, projectAdminID)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM admin_operations WHERE requested_by=$1`, sysadminID)
	})

	repo := adminrepo.NewOperationRepository(pool)
	runner := &integrationRunnerFake{}
	uses := adminusecase.UseCases{
		RunBackfill:    adminusecase.NewRunBackfillSchedulesUseCase(repo, runner, adapter.NewIDIssuer(), integrationClockFake{}, time.Minute),
		ListOperations: adminusecase.NewListOperationsUseCase(repo),
		GetOperation:   adminusecase.NewGetOperationUseCase(repo),
	}
	for _, actorID := range []string{ordinaryID, projectAdminID} {
		if _, err := uses.RunBackfill.Execute(t.Context(), actorID); !errors.Is(err, adminusecase.ErrPermissionDenied) {
			t.Errorf("RunBackfill(%s) error = %v, want permission denied", actorID, err)
		}
		if _, err := uses.ListOperations.Execute(t.Context(), actorID, "", 10); !errors.Is(err, adminusecase.ErrPermissionDenied) {
			t.Errorf("ListOperations(%s) error = %v, want permission denied", actorID, err)
		}
		if _, err := uses.GetOperation.Execute(t.Context(), actorID, ulid.Make().String()); !errors.Is(err, adminusecase.ErrPermissionDenied) {
			t.Errorf("GetOperation(%s) error = %v, want permission denied", actorID, err)
		}
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls after denied runs = %d", runner.calls)
	}

	operation, err := uses.RunBackfill.Execute(t.Context(), sysadminID)
	if err != nil {
		t.Fatalf("sysadmin RunBackfill() error = %v", err)
	}
	if runner.calls != 1 || runner.actorID != sysadminID {
		t.Fatalf("runner calls=%d actor=%q", runner.calls, runner.actorID)
	}
	if _, err := uses.ListOperations.Execute(t.Context(), sysadminID, "", 10); err != nil {
		t.Fatalf("sysadmin ListOperations() error = %v", err)
	}
	if _, err := uses.GetOperation.Execute(t.Context(), sysadminID, operation.ID); err != nil {
		t.Fatalf("sysadmin GetOperation() error = %v", err)
	}
	if err := repo.Fail(t.Context(), operation.ID, runner.leaseToken, "test_complete", "integration cleanup"); err != nil {
		t.Fatalf("terminalize sysadmin operation: %v", err)
	}
}

func projectCleanup(t *testing.T, pool *pgxpool.Pool, projectID, ownerID, memberID string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(ctx, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, memberID})
	})
}

type integrationRunnerFake struct {
	calls      int
	actorID    string
	leaseToken string
}

func (r *integrationRunnerFake) StartOperation(_, leaseToken, actorID string, _ time.Time) {
	r.calls++
	r.actorID = actorID
	r.leaseToken = leaseToken
}

type integrationClockFake struct{}

func (integrationClockFake) Now() time.Time { return time.Now().UTC() }

type result struct {
	operation admindao.Operation
	created   bool
	err       error
}

func newOperation(userID string) admindao.Operation {
	now := time.Now().UTC()
	return admindao.Operation{ID: ulid.Make().String(), JobName: "backfill-schedules", State: "running", RequestedBy: userID, Cutoff: now, CreatedAt: now, StartedAt: now}
}

func seedUser(t *testing.T, pool *pgxpool.Pool) string {
	id := ulid.Make().String()
	seedUserWithID(t, pool, id)
	return id
}

func seedUserWithID(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password) VALUES($1,'Admin','Test',$2,'unused')`, id, id+"@admin.test"); err != nil {
		t.Fatalf("insert admin test user: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	})
}
