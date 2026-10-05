package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/jackc/pgx/v5/pgconn"
)

const projectHandlerUserID = "user-1"

type projectHandlerID struct{ next int }

func (g *projectHandlerID) Generate() string {
	g.next++
	return fmt.Sprintf("generated-%d", g.next)
}

type projectHandlerProjectRepository struct {
	taskusecase.ProjectRepository
	projects    map[string]dao.Project
	listErr     error
	createErr   error
	updateErr   error
	deleteErr   error
	listCalls   int
	createCalls int
	getCalls    int
	updateCalls int
	deleteCalls int
	created     domain.Project
	updated     domain.Project
}

func (repo *projectHandlerProjectRepository) ListByUserID(_ context.Context, userID domain.UserID) ([]dao.Project, error) {
	repo.listCalls++
	if repo.listErr != nil {
		return nil, repo.listErr
	}
	rows := make([]dao.Project, 0, len(repo.projects))
	for _, row := range repo.projects {
		if row.UserID == string(userID) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (repo *projectHandlerProjectRepository) ListByUserIDCursor(_ context.Context, userID domain.UserID, limit int, _ *taskusecase.CursorAnchor) ([]dao.Project, error) {
	rows, err := repo.ListByUserID(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (repo *projectHandlerProjectRepository) Create(_ context.Context, project domain.Project) (dao.Project, error) {
	repo.createCalls++
	repo.created = project
	if repo.createErr != nil {
		return dao.Project{}, repo.createErr
	}
	row := projectHandlerProjectDAO(project)
	repo.projects[row.ID] = row
	return row, nil
}

func (repo *projectHandlerProjectRepository) GetByUserID(_ context.Context, userID domain.UserID, id domain.ProjectID) (dao.Project, error) {
	repo.getCalls++
	row, exists := repo.projects[string(id)]
	if !exists || row.UserID != string(userID) {
		return dao.Project{}, domain.ErrProjectNotFound
	}
	return row, nil
}

func (repo *projectHandlerProjectRepository) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.ProjectID, _ shared.Capability) (dao.Project, error) {
	return repo.GetByUserID(ctx, userID, id)
}

func (repo *projectHandlerProjectRepository) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.ProjectID, _ shared.Capability) (dao.Project, error) {
	return repo.GetByUserID(ctx, userID, id)
}

func (repo *projectHandlerProjectRepository) LockActiveTasksForDeletion(context.Context, domain.ProjectID) error {
	return nil
}

func (repo *projectHandlerProjectRepository) UpdateByUserID(_ context.Context, userID domain.UserID, project domain.Project, expectedRevision int32) (dao.Project, error) {
	repo.updateCalls++
	repo.updated = project
	if repo.updateErr != nil {
		return dao.Project{}, repo.updateErr
	}
	existing, ok := repo.projects[string(project.ID)]
	if !ok || existing.UserID != string(userID) {
		return dao.Project{}, domain.ErrProjectNotFound
	}
	if existing.Revision != expectedRevision {
		return dao.Project{}, taskusecase.ErrRevisionConflict
	}
	row := projectHandlerProjectDAO(project)
	row.Revision = expectedRevision + 1
	repo.projects[row.ID] = row
	return row, nil
}

func (repo *projectHandlerProjectRepository) DeleteByUserID(_ context.Context, userID domain.UserID, id domain.ProjectID, expectedRevision int32) error {
	repo.deleteCalls++
	if repo.deleteErr != nil {
		return repo.deleteErr
	}
	row, ok := repo.projects[string(id)]
	if !ok || row.UserID != string(userID) {
		return domain.ErrProjectNotFound
	}
	if row.Revision != expectedRevision {
		return taskusecase.ErrRevisionConflict
	}
	delete(repo.projects, string(id))
	return nil
}

type projectHandlerTaskRepository struct {
	taskusecase.TaskRepository
	tasks       map[string]dao.Task
	pageRows    []dao.Task
	createErr   error
	assignErr   error
	removeErr   error
	createCalls int
	getCalls    int
	assignCalls int
	removeCalls int
	pageCalls   int
	pageLimit   int
	pageOffset  int
}

func (repo *projectHandlerTaskRepository) ReadTaskProgressSources(_ context.Context, taskIDs, projectIDs []string, _ time.Time) (dao.TaskProgressSources, error) {
	requested := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		requested[id] = true
	}
	projects := make(map[string]bool, len(projectIDs))
	for _, id := range projectIDs {
		projects[id] = true
	}
	sources := dao.TaskProgressSources{Counts: make(map[string]dao.TaskProgressCounts), Statuses: make(map[string]dao.TaskStatus)}
	for _, row := range repo.tasks {
		if !requested[row.ID] && !projects[row.ProjectID] {
			continue
		}
		sources.Counts[row.ID] = dao.TaskProgressCounts{Total: 100, Completed: row.Progress}
		sources.Statuses[row.ID] = row.Status
		if projects[row.ProjectID] {
			sources.ProjectTasks = append(sources.ProjectTasks, dao.ProjectProgressTask{ID: row.ID, ProjectID: row.ProjectID, Status: row.Status})
		}
	}
	return sources, nil
}

func (repo *projectHandlerTaskRepository) ListByProjectAndUserIDCursor(_ context.Context, userID domain.UserID, projectID domain.ProjectID, limit int, _ *taskusecase.CursorAnchor) ([]dao.Task, error) {
	repo.pageCalls++
	repo.pageLimit, repo.pageOffset = limit, 0
	rows := make([]dao.Task, 0, len(repo.pageRows))
	for _, row := range repo.pageRows {
		if row.UserID == string(userID) && row.ProjectID == string(projectID) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (repo *projectHandlerTaskRepository) CreateInProject(_ context.Context, _ domain.UserID, task domain.Task) (dao.Task, error) {
	repo.createCalls++
	if repo.createErr != nil {
		return dao.Task{}, repo.createErr
	}
	row := projectHandlerTaskDAO(task)
	repo.tasks[row.ID] = row
	return row, nil
}

func (repo *projectHandlerTaskRepository) GetByUserID(_ context.Context, userID domain.UserID, id domain.TaskID) (dao.Task, error) {
	repo.getCalls++
	row, ok := repo.tasks[string(id)]
	if !ok || row.UserID != string(userID) {
		return dao.Task{}, taskusecase.ErrTaskNotFound
	}
	return row, nil
}

func (repo *projectHandlerTaskRepository) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, id)
}

func (repo *projectHandlerTaskRepository) AssignToProjectByUserID(_ context.Context, userID domain.UserID, id domain.TaskID, projectID domain.ProjectID, expectedRevision int32) (dao.Task, error) {
	repo.assignCalls++
	if repo.assignErr != nil {
		return dao.Task{}, repo.assignErr
	}
	row, ok := repo.tasks[string(id)]
	if !ok || row.UserID != string(userID) {
		return dao.Task{}, taskusecase.ErrTaskNotFound
	}
	if row.Revision != expectedRevision {
		return dao.Task{}, taskusecase.ErrRevisionConflict
	}
	row.ProjectID = string(projectID)
	row.Revision++
	repo.tasks[row.ID] = row
	return row, nil
}

func (repo *projectHandlerTaskRepository) RemoveFromProjectByUserID(_ context.Context, userID domain.UserID, id domain.TaskID, projectID domain.ProjectID, expectedRevision int32) (dao.Task, error) {
	repo.removeCalls++
	if repo.removeErr != nil {
		return dao.Task{}, repo.removeErr
	}
	row, ok := repo.tasks[string(id)]
	if !ok || row.UserID != string(userID) {
		return dao.Task{}, taskusecase.ErrTaskNotFound
	}
	if row.Revision != expectedRevision {
		return dao.Task{}, taskusecase.ErrRevisionConflict
	}
	if row.ProjectID == string(projectID) {
		row.ProjectID = ""
		row.Revision++
	}
	repo.tasks[row.ID] = row
	return row, nil
}

type projectHandlerRepositories struct {
	taskusecase.Repositories
	projects *projectHandlerProjectRepository
	tasks    *projectHandlerTaskRepository
}

func (repos projectHandlerRepositories) Projects() taskusecase.ProjectRepository {
	return repos.projects
}
func (repos projectHandlerRepositories) Tasks() taskusecase.TaskRepository { return repos.tasks }

type projectHandlerUOW struct {
	repos taskusecase.Repositories
	err   error
	calls int
}

func (uow *projectHandlerUOW) Do(ctx context.Context, fn func(context.Context, taskusecase.Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type projectHandlerHarness struct {
	handler  *ProjectHandler
	projects *projectHandlerProjectRepository
	tasks    *projectHandlerTaskRepository
	uow      *projectHandlerUOW
}

func newProjectHandlerHarness() projectHandlerHarness {
	projectRow := dao.Project{
		ID: "project-1", UserID: projectHandlerUserID,
		Type:  dao.ProjectType{Value: "work", Label: "Work", LabelJp: "仕事"},
		Title: "Current project", Goal: "Ship", Description: "Notes", Progress: 20,
		Priority:  dao.Priority{Value: "high", Label: "High", LabelJp: "高", Weight: 50},
		StartDate: projectHandlerString("2026-04-01"), EndDate: projectHandlerString("2026-04-30"),
		CreatedAt: 100, UpdatedAt: 200, Revision: 1,
	}
	projects := &projectHandlerProjectRepository{projects: map[string]dao.Project{"project-1": projectRow}}
	tasks := &projectHandlerTaskRepository{tasks: map[string]dao.Task{
		"task-1": {ID: "task-1", UserID: projectHandlerUserID, ProjectID: "project-old", Title: "Move me", Priority: dao.Priority{Value: "low", Label: "Low", Weight: 10}, Status: dao.TaskStatus{Value: "open", Label: "Open"}, Revision: 1},
	}}
	repos := projectHandlerRepositories{projects: projects, tasks: tasks}
	uow := &projectHandlerUOW{repos: repos}
	usecases := taskusecase.ProjectUseCases{
		List:       taskusecase.NewListProjectsUseCase(projects, tasks, nil),
		Create:     taskusecase.NewCreateProjectUseCase(projects, nil),
		Get:        taskusecase.NewGetProjectUseCase(projects, tasks, nil),
		Update:     taskusecase.NewUpdateProjectUseCase(projects, tasks, nil),
		Delete:     taskusecase.NewDeleteProjectUseCase(uow, nil),
		ListTasks:  taskusecase.NewListProjectTasksUseCase(projects, tasks, nil),
		CreateTask: taskusecase.NewCreateTaskInProjectUseCase(uow, nil),
		AddTask:    taskusecase.NewAddTaskToProjectUseCase(uow, tasks, nil),
		RemoveTask: taskusecase.NewRemoveTaskFromProjectUseCase(uow, tasks, nil),
	}
	return projectHandlerHarness{
		handler:  NewProjectHandler(usecases, &projectHandlerID{}, listTestCodec()),
		projects: projects, tasks: tasks, uow: uow,
	}
}

func projectHandlerRequest(method, target, body string, authenticated bool) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if authenticated && method != http.MethodGet && method != http.MethodHead && strings.Contains(target, "/projects/") {
		request.Header.Set(ifMatchHeader, `"1"`)
	}
	if strings.Contains(target, "/projects/") {
		request.SetPathValue("id", "project-1")
	}
	if authenticated {
		ctx := context.WithValue(request.Context(), UserIDContextKey, projectHandlerUserID)
		request = request.WithContext(ctx)
	}
	return request
}

func projectHandlerString(value string) *string { return &value }

func projectHandlerTaskDAO(task domain.Task) dao.Task {
	return dao.Task{
		ID: string(task.ID), UserID: string(task.UserID), ProjectID: string(task.ProjectID), Title: task.Title,
		Description: task.Description, DueDate: task.DueDate.Unix(), EstimatedMinutes: task.EstimatedMinutes,
		ActualMinutes: task.ActualMinutes, Progress: task.Progress,
		Priority: dao.Priority{Value: task.Priority.Value, Label: task.Priority.Label, LabelJp: task.Priority.LabelJp, Weight: task.Priority.Weight},
		Status:   dao.TaskStatus{Value: task.Status.Value, Label: task.Status.Label, LabelJp: task.Status.LabelJp}, Revision: 1,
	}
}

func projectHandlerProjectDAO(project domain.Project) dao.Project {
	var startDate, endDate *string
	if project.Schedule.StartDate != nil {
		startDate = projectHandlerString(project.Schedule.StartDate.Format("2006-01-02"))
	}
	if project.Schedule.EndDate != nil {
		endDate = projectHandlerString(project.Schedule.EndDate.Format("2006-01-02"))
	}
	return dao.Project{
		ID: string(project.ID), UserID: string(project.UserID),
		Type:  dao.ProjectType{Value: project.Type.Value, Label: project.Type.Label, LabelJp: project.Type.LabelJp},
		Title: project.Title, Goal: project.Goal, Description: project.Description, Progress: project.Progress,
		Priority:  dao.Priority{Value: project.Priority.Value, Label: project.Priority.Label, LabelJp: project.Priority.LabelJp, Weight: project.Priority.Weight},
		StartDate: startDate, EndDate: endDate, Revision: 1,
	}
}

func TestProjectHandlerListCreateGetAndDelete(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		h := newProjectHandlerHarness()
		response := httptest.NewRecorder()
		h.handler.List(response, projectHandlerRequest(http.MethodGet, "/projects", "", true))
		var got listEnvelope[ProjectResponse]
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || len(got.Items) != 1 || got.Items[0].ID != "project-1" || got.Items[0].Type.Value != "work" || got.Items[0].UserID != projectHandlerUserID {
			t.Errorf("List() = status %d, body %#v; want project-1", response.Code, got)
		}
	})
	t.Run("create", func(t *testing.T) {
		h := newProjectHandlerHarness()
		response := httptest.NewRecorder()
		h.handler.Create(response, projectHandlerRequest(http.MethodPost, "/projects", `{"title":"New","start_date":"2026-05-03"}`, true))
		var got ProjectResponse
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusCreated || got.ID != "generated-1" || got.Type.Value != "other" || got.Priority.Value != "low" || got.StartDate == nil || *got.StartDate != "2026-05-03" || got.EndDate != nil {
			t.Errorf("Create() = status %d, body %#v; want generated project with one-sided date and defaults", response.Code, got)
		}
		if h.projects.created.UserID != projectHandlerUserID || h.projects.created.Schedule.EndDate != nil {
			t.Errorf("created domain project = %#v; want authenticated owner and optional end date", h.projects.created)
		}
	})
	t.Run("get", func(t *testing.T) {
		h := newProjectHandlerHarness()
		response := httptest.NewRecorder()
		h.handler.Get(response, projectHandlerRequest(http.MethodGet, "/projects/project-1", "", true))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"id":"project-1"`) {
			t.Errorf("Get() = status %d, body %s; want project-1", response.Code, response.Body.String())
		}
	})
	t.Run("delete", func(t *testing.T) {
		h := newProjectHandlerHarness()
		response := httptest.NewRecorder()
		h.handler.Delete(response, projectHandlerRequest(http.MethodDelete, "/projects/project-1", "", true))
		if response.Code != http.StatusNoContent || h.projects.deleteCalls != 1 {
			t.Errorf("Delete() = status %d, deletes %d; want 204 and one delete", response.Code, h.projects.deleteCalls)
		}
	})
}

func TestProjectHandlerUsesPathParametersRegisteredByChi(t *testing.T) {
	h := newProjectHandlerHarness()
	router := chi.NewRouter()
	router.Get("/projects/{id}", h.handler.Get)
	request := projectHandlerRequest(http.MethodGet, "/projects/project-1", "", true)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"id":"project-1"`) {
		t.Errorf("Chi route GET /projects/{id} = status %d, body %s; want project-1", response.Code, response.Body.String())
	}
}

func TestProjectHandlerUpdatePreservesOmittedFieldsAndAppliesDateNull(t *testing.T) {
	h := newProjectHandlerHarness()
	response := httptest.NewRecorder()
	h.handler.Update(response, projectHandlerRequest(http.MethodPatch, "/projects/project-1", `{"goal":null,"start_date":"2026-04-05","end_date":null}`, true))
	var got ProjectResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || got.Title != "Current project" || got.Goal != "" || got.Description != "Notes" || got.StartDate == nil || *got.StartDate != "2026-04-05" || got.EndDate != nil {
		t.Errorf("Update() = status %d, body %#v; want omitted fields preserved, nullable fields changed", response.Code, got)
	}
	if h.projects.updated.Schedule.EndDate != nil || h.projects.updated.Schedule.StartDate == nil || h.projects.updated.Schedule.StartDate.Format("2006-01-02") != "2026-04-05" {
		t.Errorf("updated schedule = %#v; want new start date and cleared end date", h.projects.updated.Schedule)
	}
}

func TestProjectHandlerProjectTaskRoutesAndPagination(t *testing.T) {
	t.Run("list tasks pages project-owned rows", func(t *testing.T) {
		h := newProjectHandlerHarness()
		h.tasks.pageRows = []dao.Task{
			{ID: "task-a", UserID: projectHandlerUserID, ProjectID: "project-1", Title: "A"},
			{ID: "task-b", UserID: projectHandlerUserID, ProjectID: "project-1", Title: "B"},
		}
		response := httptest.NewRecorder()
		h.handler.ListTasks(response, projectHandlerRequest(http.MethodGet, "/projects/project-1/tasks?page_size=1", "", true))
		var got ProjectTaskPageResponse
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || len(got.Items) != 1 || got.Items[0].ID != "task-a" || h.tasks.pageLimit != 2 {
			t.Errorf("ListTasks() = status %d, body %#v, repository page (%d,%d); want one item, fetch limit 2", response.Code, got, h.tasks.pageLimit, h.tasks.pageOffset)
		}
	})
	t.Run("create task", func(t *testing.T) {
		h := newProjectHandlerHarness()
		response := httptest.NewRecorder()
		h.handler.CreateTask(response, projectHandlerRequest(http.MethodPost, "/projects/project-1/tasks", `{"title":"New task","due_date":"2026-05-03"}`, true))
		var got ProjectTaskResponse
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusCreated || got.ID != "generated-1" || got.ProjectID != "project-1" || got.Priority.Value != "high" || got.Status.Value != "open" || got.DueDate == nil || *got.DueDate != "2026-05-03" || h.uow.calls != 1 {
			t.Errorf("CreateTask() = status %d, body %#v, UOW %d; want project defaults and one transaction", response.Code, got, h.uow.calls)
		}
	})
	t.Run("assignment moves one task", func(t *testing.T) {
		h := newProjectHandlerHarness()
		h.projects.projects["project-old"] = dao.Project{ID: "project-old", UserID: projectHandlerUserID, Revision: 1}
		response := httptest.NewRecorder()
		h.handler.AddTask(response, projectHandlerRequest(http.MethodPost, "/projects/project-1/tasks:add", `{"task_id":"task-1"}`, true))
		var got ProjectTaskResponse
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || got.ProjectID != "project-1" || h.tasks.assignCalls != 1 || h.uow.calls != 1 {
			t.Errorf("AddTask() = status %d, project %q, assignments %d, UOW %d; want one move into project", response.Code, got.ProjectID, h.tasks.assignCalls, h.uow.calls)
		}
	})
	t.Run("removal detaches one task", func(t *testing.T) {
		h := newProjectHandlerHarness()
		h.tasks.tasks["task-1"] = dao.Task{ID: "task-1", UserID: projectHandlerUserID, ProjectID: "project-1", Title: "Remove me", Revision: 1}
		response := httptest.NewRecorder()
		h.handler.RemoveTask(response, projectHandlerRequest(http.MethodPost, "/projects/project-1/tasks:remove", `{"task_id":"task-1"}`, true))
		var got ProjectTaskResponse
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || got.ProjectID != "" || h.tasks.removeCalls != 1 || h.uow.calls != 1 {
			t.Errorf("RemoveTask() = status %d, project %q, removals %d, UOW %d; want one detached task", response.Code, got.ProjectID, h.tasks.removeCalls, h.uow.calls)
		}
	})
}

func TestProjectHandlerMapsUnauthorizedBadRequestNotFoundConflictAndInternalError(t *testing.T) {
	t.Run("unauthorized", func(t *testing.T) {
		h := newProjectHandlerHarness()
		response := httptest.NewRecorder()
		h.handler.Get(response, projectHandlerRequest(http.MethodGet, "/projects/project-1", "", false))
		if response.Code != http.StatusUnauthorized {
			t.Errorf("Get() status = %d, want 401", response.Code)
		}
	})
	t.Run("invalid JSON and invalid date", func(t *testing.T) {
		for _, body := range []string{"{", `{"title":"x","start_date":"04/05/2026"}`} {
			h := newProjectHandlerHarness()
			response := httptest.NewRecorder()
			h.handler.Create(response, projectHandlerRequest(http.MethodPost, "/projects", body, true))
			if response.Code != http.StatusBadRequest {
				t.Errorf("Create(%q) status = %d, want 400", body, response.Code)
			}
		}
	})
	t.Run("not found", func(t *testing.T) {
		h := newProjectHandlerHarness()
		request := projectHandlerRequest(http.MethodGet, "/projects/missing", "", true)
		request.SetPathValue("id", "missing")
		response := httptest.NewRecorder()
		h.handler.Get(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("Get() status = %d, want 404", response.Code)
		}
	})
	t.Run("conflict", func(t *testing.T) {
		h := newProjectHandlerHarness()
		h.projects.createErr = &pgconn.PgError{Code: "23505", ConstraintName: "projects_pkey"}
		response := httptest.NewRecorder()
		h.handler.Create(response, projectHandlerRequest(http.MethodPost, "/projects", `{"title":"Duplicate"}`, true))
		if response.Code != http.StatusConflict {
			t.Errorf("Create() status = %d, want 409", response.Code)
		}
	})
	t.Run("internal failure is sanitized", func(t *testing.T) {
		h := newProjectHandlerHarness()
		h.projects.listErr = errors.New("sensitive database detail")
		response := httptest.NewRecorder()
		h.handler.List(response, projectHandlerRequest(http.MethodGet, "/projects", "", true))
		if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "sensitive database detail") {
			t.Errorf("List() = status %d, body %s; want safe 500 response", response.Code, response.Body.String())
		}
	})
}

