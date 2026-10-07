package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewTagValidatesIdentityAndTrimsName(t *testing.T) {
	tag, err := NewTag("tag-1", "user-1", "  Work  ")
	if err != nil || tag.Name != "Work" || tag.UserID != "user-1" {
		t.Fatalf("NewTag() = %+v, %v", tag, err)
	}
	for _, input := range []struct {
		id         TagID
		user, name string
		want       error
	}{
		{user: "user-1", name: "Work", want: ErrTagIDEmpty},
		{id: "tag-1", name: "Work", want: ErrTagUserIDEmpty},
		{id: "tag-1", user: "user-1", name: " ", want: ErrTagNameEmpty},
	} {
		if _, err := NewTag(input.id, input.user, input.name); !errors.Is(err, input.want) {
			t.Errorf("NewTag(%q,%q,%q) error=%v, want %v", input.id, input.user, input.name, err, input.want)
		}
	}
}

func TestRenameTagPreservesIdentityAndOwner(t *testing.T) {
	tag, err := NewExistingTag("tag-1", "user-1", "Work", time.Unix(10, 0), time.Unix(20, 0))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := tag.Rename(" Home ")
	if err != nil || updated.ID != tag.ID || updated.UserID != tag.UserID || updated.Name != "Home" || updated.CreatedAt != tag.CreatedAt {
		t.Fatalf("Rename() = %+v, %v", updated, err)
	}
}
