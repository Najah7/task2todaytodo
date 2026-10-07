package rest

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/tag/dao"
	"github.com/Najah7/task2todaytodo/internal/application/tag/domain"
	tagusecase "github.com/Najah7/task2todaytodo/internal/application/tag/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/go-chi/chi/v5"
)

const tagResource = "tags"

var (
	tagCreateFailure = NewFailureErrSpec(tagResource, ActionCreate, "Failed to create tag")
	tagListFailure   = NewFailureErrSpec(tagResource, ActionList, "Failed to list tags")
	tagGetFailure    = NewFailureErrSpec(tagResource, ActionGet, "Failed to get tag")
	tagUpdateFailure = NewFailureErrSpec(tagResource, ActionUpdate, "Failed to update tag")
	tagDeleteFailure = NewFailureErrSpec(tagResource, ActionDelete, "Failed to delete tag")
)

type TagHandler struct {
	tags       tagusecase.CatalogUseCases
	ID         shared.ID
	pageTokens *pagination.Codec
}

func NewTagHandler(tags tagusecase.CatalogUseCases, id shared.ID, codecs ...*pagination.Codec) *TagHandler {
	h := &TagHandler{tags: tags, ID: id}
	if len(codecs) > 0 {
		h.pageTokens = codecs[0]
	}
	return h
}

type TagResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
type TagListResponse struct {
	Items         []TagResponse `json:"items"`
	NextPageToken string        `json:"next_page_token"`
}
type TagCreateRequest struct {
	Name string `json:"name"`
}
type TagRenameRequest struct {
	Name string `json:"name"`
}

func tagResponse(row dao.Tag) TagResponse {
	return TagResponse{ID: row.ID, Name: row.Name, CreatedAt: time.Unix(row.CreatedAt, 0).UTC().Format(time.RFC3339), UpdatedAt: time.Unix(row.UpdatedAt, 0).UTC().Format(time.RFC3339)}
}

// List returns the authenticated user's shared Tag catalog.
//
//	@Summary	List tags
//	@Tags		Tags
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page_size	query		int		false	"Items per page"
//	@Param		page_token	query		string	false	"Opaque next page token"
//	@Param		fields		query		string	false	"Response field mask"
//	@Success	200			{object}	TagListResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	500			{object}	ErrResponse
//	@Router		/tags [get]
func (h *TagHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := tagUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, tagListFailure, ErrDetailUnauthorized)
		return
	}
	request, err := parseListRequest(r, h.pageTokens, userID, tagResource, "", "name_asc_id_asc", listEnvelope[TagResponse]{})
	if err != nil {
		writeListError(w, tagListFailure, err)
		return
	}
	var anchor *tagusecase.CursorAnchor
	if request.Anchor != nil {
		anchor = &tagusecase.CursorAnchor{Name: request.Anchor.Name, ID: request.Anchor.ID}
	}
	page, err := h.tags.List.ExecutePage(r.Context(), userID, tagusecase.CursorPageRequest{Size: request.Size, Anchor: anchor})
	if err != nil {
		writeTagUseCaseError(w, tagListFailure, err)
		return
	}
	items := make([]TagResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, tagResponse(row))
		anchors = append(anchors, listAnchor{Name: row.Name, ID: row.ID})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, tagListFailure)
}

// Create creates a Tag in the authenticated user's shared Tag catalog.
//
//	@Summary	Create tag
//	@Tags		Tags
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body		TagCreateRequest	true	"Tag"
//	@Success	201		{object}	TagResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	409		{object}	ErrResponse
//	@Router		/tags [post]
func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := tagUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, tagCreateFailure, ErrDetailUnauthorized)
		return
	}
	var request TagCreateRequest
	if !decodeTagJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, tagCreateFailure, ErrDetailInvalidRequestBody)
		return
	}
	row, err := h.tags.Create.Execute(r.Context(), tagusecase.CreateTagInput{ID: domain.TagID(h.ID.Generate()), UserID: userID, Name: request.Name})
	if err != nil {
		writeTagUseCaseError(w, tagCreateFailure, err)
		return
	}
	WriteJSON(w, http.StatusCreated, tagResponse(row))
}

