//go:build integration

package allocation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	restapi "github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/Najah7/task2todaytodo/internal/settings"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type allocationHTTP struct {
	pool       *pgxpool.Pool
	router     http.Handler
	closeApp   func(context.Context) error
	users      []string
	projects   []string
	cleanupSet bool
}

func newAllocationHTTP(t *testing.T) *allocationHTTP {
	t.Helper()
	pool := testdb.Open(t)
	connection := pool.Config().ConnConfig
	issuer := adapter.NewIDIssuer()
	app, closeApp, err := application.New(settings.DatabaseSettings{
		User: connection.User, Password: connection.Password, Host: connection.Host,
		Port: strconv.Itoa(int(connection.Port)), Name: connection.Database, SSLMode: "disable",
	}, issuer, telemetry.Nop())
	if err != nil {
		t.Fatalf("application.New(): %v", err)
	}
	handlers, err := restapi.NewHandler(app.UseCase, issuer.NewULID, []byte(strings.Repeat("a", 32)))
	if err != nil {
		t.Fatalf("rest.NewHandler(): %v", err)
	}
	router := chi.NewRouter()
	router.Route("/api", func(r chi.Router) {
		r.Get("/allocation-rules", handlers.Allocation.GetRules)
		r.Put("/allocation-rules", handlers.Allocation.PutRules)
		r.Get("/allocation-plans", handlers.Allocation.GetPlans)
		r.Put("/allocation-plans", handlers.Allocation.PutPlans)
		r.Get("/allocation-weeks/{week_start}", handlers.Allocation.GetWeek)
		r.Post("/allocations", handlers.Allocation.Create)
		r.Patch("/allocations/{rule_id}/{occurrence_date}", handlers.Allocation.Update)
		r.Delete("/allocations/{rule_id}/{occurrence_date}", handlers.Allocation.Delete)
		r.Put("/daily-sleep-exceptions/{local_date}", handlers.Allocation.PutSleepException)
		r.Delete("/daily-sleep-exceptions/{local_date}", handlers.Allocation.DeleteSleepException)
		r.Post("/admin/jobs/prefill-allocations:run", handlers.Admin.RunAllocationPrefill)
		r.Get("/admin/operations/{id}", handlers.Admin.GetOperation)
		r.Post("/projects", handlers.Project.Create)
		r.Get("/projects/{id}", handlers.Project.Get)
		r.Put("/projects/{id}/members/{user_id}", handlers.ProjectMember.UpsertMember)
		r.Post("/projects/{id}:complete", handlers.Project.Complete)
		r.Post("/projects/{id}:reopen", handlers.Project.Reopen)
		r.Post("/projects/{id}/schedules", handlers.Schedule.CreateProjectRule)
		r.Patch("/users/me/sleep", handlers.User.UpdateSleepSettings)
		r.Patch("/users/me/minimum-gap", handlers.User.UpdateMinimumGap)
	})
	api := &allocationHTTP{pool: pool, router: router, closeApp: closeApp}
	api.registerCleanup(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := api.closeApp(ctx); err != nil {
			t.Errorf("shutdown allocation application: %v", err)
		}
	})
	return api
}

func (api *allocationHTTP) request(t *testing.T, actorID, method, path, body string, headers ...[2]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request = request.WithContext(context.WithValue(request.Context(), restapi.UserIDContextKey, actorID))
	for _, header := range headers {
		request.Header.Set(header[0], header[1])
	}
	recorder := httptest.NewRecorder()
	api.router.ServeHTTP(recorder, request)
	return recorder
}

func allocationCountForOrigin(t *testing.T, api *allocationHTTP, ruleID string, date time.Time) int {
	t.Helper()
	var count int
	if err := api.pool.QueryRow(t.Context(), `SELECT count(*) FROM project_time_allocations
		WHERE rule_id=$1 AND occurrence_date=$2 AND deleted_at IS NULL`, ruleID, date.Format("2006-01-02")).Scan(&count); err != nil {
		t.Fatalf("count concrete allocation origin: %v", err)
	}
	return count
}

func (api *allocationHTTP) seedUser(t *testing.T, timezone string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := api.pool.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone)
		VALUES($1,'Allocation','Integration',$2,'unused',now(),$3)`, id, id+"@allocation.test", timezone)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	api.users = append(api.users, id)
	api.registerCleanup(t)
	return id
}

// seedProjectMember creates an already accepted membership for fixture setup.
// Production memberships are created by explicit invitation acceptance.
func (api *allocationHTTP) seedProjectMember(t *testing.T, projectID, userID, ownerID, roleID string) {
	t.Helper()
	if _, err := api.pool.Exec(t.Context(), `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,$3,$4)`, projectID, userID, roleID, ownerID); err != nil {
		t.Fatalf("insert accepted test project member: %v", err)
	}
}

func (api *allocationHTTP) trackProject(t *testing.T, id string) {
	t.Helper()
	api.projects = append(api.projects, id)
	api.registerCleanup(t)
}

func (api *allocationHTTP) grantSystemAdmin(t *testing.T, userID string) {
	t.Helper()
	if _, err := api.pool.Exec(t.Context(), `INSERT INTO system_user_roles(user_id,role_id) VALUES($1,'sysadmin')`, userID); err != nil {
		t.Fatalf("grant test system admin: %v", err)
	}
}

func (api *allocationHTTP) registerCleanup(t *testing.T) {
	t.Helper()
	if api.cleanupSet {
		return
	}
	api.cleanupSet = true
	t.Cleanup(func() {
		ctx := context.Background()
		users := api.users
		projects := api.projects
		tx, err := api.pool.Begin(ctx)
		if err != nil {
			t.Errorf("begin allocation fixture cleanup: %v", err)
			return
		}
		failed := false
		exec := func(query string, args ...any) {
			if _, err := tx.Exec(ctx, query, args...); err != nil {
				t.Errorf("allocation fixture cleanup: %v", err)
				failed = true
			}
		}
		if len(users) > 0 {
			exec(`DELETE FROM admin_operations WHERE requested_by = ANY($1::text[])`, users)
			exec(`DELETE FROM daily_sleep_exceptions WHERE user_id = ANY($1::text[])`, users)
			exec(`DELETE FROM project_time_allocation_exceptions WHERE user_id = ANY($1::text[])`, users)
			exec(`DELETE FROM project_time_allocations WHERE user_id = ANY($1::text[])`, users)
			exec(`DELETE FROM project_time_allocation_pause_intervals WHERE rule_id IN (SELECT id FROM project_time_allocation_rules WHERE user_id = ANY($1::text[]))`, users)
			exec(`DELETE FROM project_time_allocation_rule_revisions WHERE user_id = ANY($1::text[])`, users)
			exec(`DELETE FROM project_time_allocation_rules WHERE user_id = ANY($1::text[])`, users)
			exec(`DELETE FROM project_time_allocation_plans WHERE user_id = ANY($1::text[])`, users)
		}
		if len(projects) > 0 {
			exec(`DELETE FROM project_members WHERE project_id = ANY($1::text[])`, projects)
			exec(`DELETE FROM project_revisions WHERE id = ANY($1::text[])`, projects)
			exec(`DELETE FROM projects WHERE id = ANY($1::text[])`, projects)
		}
		if len(users) > 0 {
			exec(`DELETE FROM system_user_roles WHERE user_id = ANY($1::text[])`, users)
			exec(`DELETE FROM users WHERE id = ANY($1::text[])`, users)
		}
		if failed {
			_ = tx.Rollback(ctx)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			t.Errorf("commit allocation fixture cleanup: %v", err)
		}
	})
}
