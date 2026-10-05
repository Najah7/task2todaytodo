package domain

import (
	"testing"
	"time"
)

func TestNewTodoList(t *testing.T) {
	listDate := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)

	list, err := NewTodoList("todo-list-1", "user-1", listDate)
	if err != nil {
		t.Fatalf("NewTodoList() error = %v", err)
	}

	if list.ID != "todo-list-1" || list.UserID != "user-1" || list.ListDate != listDate {
		t.Errorf("todo list = %+v, want ID, user ID, and list date to be set", list)
	}
}

func TestNewTodoListValidation(t *testing.T) {
	listDate := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		id       TodoListID
		userID   UserID
		listDate time.Time
		wantErr  error
	}{
		{name: "empty ID", userID: "user-1", listDate: listDate, wantErr: ErrTodoListIDEmpty},
		{name: "empty user ID", id: "todo-list-1", listDate: listDate, wantErr: ErrTodoListUserIDEmpty},
		{name: "missing list date", id: "todo-list-1", userID: "user-1", wantErr: ErrTodoListListDateEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTodoList(tt.id, tt.userID, tt.listDate)
			assertTaskDomainErrorIs(t, err, tt.wantErr)
			if got.ID != tt.id || got.UserID != tt.userID || got.ListDate != tt.listDate {
				t.Errorf("todo list = %+v, want input values to be preserved", got)
			}
		})
	}
}
