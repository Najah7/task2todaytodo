package rest

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
)

func TestScheduleBlueprintRequestPreservesLocalWallTimes(t *testing.T) {
	request := ScheduleBlueprintCreateRequest{
		Title: "Daily review", Timezone: "Asia/Tokyo", LocalStart: "2026-10-11T09:30:00",
		LocalEnd: "2026-10-11T10:15:00", Frequency: "daily", Interval: 1,
	}
	input, err := request.input()
	if err != nil {
		t.Fatal(err)
	}
	if input.LocalStart.Format("2006-01-02T15:04:05") != request.LocalStart || input.LocalEnd.Format("2006-01-02T15:04:05") != request.LocalEnd {
		t.Fatalf("local datetimes changed: start=%s end=%s", input.LocalStart, input.LocalEnd)
	}
}

func TestScheduleRequestsCarryAllDayToggle(t *testing.T) {
	create, err := (ScheduleBlueprintCreateRequest{
		Title: "Holiday", Timezone: "UTC", LocalStart: "2026-12-24T00:00:00",
		LocalEnd: "2026-12-27T00:00:00", AllDay: true, Frequency: "once",
	}).input()
	if err != nil || !create.AllDay {
		t.Fatalf("create input all_day=%v err=%v", create.AllDay, err)
	}
	update := ScheduleBlueprintUpdateRequest{AllDay: optionalJSON[bool]{Present: true, Value: ptr(true)}}
	blueprintPatch, err := update.input()
	if err != nil || blueprintPatch.AllDay == nil || !*blueprintPatch.AllDay {
		t.Fatalf("blueprint update all_day=%v err=%v", blueprintPatch.AllDay, err)
	}
	occurrencePatch := ScheduleOccurrenceUpdateRequest{AllDay: optionalJSON[bool]{Present: true, Value: ptr(true)}}
	occurrence, err := occurrencePatch.input()
	if err != nil || occurrence.AllDay == nil || !*occurrence.AllDay {
		t.Fatalf("occurrence update all_day=%v err=%v", occurrence.AllDay, err)
	}
}

func TestScheduleBlueprintRequestReportsInvalidField(t *testing.T) {
	request := ScheduleBlueprintCreateRequest{LocalStart: "2026-10-11T09:30:00", LocalEnd: "invalid", EndDate: ptr("not-a-date")}
	_, err := request.input()
	var fieldError scheduleRequestFieldError
	if !errors.As(err, &fieldError) || fieldError.field != "local_end" {
		t.Fatalf("input error=%v; want field local_end", err)
	}
	request.LocalEnd = "2026-10-11T10:00:00"
	_, err = request.input()
	if !errors.As(err, &fieldError) || fieldError.field != "end_date" {
		t.Fatalf("input error=%v; want field end_date", err)
	}
	request.EndDate = nil
	request.Weekdays = []string{"daily"}
	_, err = request.input()
	if !errors.As(err, &fieldError) || fieldError.field != "weekdays" {
		t.Fatalf("input error=%v; want field weekdays", err)
	}
}

func TestScheduleBlueprintResponseUsesRoundTrippableLocalTimestamp(t *testing.T) {
	cutoff := "2026-10-12"
	response := scheduleBlueprintResponse(dao.Blueprint{
		LocalStart: "2026-10-11 09:30:00.123456", LocalEnd: "2026-10-11 10:15:00.123456", AllDay: true,
		CutoffAfter: &cutoff,
	})
	if response.LocalStart != "2026-10-11T09:30:00.123456" || response.LocalEnd != "2026-10-11T10:15:00.123456" {
		t.Fatalf("response local timestamps = %q, %q", response.LocalStart, response.LocalEnd)
	}
	if response.CutoffAfter == nil || *response.CutoffAfter != cutoff {
		t.Fatalf("response cutoff_after=%v, want %q", response.CutoffAfter, cutoff)
	}
	if !response.AllDay {
		t.Fatal("response omitted all_day=true")
	}
	parsed, err := parseBlueprintLocal(response.LocalStart)
	if err != nil || parsed.Hour() != 9 || parsed.Minute() != 30 {
		t.Fatalf("round-trip parse = %v, %v", parsed, err)
	}
}

func TestScheduleOccurrenceResponseRepresentsVirtualIdentityWithoutMagicID(t *testing.T) {
	row := dao.ScheduleOccurrence{
		BlueprintID: "blueprint-1", OccurrenceDate: "2026-10-11", Timezone: "Asia/Tokyo",
		StartAt: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC).Unix(),
		EndAt:   time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC).Unix(),
	}
	response := scheduleOccurrenceResponse(row)
	if response.ID != nil || response.Persisted {
		t.Fatalf("virtual identity = id %v persisted %v", response.ID, response.Persisted)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"id"`) {
		t.Fatalf("virtual JSON contains a fabricated ID: %s", encoded)
	}
	if response.StartAt != "2026-10-11T00:00:00Z" {
		t.Fatalf("start_at=%q; want UTC instant", response.StartAt)
	}
}

func ptr[T any](value T) *T { return &value }
