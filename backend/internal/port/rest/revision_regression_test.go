package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/go-chi/chi/v5"
)

func TestTaskUpdateIfMatchStatusesAndSuccessETag(t *testing.T) {
	for _, tc := range []struct {
		name           string
		ifMatch        string
		setIfMatch     bool
		updateErr      error
		wantStatus     int
		wantETag       string
		wantUpdateCall bool
		wantRevision   int32
	}{
		{name: "missing precondition", wantStatus: http.StatusPreconditionRequired},
		{name: "malformed revision", ifMatch: `not-a-tag`, setIfMatch: true, wantStatus: http.StatusBadRequest},
		{name: "stale revision", ifMatch: `"1"`, setIfMatch: true, updateErr: taskusecase.ErrRevisionConflict, wantStatus: http.StatusConflict, wantUpdateCall: true, wantRevision: 1},
		{name: "current revision", ifMatch: `"1"`, setIfMatch: true, wantStatus: http.StatusOK, wantETag: `"2"`, wantUpdateCall: true, wantRevision: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &revisionUpdateRepository{task: revisionTaskDAO(), updateErr: tc.updateErr}
			handler := NewTaskHandler(taskusecase.TaskUseCases{
				Update: taskusecase.NewUpdateTaskUseCase(repo, repo, nil),
			}, taskHandlerID{value: "unused"}, listTestCodec())
			request := newRevisionTaskRequest(http.MethodPatch, "/tasks/task-1", `{"title":"Updated title"}`, "user-1")
			if tc.setIfMatch {
				request.Header.Set("If-Match", tc.ifMatch)
			}
			response := httptest.NewRecorder()
			handler.Update(response, request)

			if response.Code != tc.wantStatus {
				t.Fatalf("response status = %d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
			}
			if repo.updateCalls != boolInt(tc.wantUpdateCall) {
				t.Fatalf("repository update calls = %d, want %d", repo.updateCalls, boolInt(tc.wantUpdateCall))
			}
			if tc.wantUpdateCall && repo.expectedRevision != tc.wantRevision {
				t.Fatalf("expected revision passed to repository = %d, want %d", repo.expectedRevision, tc.wantRevision)
			}
			if got := response.Header().Get("ETag"); got != tc.wantETag {
				t.Fatalf("ETag = %q, want %q", got, tc.wantETag)
			}
			if tc.wantStatus == http.StatusOK {
				var body TaskResponse
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode successful response: %v", err)
				}
				if body.Revision != 2 || body.Title != "Updated title" {
					t.Fatalf("response task = revision %d title %q; want revision 2 with updated title", body.Revision, body.Title)
				}
			}
		})
	}
}

func TestTaskAssignRejectsWhitespaceAssigneeID(t *testing.T) {
	uow := &assignmentNeverUOW{}
	handler := NewTaskHandler(taskusecase.TaskUseCases{
		Assign: taskusecase.NewAssignTaskUseCase(uow, nil),
	}, taskHandlerID{value: "unused"}, listTestCodec())
	request := newRevisionTaskRequest(http.MethodPatch, "/tasks/task-1/assignees", `{"assignee_id":"   "}`, "user-1")
	request.Header.Set(ifMatchHeader, `"1"`)
	response := httptest.NewRecorder()
	handler.Assign(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("whitespace assignee status = %d, want 400; body=%s", response.Code, response.Body.String())
	}
	var body ErrResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode whitespace assignee error: %v", err)
	}
	if len(body.Error.Details) != 1 || body.Error.Details[0].Code != "required" {
		t.Fatalf("whitespace assignee details = %+v, want required validation error", body.Error.Details)
	}
	if uow.calls != 0 {
		t.Fatalf("assignment UOW calls = %d, want validation before persistence", uow.calls)
	}
}

type assignmentNeverUOW struct{ calls int }

func (uow *assignmentNeverUOW) Do(context.Context, func(context.Context, taskusecase.Repositories) error) error {
	uow.calls++
	return nil
}

