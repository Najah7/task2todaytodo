package domain

import (
	"errors"
	"time"
)

var ErrInvalidTodo = errors.New("invalid todo")

type Todo struct {
	ID       string
	UserID   string
	Date     time.Time
	Timezone string
	Revision int32
}

func NewTodo(id, userID string, todoDate time.Time, timezone string) (Todo, error) {
	todo := Todo{ID: id, UserID: userID, Date: dateOnly(todoDate), Timezone: timezone, Revision: 1}
	return todo, todo.Validate()
}

func RestoreTodo(todo Todo) (Todo, error) {
	todo.Date = dateOnly(todo.Date)
	return todo, todo.Validate()
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func (todo Todo) Validate() error {
	if todo.ID == "" || todo.UserID == "" || todo.Timezone == "" || todo.Revision < 1 || todo.Date.IsZero() {
		return ErrInvalidTodo
	}
	if _, err := time.LoadLocation(todo.Timezone); err != nil {
		return ErrInvalidTodo
	}
	return nil
}

func (todo Todo) NextRevision(expected int32) (Todo, error) {
	if todo.Revision != expected || expected < 1 {
		return Todo{}, ErrInvalidTodo
	}
	todo.Revision++
	return todo, nil
}

func (todo Todo) WithTimezone(timezone string) (Todo, error) {
	todo.Timezone = timezone
	return todo, todo.Validate()
}
