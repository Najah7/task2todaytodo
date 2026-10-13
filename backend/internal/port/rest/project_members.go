package rest

import (
	"errors"
	"net/http"

	authdao "github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	projectdomain "github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

type ProjectMemberHandler struct {
	members    *projectusecase.UseCases
	pageTokens *pagination.Codec
	issueID    func() string
	roleList   *authusecase.ListRolesUseCase
}

func NewProjectMemberHandler(members *projectusecase.UseCases, options ...any) *ProjectMemberHandler {
	h := &ProjectMemberHandler{members: members}
	for _, option := range options {
		switch value := option.(type) {
		case *pagination.Codec:
			h.pageTokens = value
		case func() string:
			h.issueID = value
		case *authusecase.ListRolesUseCase:
			h.roleList = value
		}
	}
	return h
}

type ProjectMemberResponse struct {
	ProjectID    string `json:"project_id"`
	UserID       string `json:"user_id"`
	RoleID       string `json:"role_id"`
	RoleName     string `json:"role_name"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Email        string `json:"email"`
	AddedBy      string `json:"added_by"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	Status       string `json:"status" enums:"owner,accepted,pending,expired"`
	InvitationID string `json:"invitation_id,omitempty"`
	ExpiresAt    *int64 `json:"expires_at" extensions:"x-nullable"`
	CanEditRole  bool   `json:"can_edit_role"`
	CanRemove    bool   `json:"can_remove"`
}

type ProjectMemberListResponse struct {
	Items                  []ProjectMemberResponse           `json:"items"`
	TotalCount             int64                             `json:"total_count"`
	MemberCount            int64                             `json:"member_count"`
	PendingInvitationCount int64                             `json:"pending_invitation_count"`
	NextPageToken          string                            `json:"next_page_token"`
	PreviousPageToken      string                            `json:"previous_page_token"`
	CanInvite              bool                              `json:"can_invite"`
	AssignableRoles        []ProjectMemberRoleOptionResponse `json:"assignable_roles"`
}

type ProjectMemberRoleOptionResponse struct {
	ID         string `json:"id" enums:"admin,editor,viewer"`
	Name       string `json:"name"`
	Assignable bool   `json:"assignable"`
}

type ProjectMemberRoleCapabilitiesResponse struct {
	Roles        []ProjectMemberRoleOptionResponse     `json:"roles"`
	Capabilities []ProjectMemberRoleCapabilityResponse `json:"capabilities"`
}

type ProjectMemberRoleCapabilityResponse struct {
	ID     string `json:"id" enums:"project.read,project.update,project.delete,task.read,task.create,task.update,task.delete,action_item.read,action_item.create,action_item.update,action_item.delete,schedule.read,schedule.create,schedule.update,schedule.delete,task_assignment.update,schedule_assignment.update,occurrence.update,project_member.read,project_member.invite,project_member.manage_basic,project_member.manage_admin"`
	Name   string `json:"name"`
	Owner  bool   `json:"owner"`
	Admin  bool   `json:"admin"`
	Editor bool   `json:"editor"`
	Viewer bool   `json:"viewer"`
}

type ProjectInvitationCreateRequest struct {
	Email  string `json:"email" validate:"required"`
	RoleID string `json:"role_id" validate:"required" enums:"admin,editor,viewer"`
}

type ProjectInvitationResponse struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Email     string `json:"email"`
	RoleID    string `json:"role_id" enums:"admin,editor,viewer"`
	Status    string `json:"status" enums:"pending"`
	ExpiresAt int64  `json:"expires_at"`
}

type ProjectInvitationPreviewRequest struct {
	Token string `json:"token" validate:"required"`
}

type ProjectInvitationPreviewResponse struct {
	ProjectID       string `json:"project_id"`
	ProjectTitle    string `json:"project_title"`
	Email           string `json:"email"`
	RoleID          string `json:"role_id" enums:"admin,editor,viewer"`
	RoleName        string `json:"role_name"`
	Status          string `json:"status" enums:"pending,expired,accepted,revoked,project_deleted"`
	ExpiresAt       int64  `json:"expires_at"`
	RecipientStatus string `json:"recipient_status" enums:"new,unverified,verified"`
}

type ProjectMemberUpsertRequest struct {
	RoleID string `json:"role_id"`
}