func TestProjectHandlerRejectsNullRequiredPatchAndMultipleTaskIDs(t *testing.T) {
	t.Run("required null", func(t *testing.T) {
		h := newProjectHandlerHarness()
		response := httptest.NewRecorder()
		h.handler.Update(response, projectHandlerRequest(http.MethodPatch, "/projects/project-1", `{"title":null}`, true))
		if response.Code != http.StatusBadRequest || h.projects.updateCalls != 0 {
			t.Errorf("Update() = status %d, update calls %d; want 400 before save", response.Code, h.projects.updateCalls)
		}
	})
	t.Run("multiple task ids are not accepted", func(t *testing.T) {
		h := newProjectHandlerHarness()
		response := httptest.NewRecorder()
		h.handler.AddTask(response, projectHandlerRequest(http.MethodPost, "/projects/project-1/tasks:add", `{"task_ids":["task-1","task-2"]}`, true))
		if response.Code != http.StatusBadRequest || h.tasks.assignCalls != 0 {
			t.Errorf("AddTask() = status %d, assignments %d; want 400 without assignment", response.Code, h.tasks.assignCalls)
		}
	})
}

func TestProjectHandlerRejectsInvalidProjectTaskPagination(t *testing.T) {
	h := newProjectHandlerHarness()
	response := httptest.NewRecorder()
	h.handler.ListTasks(response, projectHandlerRequest(http.MethodGet, "/projects/project-1/tasks?limit=101", "", true))
	if response.Code != http.StatusBadRequest || h.tasks.pageCalls != 0 {
		t.Errorf("ListTasks() = status %d, task queries %d; want 400 before repository access", response.Code, h.tasks.pageCalls)
	}
}

