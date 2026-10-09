package rest

const (
	ResourcePersonalAccessTokens = "personal_access_tokens"
	ResourceAuth                 = "auth"
	ResourceProjects             = "projects"
	ResourceProjectTypes         = "project_types"
	ResourceResponses            = "responses"
	ResourceTaskFrequencies      = "task_frequencies"
	ResourceTasks                = "tasks"
	ResourceActionItems          = "action_items"
	ResourceTaskPriorities       = "task_priorities"
	ResourceTaskStatuses         = "task_statuses"
	ResourceUsers                = "users"
	ResourceRoles                = "roles"
	ResourcePermissions          = "permissions"

	ActionAuthenticate              = "authenticate"
	ActionCreate                    = "create"
	ActionGenerate                  = "generate"
	ActionGet                       = "get"
	ActionList                      = "list"
	ActionMarshal                   = "marshal"
	ActionRevoke                    = "revoke"
	ActionUpdate                    = "update"
	ActionDelete                    = "delete"
	ActionUpdateBasicInfo           = "update_basic_info"
	ActionUpdateTimezone            = "update_timezone"
	ActionUpdatePassword            = "update_password"
	ResultFailed                    = "failed"
	DetailInvalidBodyCode           = "invalid_request_body"
	DetailInvalidBodyMsg            = "Invalid request body"
	DetailInvalidCredCode           = "invalid_credentials"
	DetailInvalidCredMsg            = "Invalid email or password"
	DetailUnauthorizedCode          = "unauthorized"
	DetailUnauthorizedMsg           = "Unauthorized"
	DetailMissingTokenCode          = "missing_personal_access_token"
	DetailMissingTokenMsg           = "Missing access token"
	DetailInvalidTokenCode          = "invalid_personal_access_token"
	DetailInvalidTokenMsg           = "Invalid access token"
	DetailUserLookupCode            = "failed_to_get_user_by_email"
	DetailUserLookupMsg             = "Failed to get user by email"
	DetailMissingOrInvalidTokenCode = "missing_or_invalid_personal_access_token"
	DetailInternalErrorCode         = "internal_error"
	DetailInternalErrorMsg          = "An unexpected error occurred"
)

var (
	ErrSpecPersonalAccessTokensGenerateFailed = NewFailureErrSpec(ResourcePersonalAccessTokens, ActionGenerate, "Failed to generate access token")
	ErrSpecPersonalAccessTokensRevokeFailed   = NewFailureErrSpec(ResourcePersonalAccessTokens, ActionRevoke, "Failed to revoke access token")
	ErrSpecAuthAuthenticateFailed             = NewFailureErrSpec(ResourceAuth, ActionAuthenticate, "Failed to authenticate")
	ErrSpecProjectsCreateFailed               = NewFailureErrSpec(ResourceProjects, ActionCreate, "Failed to create project")
	ErrSpecProjectsGetFailed                  = NewFailureErrSpec(ResourceProjects, ActionGet, "Failed to get project")
	ErrSpecProjectsUpdateFailed               = NewFailureErrSpec(ResourceProjects, ActionUpdate, "Failed to update project")
	ErrSpecProjectsDeleteFailed               = NewFailureErrSpec(ResourceProjects, ActionDelete, "Failed to delete project")
	ErrSpecProjectTypesListFailed             = NewFailureErrSpec(ResourceProjectTypes, ActionList, "Failed to list project types")
	ErrSpecResponsesMarshalFailed             = NewFailureErrSpec(ResourceResponses, ActionMarshal, "Failed to marshal response")
	ErrSpecTaskFrequenciesListFailed          = NewFailureErrSpec(ResourceTaskFrequencies, ActionList, "Failed to list task frequencies")
	ErrSpecTasksCreateFailed                  = NewFailureErrSpec(ResourceTasks, ActionCreate, "Failed to create task")
	ErrSpecTasksGetFailed                     = NewFailureErrSpec(ResourceTasks, ActionGet, "Failed to get task")
	ErrSpecTasksUpdateFailed                  = NewFailureErrSpec(ResourceTasks, ActionUpdate, "Failed to update task")
	ErrSpecTasksDeleteFailed                  = NewFailureErrSpec(ResourceTasks, ActionDelete, "Failed to delete task")
	ErrSpecActionItemsCreateFailed            = NewFailureErrSpec(ResourceActionItems, ActionCreate, "Failed to create action item")
	ErrSpecActionItemsUpdateFailed            = NewFailureErrSpec(ResourceActionItems, ActionUpdate, "Failed to update action item")
	ErrSpecActionItemsDeleteFailed            = NewFailureErrSpec(ResourceActionItems, ActionDelete, "Failed to delete action item")
	ErrSpecTaskPrioritiesListFailed           = NewFailureErrSpec(ResourceTaskPriorities, ActionList, "Failed to list task priorities")
	ErrSpecTaskStatusesListFailed             = NewFailureErrSpec(ResourceTaskStatuses, ActionList, "Failed to list task statuses")
	ErrSpecUsersCreateFailed                  = NewFailureErrSpec(ResourceUsers, ActionCreate, "Failed to create user")
	ErrSpecUsersGetFailed                     = NewFailureErrSpec(ResourceUsers, ActionGet, "Failed to get user")
	ErrSpecUsersUpdateBasicInfoFailed         = NewFailureErrSpec(ResourceUsers, ActionUpdateBasicInfo, "Failed to update user")
	ErrSpecUsersUpdateTimezoneFailed          = NewFailureErrSpec(ResourceUsers, ActionUpdateTimezone, "Failed to update user timezone")
	ErrSpecUsersUpdatePasswordFailed          = NewFailureErrSpec(ResourceUsers, ActionUpdatePassword, "Failed to update password")

	ErrDetailInvalidRequestBody                  = NewErrDetail("", DetailInvalidBodyCode, DetailInvalidBodyMsg)
	ErrDetailInvalidCredentials                  = NewErrDetail("", DetailInvalidCredCode, DetailInvalidCredMsg)
	ErrDetailUnauthorized                        = NewErrDetail("", DetailUnauthorizedCode, DetailUnauthorizedMsg)
	ErrDetailMissingPersonalAccessToken          = NewErrDetail("", DetailMissingTokenCode, DetailMissingTokenMsg)
	ErrDetailInvalidPersonalAccessToken          = NewErrDetail("", DetailInvalidTokenCode, DetailInvalidTokenMsg)
	ErrDetailFailedUserLookup                    = NewErrDetail("", DetailUserLookupCode, DetailUserLookupMsg)
	ErrDetailMissingOrInvalidPersonalAccessToken = NewErrDetail("", DetailMissingTokenCode, DetailMissingTokenMsg)
	ErrDetailInternalServerError                 = NewErrDetail("", DetailInternalErrorCode, DetailInternalErrorMsg)
)

type ErrSpec struct {
	Code    string
	Message string
}

func NewFailureErrSpec(resource, action, message string) ErrSpec {
	return ErrSpec{
		Code:    resource + "::" + action + "::" + ResultFailed,
		Message: message,
	}
}

type ErrDetail struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

func NewErrDetail(field, code, message string) ErrDetail {
	return ErrDetail{
		Field:   field,
		Code:    code,
		Message: message,
	}
}

type Err struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Details   []ErrDetail `json:"details,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

type ErrResponse struct {
	Error Err `json:"error"`
}

func NewErrResponse(code, message string, details []ErrDetail, requestID string) *ErrResponse {
	return &ErrResponse{
		Error: Err{
			Code:      code,
			Message:   message,
			Details:   details,
			RequestID: requestID,
		},
	}
}
