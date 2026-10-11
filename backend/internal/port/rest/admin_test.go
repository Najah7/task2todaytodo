package rest

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	admindao "github.com/Najah7/task2todaytodo/internal/application/admin/dao"
	adminusecase "github.com/Najah7/task2todaytodo/internal/application/admin/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/go-chi/chi/v5"
)

type adminRepositoryFake struct {
	operation   admindao.Operation
	createCalls int
	getCalls    int
	listCalls   int
	permissions []shared.Capability
}

func (r *adminRepositoryFake) HasPermission(_ context.Context, actor string, capability shared.Capability) (bool, error) {
	r.permissions = append(r.permissions, capability)
	return actor == "sysadmin", nil
}
func (r *adminRepositoryFake) CreateOrGetActive(_ context.Context, o admindao.Operation, _ string, _ time.Duration) (admindao.Operation, bool, string, error) {
	r.createCalls++
	return o, true, "lease", nil
}
func (r *adminRepositoryFake) Get(context.Context, string) (admindao.Operation, error) {
	r.getCalls++
	return r.operation, nil
}
func (r *adminRepositoryFake) List(context.Context, string, int) ([]admindao.Operation, error) {
	r.listCalls++
	return []admindao.Operation{r.operation}, nil
}
func (r *adminRepositoryFake) Heartbeat(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}
func (r *adminRepositoryFake) Progress(context.Context, string, string, int64, int64, time.Duration) (bool, error) {
	return true, nil
}
func (r *adminRepositoryFake) Succeed(context.Context, string, string, int64, int64) error {
	return nil
}
func (r *adminRepositoryFake) Fail(context.Context, string, string, string, string) error { return nil }

type adminRunnerFake struct{}

func (adminRunnerFake) StartOperation(string, string, string, time.Time) {}

type adminIDFake struct{}

func (adminIDFake) Generate() string { return "01J00000000000000000000000" }

type adminClockFake struct{}

func (adminClockFake) Now() time.Time { return time.Unix(1, 0).UTC() }

func TestAdminEndpointsAuthorizeThroughUseCases(t *testing.T) {
	operation := admindao.Operation{ID: "01J00000000000000000000000", Name: "admin/operations/01J00000000000000000000000", JobName: "backfill-schedules", State: "running", CreatedAt: time.Unix(1, 0).UTC(), StartedAt: time.Unix(1, 0).UTC(), Cutoff: time.Unix(1, 0).UTC()}
	repo := &adminRepositoryFake{operation: operation}
	uses := adminusecase.UseCases{
		RunBackfill:    adminusecase.NewRunBackfillSchedulesUseCase(repo, adminRunnerFake{}, adminIDFake{}, adminClockFake{}, time.Minute),
		ListOperations: adminusecase.NewListOperationsUseCase(repo),
		GetOperation:   adminusecase.NewGetOperationUseCase(repo),
	}
	codec, err := pagination.NewCodec([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAdminHandler(uses, codec)

	requests := []struct {
		name string
		call func(context.Context) *httptest.ResponseRecorder
	}{
		{"run", func(ctx context.Context) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			handler.RunBackfill(w, httptest.NewRequest("POST", "/admin/jobs/backfill-schedules:run", nil).WithContext(ctx))
			return w
		}},
		{"list", func(ctx context.Context) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			handler.ListOperations(w, httptest.NewRequest("GET", "/admin/operations", nil).WithContext(ctx))
			return w
		}},
		{"get", func(ctx context.Context) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			router := chi.NewRouter()
			router.Get("/admin/operations/{id}", handler.GetOperation)
			router.ServeHTTP(w, httptest.NewRequest("GET", "/admin/operations/"+operation.ID, nil).WithContext(ctx))
			return w
		}},
	}
	for _, actor := range []string{"ordinary", "project-admin"} {
		for _, request := range requests {
			t.Run(actor+"/"+request.name, func(t *testing.T) {
				w := request.call(authenticatedRequest(actor))
				if w.Code != 403 {
					t.Fatalf("status = %d, want 403", w.Code)
				}
			})
		}
	}
	if repo.createCalls != 0 || repo.getCalls != 0 || repo.listCalls != 0 {
		t.Fatalf("denied requests reached operation storage: create=%d get=%d list=%d", repo.createCalls, repo.getCalls, repo.listCalls)
	}

	for _, request := range requests {
		t.Run("missing_actor/"+request.name, func(t *testing.T) {
			w := request.call(context.Background())
			if w.Code != 401 {
				t.Fatalf("status = %d, want 401", w.Code)
			}
		})
	}

	for _, request := range requests {
		t.Run("sysadmin/"+request.name, func(t *testing.T) {
			w := request.call(authenticatedRequest("sysadmin"))
			wantStatus := 200
			if request.name == "run" {
				wantStatus = 202
			}
			if w.Code != wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, wantStatus, w.Body.String())
			}
			if request.name == "get" {
				var response OperationResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Name != operation.Name || response.Done {
					t.Fatalf("response = %+v", response)
				}
			}
		})
	}
	wantCapabilities := []shared.Capability{
		shared.AdminJobRun(), shared.AdminOperationRead(), shared.AdminOperationRead(),
		shared.AdminJobRun(), shared.AdminOperationRead(), shared.AdminOperationRead(),
		shared.AdminJobRun(), shared.AdminOperationRead(), shared.AdminOperationRead(),
	}
	if len(repo.permissions) != len(wantCapabilities) {
		t.Fatalf("permission checks = %d, want %d", len(repo.permissions), len(wantCapabilities))
	}
	for i, want := range wantCapabilities {
		if repo.permissions[i] != want {
			t.Errorf("permission check %d = %+v, want %+v", i, repo.permissions[i], want)
		}
	}
}

func authenticatedRequest(user string) context.Context {
	return context.WithValue(context.Background(), UserIDContextKey, user)
}