var (
	projectMemberListFailure        = NewFailureErrSpec(ResourceProjects, "list_members", "Failed to list project members")
	projectMemberUpsertFailure      = NewFailureErrSpec(ResourceProjects, "upsert_member", "Failed to update project member")
	projectMemberDeleteFailure      = NewFailureErrSpec(ResourceProjects, "delete_member", "Failed to delete project member")
	projectInvitationCreateFailure  = NewFailureErrSpec(ResourceProjects, "invite_member", "Failed to invite project member")
	projectInvitationResendFailure  = NewFailureErrSpec(ResourceProjects, "resend_invitation", "Failed to resend project invitation")
	projectInvitationRevokeFailure  = NewFailureErrSpec(ResourceProjects, "revoke_invitation", "Failed to revoke project invitation")
	projectInvitationPreviewFailure = NewFailureErrSpec(ResourceProjects, "preview_invitation", "Failed to preview project invitation")
)

// ListMembers returns owner, members, and invitations the caller may view.
//
//	@Summary	List project members
//	@Tags		Projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Project ID"
//	@Param		page_size	query		int		false	"Items per page (default 50, maximum 100)"
//	@Param		page_token	query		string	false	"Opaque cursor for the next page"
//	@Param		fields		query		string	false	"Response field mask"
//	@Success	200			{object}	ProjectMemberListResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	403			{object}	ErrResponse
//	@Failure	404			{object}	ErrResponse
//	@Failure	500			{object}	ErrResponse
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
	request, err := parseListRequest(r, h.pageTokens, string(actor), "project_members", string(projectID), "created_at_desc_sort_id_desc", ProjectMemberListResponse{})
	if err != nil {
		writeListError(w, projectMemberListFailure, err)
		return
	}
	var anchor *projectusecase.CursorAnchor
	if request.Anchor != nil {
		anchor = &projectusecase.CursorAnchor{At: request.Anchor.At, ID: request.Anchor.ID, Direction: request.Anchor.Direction}
	}
	page, err := h.members.Members.ListMembers.Execute(r.Context(), actor, projectID, projectusecase.ProjectRosterPageRequest{Size: request.Size, Anchor: anchor})
	if err != nil {
		writeProjectMemberUseCaseError(w, projectMemberListFailure, err)
		return
	}
	items := make([]ProjectMemberResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, member := range page.Items {
		items = append(items, projectMemberResponse(member))
		anchors = append(anchors, listAnchor{At: member.CursorCreatedAt, ID: member.CursorID})
	}
	response := ProjectMemberListResponse{
		Items: items, TotalCount: page.TotalCount, MemberCount: page.MemberCount,
		PendingInvitationCount: page.PendingInvitationCount, CanInvite: page.CanInvite,
		AssignableRoles: projectMemberRoleOptions(page.AssignableRoles),
	}
	if page.Next != nil {
		response.NextPageToken, err = pagination.Encode(h.pageTokens, request.Scope, listAnchor{At: page.Next.At, ID: page.Next.ID, Direction: page.Next.Direction})
	}
	if err == nil && page.Previous != nil {
		response.PreviousPageToken, err = pagination.Encode(h.pageTokens, request.Scope, listAnchor{At: page.Previous.At, ID: page.Previous.ID, Direction: page.Previous.Direction})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, projectMemberListFailure, ErrDetailInternalServerError)
			return
		}
	}
	encoded, err := request.Mask.Project(response)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, projectMemberListFailure, ErrDetailInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

// RoleCapabilities returns the backend-owned project role permission matrix.
//
//	@Summary	List project role capabilities
//	@Tags		Projects
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	ProjectMemberRoleCapabilitiesResponse
//	@Failure	401	{object}	ErrResponse
//	@Router		/roles/capabilities [get]
func (h *ProjectMemberHandler) RoleCapabilities(w http.ResponseWriter, r *http.Request) {
	if _, ok := projectUserID(r.Context()); !ok {
		WriteError(w, http.StatusUnauthorized, projectMemberListFailure, ErrDetailUnauthorized)
		return
	}
	var catalog []authdao.Role
	if h.roleList != nil {
		var err error
		catalog, err = h.roleList.Execute(r.Context())
		if err != nil {
			WriteError(w, http.StatusInternalServerError, projectMemberListFailure, ErrDetailInternalServerError)
			return
		}
	}
	roles, capabilities := projectusecase.ProjectMemberRoleCapabilities(catalog)
	response := ProjectMemberRoleCapabilitiesResponse{
		Roles:        projectMemberRoleOptions(roles),
		Capabilities: make([]ProjectMemberRoleCapabilityResponse, 0, len(capabilities)),
	}
	for _, capability := range capabilities {
		response.Capabilities = append(response.Capabilities, ProjectMemberRoleCapabilityResponse{
			ID: capability.ID, Name: capability.Name, Owner: capability.Owner, Admin: capability.Admin,
			Editor: capability.Editor, Viewer: capability.Viewer,
		})
	}
	WriteJSON(w, http.StatusOK, response)
}

