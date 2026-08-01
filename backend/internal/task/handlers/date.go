package handlers

import (
	"encoding/json"
	sharedhandlers "github.com/Najah7/task2todaytodo/internal/shared/handlers"
	"time"
)

const dateOnlyLayout = "2006-01-02"

func parseOptionalDateOnly(value *string) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}
	return time.Parse(dateOnlyLayout, *value)
}

func parseDateOnlyPatch(value *string, set bool) (*time.Time, error) {
	if !set {
		return nil, nil
	}
	if value == nil {
		zero := time.Time{}
		return &zero, nil
	}
	dueDate, err := time.Parse(dateOnlyLayout, *value)
	if err != nil {
		return nil, err
	}
	return &dueDate, nil
}

func dateOnlyResponse(value time.Time) *string {
	if value.IsZero() {
		return nil
	}
	formatted := value.Format(dateOnlyLayout)
	return &formatted
}

func decodeNullableDateOnly(raw map[string]json.RawMessage, field string) (*string, bool, error) {
	value, ok := raw[field]
	if !ok {
		return nil, false, nil
	}
	if string(value) == "null" {
		return nil, true, nil
	}
	var date string
	if err := json.Unmarshal(value, &date); err != nil {
		return nil, true, err
	}
	return &date, true, nil
}

func errDetailInvalidDateOnly(field string) sharedhandlers.ErrDetail {
	return sharedhandlers.NewErrDetail(field, "invalid_date", "Date must use YYYY-MM-DD format")
}