func TestProjectHandlerRequestDateParsingUsesCalendarDay(t *testing.T) {
	day, err := time.Parse("2006-01-02", "2026-08-09")
	if err != nil || day.Location() != time.UTC {
		t.Fatalf("date parse = %v, %v; want UTC calendar day", day, err)
	}
	if _, err := parseProjectTaskDueDate(projectHandlerString("2026-08-09")); err != nil {
		t.Fatalf("date parse error = %v", err)
	}
	if _, err := parseProjectTaskDueDate(projectHandlerString("2026-08-09T10:00:00Z")); err == nil {
		t.Fatal("date parser accepted RFC3339 value, want YYYY-MM-DD")
	}
}

func TestProjectHandlerListResponseDoesNotExposeOtherUsersProjects(t *testing.T) {
	h := newProjectHandlerHarness()
	other := h.projects.projects["project-1"]
	other.ID, other.UserID = "foreign-project", "other-user"
	h.projects.projects[other.ID] = other
	response := httptest.NewRecorder()
	h.handler.List(response, projectHandlerRequest(http.MethodGet, "/projects", "", true))
	var got listEnvelope[ProjectResponse]
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || len(got.Items) != 1 || got.Items[0].ID != "project-1" {
		t.Errorf("List() = status %d, items %#v; want authenticated user's project only", response.Code, got.Items)
	}
}

