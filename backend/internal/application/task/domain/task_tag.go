package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrTaskTagIDEmpty     = errors.New("task tag ID cannot be empty")
	ErrTaskTagUserIDEmpty = errors.New("task tag user ID cannot be empty")
	ErrTaskTagNameEmpty   = errors.New("task tag name cannot be empty")
)

type TaskTagID string

type TaskTag struct {
	ID        TaskTagID
	UserID    UserID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewTaskTag(id TaskTagID, userID UserID, name string) (TaskTag, error) {
	tag := TaskTag{ID: id, UserID: userID, Name: strings.TrimSpace(name)}
	return tag, tag.Validate()
}

func NewExistingTaskTag(id TaskTagID, userID UserID, name string, createdAt, updatedAt time.Time) (TaskTag, error) {
	tag := TaskTag{
		ID:        id,
		UserID:    userID,
		Name:      name,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	if err := tag.Validate(); err != nil {
		return NewZeroTaskTag(), err
	}
	return tag, nil
}

func NewZeroTaskTag() TaskTag { return TaskTag{} }

func (t TaskTag) IsZero() bool { return t.ID == "" }

func (t TaskTag) Validate() error {
	if t.ID == "" {
		return ErrTaskTagIDEmpty
	}
	if t.UserID == "" {
		return ErrTaskTagUserIDEmpty
	}
	if strings.TrimSpace(t.Name) == "" {
		return ErrTaskTagNameEmpty
	}
	return nil
}

func (t TaskTag) Rename(name string) (TaskTag, error) {
	t.Name = strings.TrimSpace(name)
	return t, t.Validate()
}
