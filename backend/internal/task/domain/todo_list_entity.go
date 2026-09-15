package domain

import (
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/shared"
)

var (
	ErrTodoListIDEmpty       = errors.New("todo list ID cannot be empty")
	ErrTodoListUserIDEmpty   = errors.New("todo list user ID cannot be empty")
	ErrTodoListListDateEmpty = errors.New("todo list date must be set")
)

type TodoListID shared.ID

type TodoList struct {
	ID        TodoListID
	UserID    UserID
	ListDate  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewTodoList(id TodoListID, userID UserID, listDate time.Time) (TodoList, error) {
	list := TodoList{
		ID:       id,
		UserID:   userID,
		ListDate: listDate,
	}
	return list, list.Validate()
}

func NewExistingTodoList(
	id TodoListID,
	userID UserID,
	listDate time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) (TodoList, error) {
	list := TodoList{
		ID:        id,
		UserID:    userID,
		ListDate:  listDate,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	if err := list.Validate(); err != nil {
		return NewZeroTodoList(), err
	}

	return list, nil
}

func NewZeroTodoList() TodoList {
	return TodoList{}
}

func (l TodoList) IsZero() bool {
	return l.ID == ""
}

func (l TodoList) Validate() error {
	if l.ID == "" {
		return ErrTodoListIDEmpty
	}
	if l.UserID == "" {
		return ErrTodoListUserIDEmpty
	}
	if l.ListDate.IsZero() {
		return ErrTodoListListDateEmpty
	}

	return nil
}