func TestProjectHandlerCreateTaskRejectsMalformedDueDate(t *testing.T) {
	for _, dueDate := range []string{"not-a-date", "2026-08-09T10:00:00Z"} {
		t.Run(dueDate, func(t *testing.T) {
			h := newProjectHandlerHarness()
			response := httptest.NewRecorder()
			body := `{"title":"x","due_date":"` + dueDate + `"}`
			h.handler.CreateTask(response, projectHandlerRequest(http.MethodPost, "/projects/project-1/tasks", body, true))
			if response.Code != http.StatusBadRequest || h.tasks.createCalls != 0 {
				t.Errorf("CreateTask() = status %d, create calls %d; want 400 before write", response.Code, h.tasks.createCalls)
			}
		})
	}
}

func TestProjectHandlerCreateTaskMapsConflict(t *testing.T) {
	h := newProjectHandlerHarness()
	h.tasks.createErr = &pgconn.PgError{Code: "23505", ConstraintName: "tasks_pkey"}
	response := httptest.NewRecorder()
	h.handler.CreateTask(response, projectHandlerRequest(http.MethodPost, "/projects/project-1/tasks", `{"title":"duplicate"}`, true))
	if response.Code != http.StatusConflict {
		t.Errorf("CreateTask() status = %d, want 409", response.Code)
	}
}

