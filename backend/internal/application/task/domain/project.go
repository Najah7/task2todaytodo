package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrProjectIDEmpty         = errors.New("project ID cannot be empty")
	ErrProjectUserIDEmpty     = errors.New("project user ID cannot be empty")
	ErrProjectTitleEmpty      = errors.New("project title cannot be empty")
	ErrProjectProgressInvalid = errors.New("project progress must be between 0 and 100")
	ErrProjectNotFound        = errors.New("project not found")
)

type Project struct {
	ID          ProjectID
	UserID      UserID
	Type        ProjectType
	Title       string
	Goal        string
	Description string
	Progress    int
	Priority    TaskPriority
	Schedule    ProjectSchedule
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewProject(
	id ProjectID,
	userID UserID,
	projectType ProjectType,
	priority TaskPriority,
	title string,
	startDate *time.Time,
	endDate *time.Time,
) (Project, error) {
	if priority == (TaskPriority{}) {
		priority = taskPriorities["low"]
	}
	schedule, err := NewProjectSchedule(startDate, endDate)
	if err != nil {
		return Project{}, err
	}
	p := Project{
		ID:       id,
		UserID:   userID,
		Type:     projectType,
		Priority: priority,
		Title:    title,
		Schedule: schedule,
	}
	return p, p.Validate()
}

func NewProjectWithDetails(
	id ProjectID,
	userID UserID,
	projectType ProjectType,
	title string,
	goal string,
	description string,
	progress int,
	priority TaskPriority,
	startDate *time.Time,
	endDate *time.Time,
) (Project, error) {
	if priority == (TaskPriority{}) {
		priority = taskPriorities["low"]
	}
	schedule, err := NewProjectSchedule(startDate, endDate)
	if err != nil {
		return Project{}, err
	}
	p := Project{
		ID:          id,
		UserID:      userID,
		Type:        projectType,
		Title:       title,
		Goal:        goal,
		Description: description,
		Progress:    progress,
		Priority:    priority,
		Schedule:    schedule,
	}
	return p, p.Validate()
}

func NewExistingProject(
	id ProjectID,
	userID UserID,
	projectType ProjectType,
	title string,
	goal string,
	description string,
	progress int,
	priority TaskPriority,
	startDate *time.Time,
	endDate *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) (Project, error) {
	schedule, err := NewProjectSchedule(startDate, endDate)
	if err != nil {
		return NewZeroProject(), err
	}
	p := Project{
		ID:          id,
		UserID:      userID,
		Type:        projectType,
		Title:       title,
		Goal:        goal,
		Description: description,
		Progress:    progress,
		Priority:    priority,
		Schedule:    schedule,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
	if err := p.Validate(); err != nil {
		return NewZeroProject(), err
	}

	return p, nil
}

func NewZeroProject() Project {
	return Project{}
}

func (p Project) IsZero() bool {
	return p.ID == ""
}

func (p Project) Validate() error {
	if p.ID == "" {
		return ErrProjectIDEmpty
	}
	if p.UserID == "" {
		return ErrProjectUserIDEmpty
	}
	if err := p.Type.validate(); err != nil {
		return err
	}
	if p.Priority != (TaskPriority{}) {
		if err := p.Priority.validate(); err != nil {
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
