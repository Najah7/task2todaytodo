package rest

import (
	"net/http"
	"strconv"
	"strings"
)

const ifMatchHeader = "If-Match"

func expectedRevision(w http.ResponseWriter, r *http.Request, spec ErrSpec) (int32, bool) {
	return parseExpectedRevision(w, r, spec, false)
}

func expectedOccurrenceRevision(w http.ResponseWriter, r *http.Request, spec ErrSpec) (int32, bool) {
	return parseExpectedRevision(w, r, spec, true)
}

func expectedCollectionRevision(w http.ResponseWriter, r *http.Request, spec ErrSpec) (string, bool) {
	values := r.Header.Values(ifMatchHeader)
	if len(values) == 0 {
		WriteError(w, http.StatusPreconditionRequired, spec, NewErrDetail("If-Match", "revision_required", "If-Match must contain the current collection revision"))
		return "", false
	}
	if len(values) != 1 {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("If-Match", "invalid_revision", "If-Match must contain one quoted SHA-256 collection revision"))
		return "", false
	}
	value := strings.TrimSpace(values[0])
	if len(value) != 66 || value[0] != '"' || value[len(value)-1] != '"' {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("If-Match", "invalid_revision", "If-Match must contain one quoted SHA-256 collection revision"))
		return "", false
	}
	token := value[1 : len(value)-1]
	for _, c := range token {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			WriteError(w, http.StatusBadRequest, spec, NewErrDetail("If-Match", "invalid_revision", "If-Match must contain one quoted SHA-256 collection revision"))
			return "", false
		}
	}
	return token, true
}

func parseExpectedRevision(w http.ResponseWriter, r *http.Request, spec ErrSpec, allowZero bool) (int32, bool) {
	values := r.Header.Values(ifMatchHeader)
	if len(values) == 0 {
		WriteError(w, http.StatusPreconditionRequired, spec, NewErrDetail("If-Match", "revision_required", "If-Match must contain the current resource revision"))
		return 0, false
	}
	minimum := int64(1)
	qualifier := "positive"
	if allowZero {
		minimum = 0
		qualifier = "nonnegative"
	}
	if len(values) != 1 {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("If-Match", "invalid_revision", "If-Match must contain one quoted "+qualifier+" revision"))
		return 0, false
	}
	value := strings.TrimSpace(values[0])
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("If-Match", "invalid_revision", "If-Match must contain one quoted "+qualifier+" revision"))
		return 0, false
	}
	parsed, err := strconv.ParseInt(value[1:len(value)-1], 10, 32)
	if err != nil || parsed < minimum {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("If-Match", "invalid_revision", "If-Match must contain one quoted "+qualifier+" revision"))
		return 0, false
	}
	return int32(parsed), true
}

func setResourceETag(w http.ResponseWriter, revision int32) {
	w.Header().Set("ETag", `"`+strconv.Itoa(int(revision))+`"`)
}