// UpsertMember changes the role of an existing project member.
//
//	@Summary	Update a project member role
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
//	@Router		/projects/{id}/members/{user_id} [patch]
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

// CreateInvitation sends a project invitation to one email address.
//
//	@Summary	Invite a project member
//	@Tags		Projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string							true	"Project ID"
//	@Param		request	body		ProjectInvitationCreateRequest	true	"Invitation"
//	@Success	202		{object}	ProjectInvitationResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	409		{object}	ErrResponse
//	@Failure	429		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/projects/{id}/invitations [post]
func (h *ProjectMemberHandler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	actor, ok := projectUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, projectInvitationCreateFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectInvitationCreateFailure)
	if !ok {
		return
	}
	var request ProjectInvitationCreateRequest
	if !decodeProjectJSON(w, r, &request) || request.Email == "" || request.RoleID == "" {
		WriteError(w, http.StatusBadRequest, projectInvitationCreateFailure, ErrDetailInvalidRequestBody)
		return
	}
	if h.issueID == nil {
		WriteError(w, http.StatusInternalServerError, projectInvitationCreateFailure, ErrDetailInternalServerError)
		return
	}
	invitation, err := h.members.Invitations.Create.Execute(r.Context(), actor, projectID, h.issueID(), request.Email, request.RoleID)
	if err != nil {
		writeProjectInvitationUseCaseError(w, projectInvitationCreateFailure, err)
		return
	}
	WriteJSON(w, http.StatusAccepted, projectInvitationResponse(invitation))
}

// ResendInvitation invalidates previous links and sends a renewed invitation.
//
//	@Summary	Resend project invitation
//	@Tags		Projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id				path	string	true	"Project ID"
//	@Param		invitation_id	path	string	true	"Invitation ID"
//	@Success	202
//	@Failure	401	{object}	ErrResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	429	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/projects/{id}/invitations/{invitation_id}/resend [post]
func (h *ProjectMemberHandler) ResendInvitation(w http.ResponseWriter, r *http.Request) {
	actor, ok := projectUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, projectInvitationResendFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectInvitationResendFailure)
	if !ok {
		return
	}
	id := r.PathValue("invitation_id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, projectInvitationResendFailure, NewErrDetail("invitation_id", "required", "Invitation ID is required"))
		return
	}
	invitation, err := h.members.Invitations.Resend.Execute(r.Context(), actor, projectID, id)
	if err != nil {
		writeProjectInvitationUseCaseError(w, projectInvitationResendFailure, err)
		return
	}
	WriteJSON(w, http.StatusAccepted, projectInvitationResponse(invitation))
}