func TestProjectHandlerTaskResponseHasExpectedMapping(t *testing.T) {
	priority := dao.Priority{Value: "high", Label: "High", LabelJp: "高", Weight: 50}
	status := dao.TaskStatus{Value: "open", Label: "Open", LabelJp: "未着手"}
	got := projectTaskResponse(dao.Task{ID: "t", ProjectID: "p", DueDate: time.Date(2026, time.April, 1, 9, 0, 0, 0, time.UTC).Unix(), Priority: priority, Status: status})
	if got.DueDate == nil || *got.DueDate != "2026-04-01" || got.Priority.Value != "high" || got.Status.Value != "open" {
		t.Errorf("projectTaskResponse() = %#v", got)
	}
	if got.EstimatedMinutes != nil {
		t.Errorf("EstimatedMinutes = %v, want nil", got.EstimatedMinutes)
	}
}

func TestProjectHandlerDeleteErrorsMapNotFound(t *testing.T) {
	h := newProjectHandlerHarness()
	request := projectHandlerRequest(http.MethodDelete, "/projects/missing", "", true)
	request.SetPathValue("id", "missing")
	h.projects.deleteErr = domain.ErrProjectNotFound
	response := httptest.NewRecorder()
	h.handler.Delete(response, request)
	if response.Code != http.StatusNotFound {
		t.Errorf("Delete() status = %d, want 404", response.Code)
	}
}

