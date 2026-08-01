package handlers

import (
	"encoding/json"
	"net/http"
)

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
