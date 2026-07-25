package handlers

import (
	"encoding/json"
	"net/http"
)

const (
	ResourceAccessTokens    = "access_tokens"
	ResourceAuth            = "auth"
	ResourceProjectTypes    = "project_types"
	ResourceResponses       = "responses"
	ResourceTaskFrequencies = "task_frequencies"
	ResourceTaskPriorities  = "task_priorities"
	ResourceTaskStatuses    = "task_statuses"
	ResourceUsers           = "users"

	ActionAuthenticate              = "authenticate"
	ActionCreate                    = "create"
	ActionGenerate                  = "generate"
	ActionGet                       = "get"
	ActionList                      = "list"
	ActionMarshal                   = "marshal"
	ActionRevoke                    = "revoke"
	ActionUpdateBasicInfo           = "update_basic_info"
	ActionUpdatePassword            = "update_password"
	ResultFailed                    = "failed"
	DetailInvalidBodyCode           = "invalid_request_body"
	DetailInvalidBodyMsg            = "Invalid request body"
	DetailInvalidCredCode           = "invalid_credentials"
	DetailInvalidCredMsg            = "Invalid email or password"
	DetailUnauthorizedCode          = "unauthorized"
	DetailUnauthorizedMsg           = "Unauthorized"
	DetailMissingTokenCode          = "missing_access_token"
	DetailMissingTokenMsg           = "Missing access token"
	DetailInvalidTokenCode          = "invalid_access_token"
	DetailInvalidTokenMsg           = "Invalid access token"
	DetailUserLookupCode            = "failed_to_get_user_by_email"
	DetailUserLookupMsg             = "Failed to get user by email"
	DetailMissingOrInvalidTokenCode = "missing_or_invalid_access_token"
	DetailInternalErrorCode         = "internal_error"
	DetailInternalErrorMsg          = "An unexpected error occurred"
)

var (
	ErrSpecAccessTokensGenerateFailed = NewFailureErrSpec(ResourceAccessTokens, ActionGenerate, "Failed to generate access token")
	ErrSpecAccessTokensRevokeFailed   = NewFailureErrSpec(ResourceAccessTokens, ActionRevoke, "Failed to revoke access token")
	ErrSpecAuthAuthenticateFailed     = NewFailureErrSpec(ResourceAuth, ActionAuthenticate, "Failed to authenticate")
	ErrSpecProjectTypesListFailed     = NewFailureErrSpec(ResourceProjectTypes, ActionList, "Failed to list project types")
	ErrSpecResponsesMarshalFailed     = NewFailureErrSpec(ResourceResponses, ActionMarshal, "Failed to marshal response")
	ErrSpecTaskFrequenciesListFailed  = NewFailureErrSpec(ResourceTaskFrequencies, ActionList, "Failed to list task frequencies")
	ErrSpecTaskPrioritiesListFailed   = NewFailureErrSpec(ResourceTaskPriorities, ActionList, "Failed to list task priorities")
	ErrSpecTaskStatusesListFailed     = NewFailureErrSpec(ResourceTaskStatuses, ActionList, "Failed to list task statuses")
	ErrSpecUsersCreateFailed          = NewFailureErrSpec(ResourceUsers, ActionCreate, "Failed to create user")
	ErrSpecUsersGetFailed             = NewFailureErrSpec(ResourceUsers, ActionGet, "Failed to get user")
	ErrSpecUsersUpdateBasicInfoFailed = NewFailureErrSpec(ResourceUsers, ActionUpdateBasicInfo, "Failed to update user")
	ErrSpecUsersUpdatePasswordFailed  = NewFailureErrSpec(ResourceUsers, ActionUpdatePassword, "Failed to update password")

	ErrDetailInvalidRequestBody          = NewErrDetail("", DetailInvalidBodyCode, DetailInvalidBodyMsg)
	ErrDetailInvalidCredentials          = NewErrDetail("", DetailInvalidCredCode, DetailInvalidCredMsg)
	ErrDetailUnauthorized                = NewErrDetail("", DetailUnauthorizedCode, DetailUnauthorizedMsg)
	ErrDetailMissingAccessToken          = NewErrDetail("", DetailMissingTokenCode, DetailMissingTokenMsg)
	ErrDetailInvalidAccessToken          = NewErrDetail("", DetailInvalidTokenCode, DetailInvalidTokenMsg)
	ErrDetailFailedUserLookup            = NewErrDetail("", DetailUserLookupCode, DetailUserLookupMsg)
	ErrDetailMissingOrInvalidAccessToken = NewErrDetail("", DetailMissingTokenCode, DetailMissingTokenMsg)
	ErrDetailInternalServerError         = NewErrDetail("", DetailInternalErrorCode, DetailInternalErrorMsg)
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

type MessageResponse struct {
	Message string `json:"message"`
}

func NewMessageResponse(message string) *MessageResponse {
	return &MessageResponse{
		Message: message,
	}
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	resp, err := json.Marshal(payload)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrSpecResponsesMarshalFailed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(resp)
}

func WriteMessage(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, NewMessageResponse(message))
}

func WriteError(w http.ResponseWriter, status int, spec ErrSpec, details ...ErrDetail) {
	if len(details) == 0 {
		details = []ErrDetail{ErrDetailInternalServerError}
	}

	WriteJSON(w, status, NewErrResponse(spec.Code, spec.Message, details, ""))
}
