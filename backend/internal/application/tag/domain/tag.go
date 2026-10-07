package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrTagIDEmpty     = errors.New("tag ID cannot be empty")
	ErrTagUserIDEmpty = errors.New("tag owner ID cannot be empty")
	ErrTagNameEmpty   = errors.New("tag name cannot be empty")
)

type TagID string

type Tag struct {
	ID        TagID
	UserID    string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewTag(id TagID, userID, name string) (Tag, error) {
	tag := Tag{ID: id, UserID: userID, Name: strings.TrimSpace(name)}
	return tag, tag.Validate()
}

func NewExistingTag(id TagID, userID, name string, createdAt, updatedAt time.Time) (Tag, error) {
	tag := Tag{ID: id, UserID: userID, Name: name, CreatedAt: createdAt, UpdatedAt: updatedAt}
	if err := tag.Validate(); err != nil {
		return Tag{}, err
	}
	return tag, nil
}

func (tag Tag) Validate() error {
	if tag.ID == "" {
		return ErrTagIDEmpty
	}
	if tag.UserID == "" {
		return ErrTagUserIDEmpty
	}
	if strings.TrimSpace(tag.Name) == "" {
		return ErrTagNameEmpty
	}
	return nil
}

func (tag Tag) Rename(name string) (Tag, error) {
	tag.Name = strings.TrimSpace(name)
	return tag, tag.Validate()
}