func TestProjectHandlerExtraJSONValueIsRejected(t *testing.T) {
	h := newProjectHandlerHarness()
	response := httptest.NewRecorder()
	h.handler.Create(response, projectHandlerRequest(http.MethodPost, "/projects", `{"title":"x"} {}`, true))
	if response.Code != http.StatusBadRequest || h.projects.createCalls != 0 {
		t.Errorf("Create() = status %d, create calls %d; want 400 before write", response.Code, h.projects.createCalls)
	}
}

func TestProjectHandlerPatchUnknownFieldIsRejected(t *testing.T) {
	h := newProjectHandlerHarness()
	response := httptest.NewRecorder()
	h.handler.Update(response, projectHandlerRequest(http.MethodPatch, "/projects/project-1", `{"progress":100}`, true))
	if response.Code != http.StatusBadRequest || h.projects.updateCalls != 0 {
		t.Errorf("Update() = status %d, update calls %d; want 400 before write", response.Code, h.projects.updateCalls)
	}
}

func TestProjectHandlerQueryRejectsDuplicatePaginationValues(t *testing.T) {
	h := newProjectHandlerHarness()
	response := httptest.NewRecorder()
	h.handler.ListTasks(response, projectHandlerRequest(http.MethodGet, "/projects/project-1/tasks?limit=1&limit=2", "", true))
	if response.Code != http.StatusBadRequest || h.tasks.pageCalls != 0 {
		t.Errorf("ListTasks() = status %d, task queries %d; want 400 before repository access", response.Code, h.tasks.pageCalls)
	}
}

func TestProjectHandlerProjectDateResponseCopiesPointers(t *testing.T) {
	original := "2026-04-01"
	got := projectResponse(dao.Project{StartDate: &original})
	if got.StartDate == nil || *got.StartDate != original || got.StartDate == &original {
		t.Errorf("projectResponse() start date = %v; want independent copy", got.StartDate)
	}
}