func TestTaskRevisionPageTokenBindsActorAndTaskResource(t *testing.T) {
	reader := &revisionHistoryReader{rows: []dao.TaskRevision{
		{ID: "task-1", Revision: 3, UserID: "user-1", AssigneeID: "user-1", Title: "Third revision"},
		{ID: "task-1", Revision: 2, UserID: "user-1", AssigneeID: "user-1", Title: "Second revision"},
	}}
	handler := NewTaskHandler(taskusecase.TaskUseCases{
		Revisions: taskusecase.NewListTaskRevisionsUseCase(reader, nil),
	}, taskHandlerID{value: "unused"}, listTestCodec())

	first := httptest.NewRecorder()
	handler.ListRevisions(first, newRevisionTaskRequest(http.MethodGet, "/tasks/task-1/revisions?page_size=1", "", "user-1"))
	if first.Code != http.StatusOK {
		t.Fatalf("first revision page status = %d; body=%s", first.Code, first.Body.String())
	}
	var firstPage TaskRevisionListResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstPage); err != nil {
		t.Fatalf("decode first revision page: %v", err)
	}
	if len(firstPage.Items) != 1 || firstPage.Items[0].Revision != 3 || firstPage.NextPageToken == "" {
		t.Fatalf("first revision page = %+v; want revision 3 and a continuation token", firstPage)
	}

	continued := httptest.NewRecorder()
	continuedTarget := "/tasks/task-1/revisions?page_size=1&page_token=" + firstPage.NextPageToken
	handler.ListRevisions(continued, newRevisionTaskRequest(http.MethodGet, continuedTarget, "", "user-1"))
	if continued.Code != http.StatusOK {
		t.Fatalf("continued revision page status = %d; body=%s", continued.Code, continued.Body.String())
	}
	var secondPage TaskRevisionListResponse
	if err := json.Unmarshal(continued.Body.Bytes(), &secondPage); err != nil {
		t.Fatalf("decode continued revision page: %v", err)
	}
	if len(secondPage.Items) != 1 || secondPage.Items[0].Revision != 2 || secondPage.NextPageToken != "" {
		t.Fatalf("continued revision page = %+v; want revision 2 without another token", secondPage)
	}
	if len(reader.calls) != 2 || reader.calls[1].actor != "user-1" || reader.calls[1].task != "task-1" || reader.calls[1].anchorRevision != 3 {
		t.Fatalf("revision reader continuation calls = %+v; want same actor/task and anchor revision 3", reader.calls)
	}

	for _, tc := range []struct {
		name  string
		path  string
		actor string
	}{
		{name: "different actor", path: continuedTarget, actor: "user-2"},
		{name: "different task", path: strings.Replace(continuedTarget, "task-1", "task-2", 1), actor: "user-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ListRevisions(response, newRevisionTaskRequest(http.MethodGet, tc.path, "", tc.actor))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("mismatched history token status = %d, want 400; body=%s", response.Code, response.Body.String())
			}
			if len(reader.calls) != 2 {
				t.Fatalf("invalid scoped token reached history usecase; calls=%d", len(reader.calls))
			}
		})
	}
}

type revisionUpdateRepository struct {
	task             dao.Task
	updateErr        error
	expectedRevision int32
	updateCalls      int
}

func (repo *revisionUpdateRepository) GetByUserID(_ context.Context, _ domain.UserID, _ domain.TaskID) (dao.Task, error) {
	return repo.task, nil
}

func (repo *revisionUpdateRepository) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, taskID)
}

func (repo *revisionUpdateRepository) UpdateByUserID(_ context.Context, _ domain.UserID, task domain.Task, expected int32) (dao.Task, error) {
	repo.updateCalls++
	repo.expectedRevision = expected
	if repo.updateErr != nil {
		return dao.Task{}, repo.updateErr
	}
	repo.task.Title = task.Title
	repo.task.Description = task.Description
	repo.task.Revision = expected + 1
	repo.task.UpdatedAt = time.Now().Unix()
	return repo.task, nil
}

func (repo *revisionUpdateRepository) ReadTaskProgressSources(_ context.Context, taskIDs, _ []string, _ time.Time) (dao.TaskProgressSources, error) {
	counts, statuses := make(map[string]dao.TaskProgressCounts, len(taskIDs)), make(map[string]dao.TaskStatus, len(taskIDs))
	for _, taskID := range taskIDs {
		counts[taskID] = dao.TaskProgressCounts{Total: 1}
		statuses[taskID] = dao.TaskStatus{Value: "open"}
	}
	return dao.TaskProgressSources{Counts: counts, Statuses: statuses}, nil
}

type revisionHistoryReaderCall struct {
	actor          domain.UserID
	task           domain.TaskID
	anchorRevision int32
}

type revisionHistoryReader struct {
	rows  []dao.TaskRevision
	calls []revisionHistoryReaderCall
}

func (reader *revisionHistoryReader) ListTaskRevisionsByActor(_ context.Context, actor domain.UserID, task domain.TaskID, limit int, anchor *taskusecase.CursorAnchor) ([]dao.TaskRevision, error) {
	call := revisionHistoryReaderCall{actor: actor, task: task}
	if anchor != nil {
		call.anchorRevision = anchor.Revision
	}
	reader.calls = append(reader.calls, call)
	rows := make([]dao.TaskRevision, 0, len(reader.rows))
	for _, row := range reader.rows {
		if (anchor == nil || row.Revision < anchor.Revision) && row.ID == string(task) {
			rows = append(rows, row)
		}
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func revisionTaskDAO() dao.Task {
	createdAt := time.Date(2026, time.October, 5, 8, 0, 0, 0, time.UTC).Unix()
	return dao.Task{
		ID: "task-1", UserID: "user-1", AssigneeID: "user-1", Title: "Original title",
		Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "open"}, Revision: 1,
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}
}

func newRevisionTaskRequest(method, target, body, actor string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	parts := strings.Split(strings.TrimPrefix(target, "/tasks/"), "/")
	taskID := strings.SplitN(parts[0], "?", 2)[0]
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", taskID)
	ctx := context.WithValue(request.Context(), chi.RouteCtxKey, routeContext)
	if actor != "" {
		ctx = context.WithValue(ctx, UserIDContextKey, actor)
	}
	return request.WithContext(ctx)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
