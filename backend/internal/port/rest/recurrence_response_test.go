package rest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	scheduledao "github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
)

func TestTodoItemResponseUsesStableVirtualIDAndRepeatMetadata(t *testing.T) {
	anchorDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	response := todoItemResponse(dao.TodoItem{
		ID: "generated-1", TaskID: "task-1", SeriesID: "root-1", OccurrenceDate: "2026-10-12",
		RepeatState: "active", FrequencyAnchorDate: anchorDate.Unix(), IntervalWeeks: 2,
	})
	if response.ID != virtualOccurrenceID || response.SeriesID != "root-1" || response.OccurrenceDate != "2026-10-12" {
		t.Fatalf("virtual response identity = id %q root %q date %q", response.ID, response.SeriesID, response.OccurrenceDate)
	}
	if response.RepeatState != "active" || response.FrequencyAnchorDate == nil || *response.FrequencyAnchorDate != "2026-10-05" {
		t.Fatalf("virtual response repeat metadata = state %q anchor %v", response.RepeatState, response.FrequencyAnchorDate)
	}
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"repeat_state":"active"`) {
		t.Fatalf("repeat-state JSON field = %s", body)
	}
}

func TestScheduleResponseUsesStableVirtualIDAndRepeatMetadata(t *testing.T) {
	anchorDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	response := scheduleResponse(scheduledao.Schedule{
		ID: "generated-1", UserID: "owner-1", AssigneeID: "assignee-1", SeriesID: "root-1", OccurrenceDate: "2026-10-12",
		RepeatState: "active", FrequencyAnchorDate: anchorDate.Unix(), IntervalWeeks: 2, Timezone: "UTC",
	})
	if response.ID != virtualOccurrenceID || response.SeriesID != "root-1" || response.OccurrenceDate != "2026-10-12" {
		t.Fatalf("virtual response identity = id %q root %q date %q", response.ID, response.SeriesID, response.OccurrenceDate)
	}
	if response.RepeatState != "active" || response.FrequencyAnchorDate == nil || *response.FrequencyAnchorDate != "2026-10-05" {
		t.Fatalf("virtual response repeat metadata = state %q anchor %v", response.RepeatState, response.FrequencyAnchorDate)
	}
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"repeat_state":"active"`) {
		t.Fatalf("repeat-state JSON field = %s", body)
	}
	if response.UserID != "owner-1" || response.AssigneeID != "assignee-1" {
		t.Fatalf("schedule ownership/assignment = %q/%q", response.UserID, response.AssigneeID)
	}
}
