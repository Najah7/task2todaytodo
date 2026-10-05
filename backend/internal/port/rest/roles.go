package rest

import (
	"net/http"

	authdao "github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

type RoleHandler struct{ roles authusecase.RoleUseCases }

func NewRoleHandler(roles authusecase.RoleUseCases) *RoleHandler { return &RoleHandler{roles: roles} }

type RoleResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Permissions []PermissionResponse `json:"permissions"`
}

type RoleListResponse struct {
	Items []RoleResponse `json:"items"`
}
type PermissionResponse struct {
	ID           int64  `json:"id"`
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	Action       string `json:"action"`
	Effect       string `json:"effect"`
	Description  string `json:"description"`
}
type PermissionListResponse struct {
	Items []PermissionResponse `json:"items"`
}

// List lists the global role catalog available for project membership.
//
//	@Summary	List roles
//	@Tags		Authorization
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	RoleListResponse
//	@Failure	401	{object}	ErrResponse
//	@Router		/roles [get]
func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.roles.List.Execute(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, NewFailureErrSpec(ResourceRoles, ActionList, "Failed to list roles"), ErrDetailInternalServerError)
		return
	}
	items := make([]RoleResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, roleResponse(row))
	}
	WriteJSON(w, http.StatusOK, RoleListResponse{Items: items})
}

// ListPermissions lists the read-only global permission catalog.
//
//	@Summary	List permissions
//	@Tags		Authorization
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	PermissionListResponse
//	@Failure	401	{object}	ErrResponse
//	@Router		/permissions [get]
func (h *RoleHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.roles.Permissions.Execute(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, NewFailureErrSpec(ResourcePermissions, ActionList, "Failed to list permissions"), ErrDetailInternalServerError)
		return
	}
	items := make([]PermissionResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, permissionResponse(row))
	}
	WriteJSON(w, http.StatusOK, PermissionListResponse{Items: items})
}

func roleResponse(row authdao.Role) RoleResponse {
	permissions := make([]PermissionResponse, 0, len(row.Permissions))
	for _, permission := range row.Permissions {
		permissions = append(permissions, permissionResponse(permission))
	}
	return RoleResponse{ID: row.ID, Name: row.Name, Permissions: permissions}
}

func permissionResponse(row authdao.Permission) PermissionResponse {
	return PermissionResponse{ID: row.ID, ResourceID: row.ResourceID, ResourceName: row.ResourceName, Action: row.Action, Effect: row.Effect, Description: row.Description}
}
