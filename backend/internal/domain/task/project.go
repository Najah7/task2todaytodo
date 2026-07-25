package task

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrProjectIDEmpty                 = errors.New("project ID cannot be empty")
	ErrProjectUserIDEmpty             = errors.New("project user ID cannot be empty")
	ErrProjectTitleEmpty              = errors.New("project title cannot be empty")
	ErrProjectStartAtEmpty            = errors.New("project start time must be set")
	ErrProjectEndAtEmpty              = errors.New("project end time must be set")
	ErrProjectProgressInvalid         = errors.New("project progress must be between 0 and 100")
	ErrProjectEndAtMustBeAfterStartAt = errors.New("project end time must be after start time")
)

type Project struct {
	ID          ProjectID
	UserID      UserID
	Type        ProjectType
	Title       string
	Goal        string
	Description string
	Progress    int
	StartAt     time.Time
	EndAt       time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewProject(
	id ProjectID,
	userID UserID,
	projectType ProjectType,
	title string,
	startAt time.Time,
	endAt time.Time,
) (Project, error) {
	p := Project{
		ID:      id,
		UserID:  userID,
		Type:    projectType,
		Title:   title,
		StartAt: startAt,
		EndAt:   endAt,
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
	startAt time.Time,
	endAt time.Time,
) (Project, error) {
	p := Project{
		ID:          id,
		UserID:      userID,
		Type:        projectType,
		Title:       title,
		Goal:        goal,
		Description: description,
		Progress:    progress,
		StartAt:     startAt,
		EndAt:       endAt,
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
	startAt time.Time,
	endAt time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) (Project, error) {
	p := Project{
		ID:          id,
		UserID:      userID,
		Type:        projectType,
		Title:       title,
		Goal:        goal,
		Description: description,
		Progress:    progress,
		StartAt:     startAt,
		EndAt:       endAt,
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
	if strings.TrimSpace(p.Title) == "" {
		return ErrProjectTitleEmpty
	}
	if p.StartAt.IsZero() {
		return ErrProjectStartAtEmpty
	}
	if p.EndAt.IsZero() {
		return ErrProjectEndAtEmpty
	}
	if p.Progress < 0 || p.Progress > 100 {
		return ErrProjectProgressInvalid
	}
	if !p.EndAt.After(p.StartAt) {
		return ErrProjectEndAtMustBeAfterStartAt
	}

	return nil
}
