package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewProject(t *testing.T) {
	projectType := mustProjectType(t, "work")
	priority := mustTaskPriority(t, "low")
	startDate := datePtr(2026, time.July, 18)
	endDate := datePtr(2026, time.July, 18)

	project, err := NewProject("project-1", "user-1", projectType, priority, "Build task domain", startDate, endDate)
	if err != nil {
		t.Fatalf("NewProject() error = %v", err)
	}

	if project.ID != "project-1" || project.UserID != "user-1" || project.Type != projectType || project.Priority != priority {
		t.Errorf("project = %+v, want IDs, type, and priority to be set", project)
	}
	if project.Goal != "" || project.Description != "" || project.Progress != 0 || project.Schedule.StartDate == nil || !project.Schedule.StartDate.Equal(*startDate) || project.Schedule.EndDate == nil || !project.Schedule.EndDate.Equal(*endDate) {
		t.Errorf("project = %+v, want only required values to be set", project)
	}
}

func TestNewProjectWithDetails(t *testing.T) {
	projectType := mustProjectType(t, "work")
	priority := mustTaskPriority(t, "high")
	startDate := datePtr(2026, time.July, 18)

	project, err := NewProjectWithDetails("project-1", "user-1", projectType, "Build task domain", "Ship it", "Entity layer", 10, priority, startDate, nil)
	if err != nil {
		t.Fatalf("NewProjectWithDetails() error = %v", err)
	}

	if project.Goal != "Ship it" || project.Description != "Entity layer" || project.Priority != priority || project.Schedule.StartDate == nil || !project.Schedule.StartDate.Equal(*startDate) || project.Schedule.EndDate != nil {
		t.Errorf("project = %+v, want details and optional start date to be set", project)
	}
}

func TestNewProjectValidation(t *testing.T) {
	projectType := mustProjectType(t, "work")
	priority := mustTaskPriority(t, "low")
	invalidType := ProjectType{Value: "fitness"}
	invalidPriority := TaskPriority{Value: "blocked"}
	start := datePtr(2026, time.July, 18)
	previous := datePtr(2026, time.July, 17)

	tests := []struct {
		name        string
		id          ProjectID
		userID      UserID
		projectType ProjectType
		priority    TaskPriority
		title       string
		startDate   *time.Time
		endDate     *time.Time
		wantErr     error
	}{
		{name: "empty ID", userID: "user-1", projectType: projectType, priority: priority, title: "Project", wantErr: ErrProjectIDEmpty},
		{name: "empty user ID", id: "project-1", projectType: projectType, priority: priority, title: "Project", wantErr: ErrProjectUserIDEmpty},
		{name: "invalid type", id: "project-1", userID: "user-1", projectType: invalidType, priority: priority, title: "Project", wantErr: ErrProjectTypeInvalid},
		{name: "invalid priority", id: "project-1", userID: "user-1", projectType: projectType, priority: invalidPriority, title: "Project", wantErr: ErrTaskPriorityInvalid},
		{name: "blank title", id: "project-1", userID: "user-1", projectType: projectType, priority: priority, title: " ", wantErr: ErrProjectTitleEmpty},
		{name: "end before start", id: "project-1", userID: "user-1", projectType: projectType, priority: priority, title: "Project", startDate: start, endDate: previous, wantErr: ErrProjectEndDateBeforeStartDate},
		{name: "start date only allowed", id: "project-1", userID: "user-1", projectType: projectType, priority: priority, title: "Project", startDate: start},
		{name: "end date only allowed", id: "project-1", userID: "user-1", projectType: projectType, priority: priority, title: "Project", endDate: start},
		{name: "both dates absent allowed", id: "project-1", userID: "user-1", projectType: projectType, priority: priority, title: "Project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProject(tt.id, tt.userID, tt.projectType, tt.priority, tt.title, tt.startDate, tt.endDate)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewProject() error = %v, want %v", err, tt.wantErr)
			}
			if errors.Is(err, ErrProjectEndDateBeforeStartDate) {
				if !got.IsZero() {
					t.Errorf("project = %+v, want zero value after invalid dates", got)
				}
				return
			}
			if got.ID != tt.id || got.UserID != tt.userID || got.Title != tt.title {
				t.Errorf("project = %+v, want input values to be preserved", got)
			}
		})
	}
}

func TestNewProjectWithDetailsValidation(t *testing.T) {
	projectType := mustProjectType(t, "work")
	priority := mustTaskPriority(t, "low")
	start := datePtr(2026, time.July, 18)

	tests := []struct {
		name     string
		progress int
		wantErr  error
	}{
		{name: "negative progress", progress: -1, wantErr: ErrProjectProgressInvalid},
		{name: "progress too high", progress: 101, wantErr: ErrProjectProgressInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProjectWithDetails("project-1", "user-1", projectType, "Project", "", "", tt.progress, priority, start, nil)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewProjectWithDetails() error = %v, want %v", err, tt.wantErr)
			}
			if got.Progress != tt.progress || got.Schedule.StartDate == nil || !got.Schedule.StartDate.Equal(*start) || got.Schedule.EndDate != nil {
				t.Errorf("project = %+v, want input details to be preserved", got)
			}
		})
	}
}

func mustProjectType(t *testing.T, value string) ProjectType {
	t.Helper()
	projectType, err := NewProjectType(value)
	if err != nil {
		t.Fatalf("NewProjectType() error = %v", err)
	}
	return projectType
}

func assertTaskDomainErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("error = %v, want %v", got, want)
	}
}