// Get returns a Tag owned by the authenticated user.
//
//	@Summary	Get tag
//	@Tags		Tags
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Tag ID"
//	@Success	200	{object}	TagResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Router		/tags/{id} [get]
func (h *TagHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := tagRequestIDs(w, r, tagGetFailure)
	if !ok {
		return
	}
	row, err := h.tags.Get.Execute(r.Context(), userID, id)
	if err != nil {
		writeTagUseCaseError(w, tagGetFailure, err)
		return
	}
	WriteJSON(w, http.StatusOK, tagResponse(row))
}

// Rename changes the display name of an owned Tag.
//
//	@Summary	Rename tag
//	@Tags		Tags
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string				true	"Tag ID"
//	@Param		request	body		TagRenameRequest	true	"New tag name"
//	@Success	200		{object}	TagResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	409		{object}	ErrResponse
//	@Router		/tags/{id} [patch]
func (h *TagHandler) Rename(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := tagRequestIDs(w, r, tagUpdateFailure)
	if !ok {
		return
	}
	var request TagRenameRequest
	if !decodeTagJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, tagUpdateFailure, ErrDetailInvalidRequestBody)
		return
	}
	row, err := h.tags.Rename.Execute(r.Context(), tagusecase.RenameTagInput{UserID: userID, ID: id, Name: request.Name})
	if err != nil {
		writeTagUseCaseError(w, tagUpdateFailure, err)
		return
	}
	WriteJSON(w, http.StatusOK, tagResponse(row))
}

// Delete removes a Tag and clears its Task and Schedule assignments.
//
//	@Summary	Delete tag
//	@Tags		Tags
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Tag ID"
//	@Success	200	{object}	MessageResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Router		/tags/{id} [delete]
func (h *TagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := tagRequestIDs(w, r, tagDeleteFailure)
	if !ok {
		return
	}
	if err := h.tags.Delete.Execute(r.Context(), userID, id); err != nil {
		writeTagUseCaseError(w, tagDeleteFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "Tag deleted")
}

func tagUserID(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(UserIDContextKey).(string)
	return value, ok && value != ""
}
func tagRequestIDs(w http.ResponseWriter, r *http.Request, spec ErrSpec) (string, domain.TagID, bool) {
	userID, ok := tagUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return "", "", false
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("id", "required", "Tag ID is required"))
		return "", "", false
	}
	return userID, domain.TagID(id), true
}
func decodeTagJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	return decodeScheduleJSON(w, r, dst)
}
func writeTagUseCaseError(w http.ResponseWriter, spec ErrSpec, err error) {
	switch {
	case errors.Is(err, tagusecase.ErrTagNotFound):
		WriteError(w, http.StatusNotFound, spec, NewErrDetail("id", "not_found", "Tag was not found"))
	case errors.Is(err, tagusecase.ErrTagNameConflict), IsUniqueConstraint(err, "tags_user_id_name_key"):
		WriteError(w, http.StatusConflict, spec, NewErrDetail("name", "tag_name_conflict", "Tag name already exists"))
	case errors.Is(err, domain.ErrTagIDEmpty):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("id", "required", "Tag ID is required"))
	case errors.Is(err, domain.ErrTagUserIDEmpty):
		WriteError(w, http.StatusBadRequest, spec, ErrDetailUnauthorized)
	case errors.Is(err, domain.ErrTagNameEmpty):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("name", "required", "Tag name is required"))
	case errors.Is(err, tagusecase.ErrInvalidTagPage):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("page_size", "invalid_page_size", "Page size is invalid"))
	default:
		WriteError(w, http.StatusInternalServerError, spec, ErrDetailInternalServerError)
	}
}
