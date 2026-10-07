package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/tag/dao"
	"github.com/Najah7/task2todaytodo/internal/application/tag/domain"
	tagusecase "github.com/Najah7/task2todaytodo/internal/application/tag/usecase"
	"github.com/go-chi/chi/v5"
)

type tagRESTID struct{}

func (tagRESTID) Generate() string { return "tag-new" }

type tagRESTRepository struct {
	rows  map[string]dao.Tag
	owner string
}

func (repo *tagRESTRepository) GetByUserID(_ context.Context, userID string, id domain.TagID) (dao.Tag, error) {
	row, ok := repo.rows[string(id)]
	if !ok || row.UserID != userID {
		return dao.Tag{}, tagusecase.ErrTagNotFound
	}
	return row, nil
}
func (repo *tagRESTRepository) ListByUserIDCursor(_ context.Context, userID string, limit int, anchor *tagusecase.CursorAnchor) ([]dao.Tag, error) {
	rows := make([]dao.Tag, 0)
	for _, row := range repo.rows {
		if row.UserID == userID && (anchor == nil || row.Name > anchor.Name || (row.Name == anchor.Name && row.ID > anchor.ID)) {
			rows = append(rows, row)
		}
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}
func (repo *tagRESTRepository) Create(_ context.Context, tag domain.Tag) (dao.Tag, error) {
	for _, row := range repo.rows {
		if row.UserID == tag.UserID && row.Name == tag.Name {
			return dao.Tag{}, tagusecase.ErrTagNameConflict
		}
	}
	row := dao.Tag{ID: string(tag.ID), UserID: tag.UserID, Name: tag.Name, CreatedAt: 1, UpdatedAt: 1}
	repo.rows[row.ID] = row
	return row, nil
}
func (repo *tagRESTRepository) RenameByUserID(_ context.Context, userID string, tag domain.Tag) (dao.Tag, error) {
	row, ok := repo.rows[string(tag.ID)]
	if !ok || row.UserID != userID {
		return dao.Tag{}, tagusecase.ErrTagNotFound
	}
	row.Name = tag.Name
	repo.rows[row.ID] = row
	return row, nil
}
func (repo *tagRESTRepository) DeleteByUserID(_ context.Context, userID string, id domain.TagID) error {
	row, ok := repo.rows[string(id)]
	if !ok || row.UserID != userID {
		return tagusecase.ErrTagNotFound
	}
	delete(repo.rows, row.ID)
	return nil
}

func tagRESTRequest(method, path, body, userID string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID != "" {
		request = request.WithContext(context.WithValue(request.Context(), UserIDContextKey, userID))
	}
	routeContext := chi.NewRouteContext()
	pathID := strings.TrimPrefix(path, "/tags/")
	if pathID == path {
		pathID = ""
	}
	routeContext.URLParams.Add("id", pathID)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func TestTagHandlerCatalogCRUDUsesAuthenticatedOwner(t *testing.T) {
	repo := &tagRESTRepository{rows: map[string]dao.Tag{}}
	handler := NewTagHandler(tagusecase.NewCatalogUseCases(repo, nil), tagRESTID{}, listTestCodec())
	create := httptest.NewRecorder()
	handler.Create(create, tagRESTRequest(http.MethodPost, "/tags", `{"name":"  Planning  "}`, "user-1"))
	if create.Code != http.StatusCreated || !strings.Contains(create.Body.String(), `"name":"Planning"`) {
		t.Fatalf("create status/body=%d/%s", create.Code, create.Body.String())
	}
	list := httptest.NewRecorder()
	handler.List(list, tagRESTRequest(http.MethodGet, "/tags", "", "user-1"))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"id":"tag-new"`) {
		t.Fatalf("list status/body=%d/%s", list.Code, list.Body.String())
	}
	rename := httptest.NewRecorder()
	handler.Rename(rename, tagRESTRequest(http.MethodPatch, "/tags/tag-new", `{"name":"Focus"}`, "user-1"))
	if rename.Code != http.StatusOK || !strings.Contains(rename.Body.String(), `"name":"Focus"`) {
		t.Fatalf("rename status/body=%d/%s", rename.Code, rename.Body.String())
	}
	delete := httptest.NewRecorder()
	handler.Delete(delete, tagRESTRequest(http.MethodDelete, "/tags/tag-new", "", "user-1"))
	if delete.Code != http.StatusOK || len(repo.rows) != 0 {
		t.Fatalf("delete status/remaining=%d/%d", delete.Code, len(repo.rows))
	}
}

func TestTagHandlerRejectsUnauthenticatedAndForeignOwner(t *testing.T) {
	repo := &tagRESTRepository{rows: map[string]dao.Tag{"private": {ID: "private", UserID: "owner", Name: "Private"}}}
	handler := NewTagHandler(tagusecase.NewCatalogUseCases(repo, nil), tagRESTID{}, listTestCodec())
	unauthorized := httptest.NewRecorder()
	handler.List(unauthorized, tagRESTRequest(http.MethodGet, "/tags", "", ""))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list status=%d", unauthorized.Code)
	}
	foreign := httptest.NewRecorder()
	handler.Get(foreign, tagRESTRequest(http.MethodGet, "/tags/private", "", "other"))
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign tag status/body=%d/%s", foreign.Code, foreign.Body.String())
	}
}