// RevokeInvitation revokes a project invitation.
//
//	@Summary	Revoke project invitation
//	@Tags		Projects
//	@Security	BearerAuth
//	@Param		id				path	string	true	"Project ID"
//	@Param		invitation_id	path	string	true	"Invitation ID"
//	@Success	204
//	@Failure	401	{object}	ErrResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/projects/{id}/invitations/{invitation_id} [delete]
func (h *ProjectMemberHandler) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	actor, ok := projectUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, projectInvitationRevokeFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectInvitationRevokeFailure)
	if !ok {
		return
	}
	id := r.PathValue("invitation_id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, projectInvitationRevokeFailure, NewErrDetail("invitation_id", "required", "Invitation ID is required"))
		return
	}
	if err := h.members.Invitations.Revoke.Execute(r.Context(), actor, projectID, id); err != nil {
		writeProjectInvitationUseCaseError(w, projectInvitationRevokeFailure, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PreviewInvitation reads invitation details without consuming the token.
//
//	@Summary	Preview project invitation
//	@Tags		Project Invitations
//	@Accept		json
//	@Produce	json
//	@Param		request	body		ProjectInvitationPreviewRequest	true	"Invitation token"
//	@Success	200		{object}	ProjectInvitationPreviewResponse
//	@Failure	400		{object}	ErrResponse
//	@Router		/invitations/preview [post]
func (h *ProjectMemberHandler) PreviewInvitation(w http.ResponseWriter, r *http.Request) {
	setAuthResponseHeaders(w)
	var request ProjectInvitationPreviewRequest
	if !decodeProjectJSON(w, r, &request) || request.Token == "" {
		WriteError(w, http.StatusBadRequest, projectInvitationPreviewFailure, ErrDetailInvalidRequestBody)
		return
	}
	preview, err := h.members.Invitations.Preview.Execute(r.Context(), request.Token)
	if err != nil {
		writeProjectInvitationUseCaseError(w, projectInvitationPreviewFailure, err)
		return
	}
	invitation := preview.Invitation
	WriteJSON(w, http.StatusOK, ProjectInvitationPreviewResponse{
		ProjectID: invitation.ProjectID, ProjectTitle: invitation.ProjectTitle, Email: invitation.Email,
		RoleID: invitation.RoleID, RoleName: invitationRoleName(invitation.RoleID), Status: preview.Status,
		ExpiresAt: invitation.ExpiresAt.Unix(), RecipientStatus: invitation.RecipientStatus,
	})
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
	var expiresAt *int64
	if member.ExpiresAt != 0 {
		value := member.ExpiresAt
		expiresAt = &value
	}
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
		UpdatedAt: member.UpdatedAt, Status: member.Status, InvitationID: member.InvitationID,
		ExpiresAt: expiresAt, CanEditRole: member.CanEditRole, CanRemove: member.CanRemove,
	}
}

func projectMemberRoleOptions(roles []projectusecase.ProjectMemberRoleOption) []ProjectMemberRoleOptionResponse {
	items := make([]ProjectMemberRoleOptionResponse, 0, len(roles))
	for _, role := range roles {
		items = append(items, ProjectMemberRoleOptionResponse{ID: role.ID, Name: role.Name, Assignable: role.Assignable})
	}
	return items
}

func projectInvitationResponse(invitation dao.ProjectInvitation) ProjectInvitationResponse {
	return ProjectInvitationResponse{
		ID: invitation.ID, ProjectID: invitation.ProjectID, Email: invitation.Email,
		RoleID: invitation.RoleID, Status: "pending", ExpiresAt: invitation.ExpiresAt.Unix(),
	}
}

func invitationRoleName(role string) string {
	switch role {
	case "admin":
		return "Administrator"
	case "editor":
		return "Editor"
	case "viewer":
		return "Viewer"
	default:
		return role
	}
}

func writeProjectInvitationUseCaseError(w http.ResponseWriter, spec ErrSpec, err error) {
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound), errors.Is(err, projectusecase.ErrProjectInvitationNotFound):
		WriteError(w, http.StatusNotFound, spec, NewErrDetail("invitation_id", "not_found", "Project or invitation was not found"))
	case errors.Is(err, projectusecase.ErrTeamSubscriptionRequired):
		WriteError(w, http.StatusForbidden, spec, NewErrDetail("plan", "team_required", "A Team subscription is required for shared Project collaboration"))
	case errors.Is(err, projectusecase.ErrPermissionDenied), errors.Is(err, projectusecase.ErrProjectInvitationRejected):
		WriteError(w, http.StatusForbidden, spec, NewErrDetail("", "permission_denied", "The caller cannot manage this invitation"))
	case errors.Is(err, projectusecase.ErrProjectInvitationRateLimited):
		WriteError(w, http.StatusTooManyRequests, spec, NewErrDetail("email", "rate_limited", "Please wait before sending another invitation"))
	case errors.Is(err, projectusecase.ErrProjectInvitationInvalidEmail):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("email", "invalid", "Enter a valid email address"))
	case errors.Is(err, projectusecase.ErrProjectInvitationTokenInvalid):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("token", "invalid", "Invitation token is invalid or expired"))
	case errors.Is(err, projectusecase.ErrProjectInvitationInvalidInput):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("", "invalid_input", "Invitation details are invalid"))
	case errors.Is(err, projectusecase.ErrProjectInvitationConflict):
		WriteError(w, http.StatusConflict, spec, NewErrDetail("email", "invitation_conflict", "This address is already a member or has a pending invitation"))
	default:
		WriteError(w, http.StatusInternalServerError, spec, ErrDetailInternalServerError)
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
	case errors.Is(err, projectusecase.ErrTeamSubscriptionRequired):
		WriteError(w, http.StatusForbidden, spec, NewErrDetail("plan", "team_required", "A Team subscription is required for shared Project collaboration"))
	case errors.Is(err, projectusecase.ErrPermissionDenied):
		WriteError(w, http.StatusForbidden, spec, NewErrDetail("", "permission_denied", "The caller lacks required project member permission"))
	default:
		WriteError(w, http.StatusInternalServerError, spec, ErrDetailInternalServerError)
	}
}
