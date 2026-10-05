package rest

import (
	"errors"
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/port/rest/fieldmask"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

type listAnchor struct {
	At             string `json:"at,omitempty"`
	ID             string `json:"id,omitempty"`
	Revision       int32  `json:"revision,omitempty"`
	Name           string `json:"name,omitempty"`
	Position       int    `json:"position,omitempty"`
	SeriesID       string `json:"series_id,omitempty"`
	OccurrenceDate string `json:"occurrence_date,omitempty"`
	AsOf           string `json:"as_of,omitempty"`
}

type listEnvelope[T any] struct {
	Items         []T    `json:"items"`
	NextPageToken string `json:"next_page_token"`
}

type listRequest struct {
	Size     int
	Mask     *fieldmask.Mask
	Anchor   *listAnchor
	Scope    pagination.Scope
	FromDate string
	AsOf     time.Time
}

var errInvalidListRequest = errors.New("invalid list request")

type invalidListRequestError struct {
	field string
}

func (e invalidListRequestError) Error() string { return errInvalidListRequest.Error() }

func invalidListField(err error) string {
	var requestErr invalidListRequestError
	if errors.As(err, &requestErr) {
		return requestErr.field
	}
	return "page_token"
}

func parseListRequest(r *http.Request, codec *pagination.Codec, userID, list, parent, order string, schema any) (listRequest, error) {
	return parseListRequestWithFromDate(r, codec, userID, list, parent, order, schema, false)
}

func parseListRequestWithFromDate(r *http.Request, codec *pagination.Codec, userID, list, parent, order string, schema any, allowFromDate bool) (listRequest, error) {
	q := r.URL.Query()
	queryKeys := []string{"page_size", "page_token", "fields"}
	if allowFromDate {
		queryKeys = append(queryKeys, "from_date")
	}
	for _, key := range queryKeys {
		if len(q[key]) > 1 {
			return listRequest{}, invalidListRequestError{field: key}
		}
	}
	if _, ok := q["limit"]; ok {
		return listRequest{}, invalidListRequestError{field: "limit"}
	}
	if _, ok := q["offset"]; ok {
		return listRequest{}, invalidListRequestError{field: "offset"}
	}
	size, err := pagination.ParsePageSize(q.Get("page_size"), q.Has("page_size"))
	if err != nil {
		return listRequest{}, invalidListRequestError{field: "page_size"}
	}
	mask, err := fieldmask.ParseOptional(q.Get("fields"), q.Has("fields"), schema)
	if err != nil {
		return listRequest{}, invalidListRequestError{field: "fields"}
	}
	fromDate := ""
	if q.Has("from_date") {
		if !allowFromDate {
			return listRequest{}, invalidListRequestError{field: "from_date"}
		}
		parsed, err := parseDate(q.Get("from_date"))
		if err != nil {
			return listRequest{}, invalidListRequestError{field: "from_date"}
		}
		fromDate = parsed.Format("2006-01-02")
	}
	scopedOrder := order
	if allowFromDate {
		scopedOrder += "|from_date=" + fromDate
	}
	scope := pagination.Scope{UserID: userID, List: list, Parent: parent, Order: scopedOrder, Fields: mask.Canonical()}
	request := listRequest{Size: size, Mask: mask, Scope: scope, FromDate: fromDate}
	if q.Has("page_token") {
		anchor, err := pagination.Decode[listAnchor](codec, q.Get("page_token"), scope)
		if err != nil {
			return listRequest{}, invalidListRequestError{field: "page_token"}
		}
		request.Anchor = &anchor
		if anchor.AsOf != "" {
			request.AsOf, err = time.Parse(time.RFC3339Nano, anchor.AsOf)
			if err != nil {
				return listRequest{}, invalidListRequestError{field: "page_token"}
			}
		}
	}
	return request, nil
}

func writeListError(w http.ResponseWriter, spec ErrSpec, err error) {
	WriteError(w, http.StatusBadRequest, spec, NewErrDetail(invalidListField(err), "invalid_list_request", "List pagination or fields are invalid"))
}

func writeListResponse[T any](w http.ResponseWriter, items []T, anchors []listAnchor, hasMore bool, request listRequest, codec *pagination.Codec, failure ErrSpec) {
	if items == nil {
		items = []T{}
	}
	response := listEnvelope[T]{Items: items, NextPageToken: ""}
	if hasMore && len(items) != 0 {
		token, err := pagination.Encode(codec, request.Scope, anchors[len(items)-1])
		if err != nil {
			WriteError(w, http.StatusInternalServerError, failure, ErrDetailInternalServerError)
			return
		}
		response.NextPageToken = token
	}
	encoded, err := request.Mask.Project(response)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, failure, ErrDetailInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}
