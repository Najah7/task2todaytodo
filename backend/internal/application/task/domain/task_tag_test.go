package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewTaskTagTrimsNameAndRequiresIdentity(t *testing.T) {
	tag, err := NewTaskTag("tag-id", "user-id", "  Work  ")
	if err != nil {
		t.Fatalf("NewTaskTag() error = %v", err)
	}
	if tag.Name != "Work" {
		t.Fatalf("Name = %q, want Work", tag.Name)
	}

	if _, err := NewTaskTag("", "user-id", "Work"); !errors.Is(err, ErrTaskTagIDEmpty) {
		t.Fatalf("empty ID error = %v", err)
	}
	if _, err := NewTaskTag("tag-id", "", "Work"); !errors.Is(err, ErrTaskTagUserIDEmpty) {
		t.Fatalf("empty user ID error = %v", err)
	}
	if _, err := NewTaskTag("tag-id", "user-id", " \t "); !errors.Is(err, ErrTaskTagNameEmpty) {
		t.Fatalf("empty name error = %v", err)
	}
}

func TestTaskTagRenamePreservesOwnerAndIdentity(t *testing.T) {
	tag, err := NewTaskTag("tag-id", "user-id", "Work")
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := tag.Rename("  Focus  ")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ID != tag.ID || renamed.UserID != tag.UserID || renamed.Name != "Focus" {
		t.Fatalf("Rename() = %#v", renamed)
	}
	if _, err := tag.Rename(" "); !errors.Is(err, ErrTaskTagNameEmpty) {
		t.Fatalf("empty rename error = %v", err)
	}
}

func TestNewExistingTaskTagPreservesStoredNameAndTimes(t *testing.T) {
	createdAt := time.Date(2026, 10, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	tag, err := NewExistingTaskTag("tag-id", "user-id", "Work", createdAt, updatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if tag.CreatedAt != createdAt || tag.UpdatedAt != updatedAt || tag.Name != "Work" {
		t.Fatalf("restored TaskTag = %#v", tag)
	}
}
