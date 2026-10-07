package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

type ProjectID string
type UserID string

var (
	ErrProjectIDEmpty                = errors.New("project ID cannot be empty")
	ErrProjectUserIDEmpty            = errors.New("project user ID cannot be empty")
	ErrProjectTitleEmpty             = errors.New("project title cannot be empty")
	ErrProjectProgressInvalid        = errors.New("project progress must be between 0 and 100")
	ErrProjectNotFound               = errors.New("project not found")
	ErrProjectEndDateBeforeStartDate = errors.New("project end date cannot be before start date")
	ErrProjectTypeEmpty              = errors.New("project type cannot be empty")
	ErrProjectTypeInvalid            = errors.New("invalid project type")
	ErrProjectPriorityEmpty          = errors.New("project priority cannot be empty")
	ErrProjectPriorityInvalid        = errors.New("invalid project priority")
	ErrProjectMemberInvalid          = errors.New("project member requires project, user, role, and actor IDs")
)

type ProjectType struct{ Value, Label, LabelJp string }

var projectTypes = map[string]ProjectType{
	"work": {"work", "Work", "仕事"}, "side_work": {"side_work", "Side work", "副業"},
	"study": {"study", "Study", "勉強"}, "book": {"book", "Book", "読書"},
	"personal_project": {"personal_project", "Personal Project", "個人プロジェクト"},
	"hobby":            {"hobby", "Hobby", "趣味"}, "other": {"other", "Other", "その他"},
}

func NewProjectType(value string) (ProjectType, error) {
	t, ok := projectTypes[value]
	if !ok {
		t = ProjectType{Value: value}
	}
	if t.Value == "" {
		return ProjectType{}, ErrProjectTypeEmpty
	}
	if _, ok := projectTypes[t.Value]; !ok {
		return ProjectType{}, ErrProjectTypeInvalid
	}
	return t, nil
}
func (t ProjectType) String() string { return t.Value }

type Priority struct {
	Value, Label, LabelJp string
	Weight                int
}

var projectPriorities = map[string]Priority{
	"urgent": {"urgent", "Urgent", "緊急", 100}, "high": {"high", "High", "高", 50},
	"medium": {"medium", "Medium", "中", 25}, "low": {"low", "Low", "低", 10},
	"someday": {"someday", "Someday", "いつか", 0},
}

func NewPriority(value string) (Priority, error) {
	p, ok := projectPriorities[value]
	if !ok {
		p = Priority{Value: value}
	}
	if p.Value == "" {
		return Priority{}, ErrProjectPriorityEmpty
	}
	if _, ok := projectPriorities[p.Value]; !ok {
		return Priority{}, ErrProjectPriorityInvalid
	}
	return p, nil
}
func (p Priority) String() string { return p.Value }

type ProjectSchedule struct{ StartDate, EndDate *time.Time }

func NewProjectSchedule(startDate, endDate *time.Time) (ProjectSchedule, error) {
	r := ProjectSchedule{StartDate: normalizeDate(startDate), EndDate: normalizeDate(endDate)}
	if err := r.Validate(); err != nil {
		return ProjectSchedule{}, err
	}
	return r, nil
}
func (r ProjectSchedule) Validate() error {
	if r.StartDate != nil && r.EndDate != nil && r.EndDate.Before(*r.StartDate) {
		return ErrProjectEndDateBeforeStartDate
	}
	return nil
}
func normalizeDate(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	d := calendar.NormalizeCalendarDate(*v)
	return &d
}

type Project struct {
	ID                       ProjectID
	UserID                   UserID
	Type                     ProjectType
	Title, Goal, Description string
	Progress                 int
	Priority                 Priority
	Schedule                 ProjectSchedule
	CreatedAt, UpdatedAt     time.Time
}

func NewProject(id ProjectID, userID UserID, typ ProjectType, priority Priority, title string, startDate, endDate *time.Time) (Project, error) {
	if priority == (Priority{}) {
		priority = projectPriorities["low"]
	}
	schedule, err := NewProjectSchedule(startDate, endDate)
	if err != nil {
		return Project{}, err
	}
	p := Project{ID: id, UserID: userID, Type: typ, Priority: priority, Title: title, Schedule: schedule}
	return p, p.Validate()
}
func NewProjectWithDetails(id ProjectID, userID UserID, typ ProjectType, title, goal, description string, progress int, priority Priority, startDate, endDate *time.Time) (Project, error) {
	p, err := NewProject(id, userID, typ, priority, title, startDate, endDate)
	if err != nil {
		return Project{}, err
	}
	p.Goal, p.Description, p.Progress = goal, description, progress
	return p, p.Validate()
}
func NewExistingProject(id ProjectID, userID UserID, typ ProjectType, title, goal, description string, progress int, priority Priority, startDate, endDate *time.Time, createdAt, updatedAt time.Time) (Project, error) {
	schedule, err := NewProjectSchedule(startDate, endDate)
	if err != nil {
		return Project{}, err
	}
	p := Project{ID: id, UserID: userID, Type: typ, Title: title, Goal: goal, Description: description, Progress: progress, Priority: priority, Schedule: schedule, CreatedAt: createdAt, UpdatedAt: updatedAt}
	if err := p.Validate(); err != nil {
		return Project{}, err
	}
	return p, nil
}
func (p Project) Validate() error {
	if p.ID == "" {
		return ErrProjectIDEmpty
	}
	if p.UserID == "" {
		return ErrProjectUserIDEmpty
	}
	if _, err := NewProjectType(p.Type.Value); err != nil {
		return err
	}
	if p.Priority != (Priority{}) {
		if _, err := NewPriority(p.Priority.Value); err != nil {
			return err
		}
	}
	if strings.TrimSpace(p.Title) == "" {
		return ErrProjectTitleEmpty
	}
	if p.Progress < 0 || p.Progress > 100 {
		return ErrProjectProgressInvalid
	}
	return p.Schedule.Validate()
}

type ProjectMember struct {
	ProjectID ProjectID
	UserID    UserID
	RoleID    string
	AddedBy   UserID
}

func NewProjectMember(projectID ProjectID, userID UserID, roleID string, addedBy UserID) (ProjectMember, error) {
	m := ProjectMember{ProjectID: projectID, UserID: userID, RoleID: strings.TrimSpace(roleID), AddedBy: addedBy}
	if m.ProjectID == "" || m.UserID == "" || m.RoleID == "" || m.AddedBy == "" {
		return ProjectMember{}, ErrProjectMemberInvalid
	}
	return m, nil
}
