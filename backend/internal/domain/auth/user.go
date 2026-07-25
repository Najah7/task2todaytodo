package auth

import (
	"errors"

	"github.com/Najah7/task2schedule/internal/domain/shared"
)

var (
	ErrPasswordMustBeHashed    = errors.New("password must be hashed")
	ErrPasswordMustNotBeHashed = errors.New("password must not be hashed")
	ErrFirstNameRequired       = errors.New("first name is required")
)

type UserID shared.ID

type User struct {
	ID       UserID
	Name     UserName
	Email    Email
	Password Password
}

func NewUser(id UserID, email Email, password Password, name UserName) User {
	return User{
		ID:       id,
		Name:     name,
		Email:    email,
		Password: password,
	}
}

func NewZeroUser() User {
	return User{}
}

func (u User) IsZero() bool {
	return u.ID == ""
}

func (u User) FullName() string {
	return u.Name.FullName()
}

func (u User) UpdateName(name UserName) (User, error) {
	if name.FirstName == "" {
		return NewZeroUser(), ErrFirstNameRequired
	}
	return NewUser(u.ID, u.Email, u.Password, name), nil
}

func (u User) UpdateEmail(email Email) User {
	return NewUser(u.ID, email, u.Password, u.Name)
}

func (u User) UpdatePassword(password Password) User {
	return NewUser(u.ID, u.Email, password, u.Name)
}

func (u User) Login(email Email, password Password) bool {
	return u.Email == email && u.Password == password
}
