package rest

import (
	"errors"
	"net/http"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	projectdomain "github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
)

type ProjectMemberHandler struct {
	members *projectusecase.UseCases
}

func NewProjectMemberHandler(members *projectusecase.UseCases) *ProjectMemberHandler {
	return &ProjectMemberHandler{members: members}
}

type ProjectMemberResponse struct {
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	RoleID    string `json:"role_id"`
	RoleName  string `json:"role_name"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	AddedBy   string `json:"added_by"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type ProjectMemberListResponse struct {
	Items []ProjectMemberResponse `json:"items"`
}

type ProjectMemberUpsertRequest struct {
	RoleID string `json:"role_id"`
}

var (
	projectMemberListFailure   = NewFailureErrSpec(ResourceProjects, "list_members", "Failed to list project members")
	projectMemberUpsertFailure = NewFailureErrSpec(ResourceProjects, "upsert_member", "Failed to update project member")
	projectMemberDeleteFailure = NewFailureErrSpec(ResourceProjects, "delete_member", "Failed to delete project member")
)

// ListMembers returns the members explicitly assigned to a project.
//
//	@Summary	List project members
//	@Tags		Projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Project ID"
//	@Success	200	{object}	ProjectMemberListResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/projects/{id}/members [get]
func (h *ProjectMemberHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	actor, ok := projectUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, projectMemberListFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectMemberListFailure)
	if !ok {
		return
	}
	members, err := h.members.Members.ListMembers.Execute(r.Context(), actor, projectID)
	if err != nil {
		writeProjectMemberUseCaseError(w, projectMemberListFailure, err)
		return
	}
	items := make([]ProjectMemberResponse, 0, len(members))
	for _, member := range members {
		items = append(items, projectMemberResponse(member))
	}
	WriteJSON(w, http.StatusOK, ProjectMemberListResponse{Items: items})
}

// UpsertMember adds a project member or changes the member's role.
//
//	@Summary	Add or update a project member
//	@Tags		Projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string						true	"Project ID"
//	@Param		user_id	path	string						true	"Member user ID"
//	@Param		request	body	ProjectMemberUpsertRequest	true	"Member role"
//	@Success	204
//	@Failure	400	{object}	ErrResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/projects/{id}/members/{user_id} [put]
func (h *ProjectMemberHandler) UpsertMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := projectUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, projectMemberUpsertFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectMemberUpsertFailure)
	if !ok {
		return
	}
	memberID, ok := projectMemberUserID(w, r, projectMemberUpsertFailure)
	if !ok {
		return
	}
	var request ProjectMemberUpsertRequest
	if !decodeProjectJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, projectMemberUpsertFailure, ErrDetailInvalidRequestBody)
		return
	}
	if request.RoleID == "" {
		WriteError(w, http.StatusBadRequest, projectMemberUpsertFailure, NewErrDetail("role_id", "required", "Role ID is required"))
		return
	}
	if err := h.members.Members.UpsertMember.Execute(r.Context(), actor, projectID, memberID, request.RoleID); err != nil {
		writeProjectMemberUseCaseError(w, projectMemberUpsertFailure, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteMember removes a project member and returns their assigned tasks to their owners.
//
//	@Summary	Remove a project member
//	@Tags		Projects
//	@Security	BearerAuth
//	@Param		id		path	string	true	"Project ID"
//	@Param		user_id	path	string	true	"Member user ID"
//	@Success	204
//	@Failure	401	{object}	ErrResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/projects/{id}/members/{user_id} [delete]
func (h *ProjectMemberHandler) DeleteMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := projectUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, projectMemberDeleteFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectMemberDeleteFailure)
	if !ok {
		return
	}
	memberID, ok := projectMemberUserID(w, r, projectMemberDeleteFailure)
	if !ok {
		return
	}
	if err := h.members.Members.DeleteMember.Execute(r.Context(), actor, projectID, memberID); err != nil {
		writeProjectMemberUseCaseError(w, projectMemberDeleteFailure, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func projectMemberUserID(w http.ResponseWriter, r *http.Request, spec ErrSpec) (projectdomain.UserID, bool) {
	id := r.PathValue("user_id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("user_id", "required", "Member user ID is required"))
		return "", false
	}
	return projectdomain.UserID(id), true
}

func projectMemberResponse(member dao.ProjectMember) ProjectMemberResponse {
	return ProjectMemberResponse{
		ProjectID: member.ProjectID,
		UserID:    member.UserID,
		RoleID:    member.RoleID,
		RoleName:  member.RoleName,
		FirstName: member.FirstName,
		LastName:  member.LastName,
		Email:     member.Email,
		AddedBy:   member.AddedBy,
		CreatedAt: member.CreatedAt,
		UpdatedAt: member.UpdatedAt,
	}
}

func writeProjectMemberUseCaseError(w http.ResponseWriter, spec ErrSpec, err error) {
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		WriteError(w, http.StatusNotFound, spec, NewErrDetail("id", "not_found", "Project was not found or membership access is not permitted"))
	case errors.Is(err, projectusecase.ErrProjectMemberNotFound):
		WriteError(w, http.StatusNotFound, spec, NewErrDetail("user_id", "not_found", "Project member was not found"))
	case errors.Is(err, projectusecase.ErrProjectMemberUpsertRejected):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("user_id", "member_not_allowed", "This user cannot be added as a project member"))
	case errors.Is(err, projectusecase.ErrProjectMemberRoleNotFound):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("role_id", "unknown_role", "The role ID is not supported"))
	case errors.Is(err, projectusecase.ErrProjectMemberUserNotFound):
		WriteError(w, http.StatusNotFound, spec, NewErrDetail("user_id", "not_found", "User was not found"))
	case errors.Is(err, projectusecase.ErrProjectMemberInvalidInput):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("", "invalid_input", "Project, member, and role IDs are required"))
	case errors.Is(err, projectusecase.ErrPermissionDenied):
		WriteError(w, http.StatusForbidden, spec, NewErrDetail("", "permission_denied", "The caller lacks required project member permission"))
	default:
		WriteError(w, http.StatusInternalServerError, spec, ErrDetailInternalServerError)
	}
}
