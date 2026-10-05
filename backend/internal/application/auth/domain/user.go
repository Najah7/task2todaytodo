package domain

import "errors"

var (
	ErrPasswordMustBeHashed    = errors.New("password must be hashed")
	ErrPasswordMustNotBeHashed = errors.New("password must not be hashed")
	ErrFirstNameRequired       = errors.New("first name is required")
)

type UserID string

type User struct {
	ID       UserID
	Name     UserName
	Email    Email
	Password Password
	Timezone UserTimezone
}

func NewUser(id UserID, email Email, password Password, name UserName) User {
	return User{
		ID:       id,
		Name:     name,
		Email:    email,
		Password: password,
		Timezone: DefaultUserTimezone(),
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
	updated := NewUser(u.ID, u.Email, u.Password, name)
	updated.Timezone = u.Timezone
	return updated, nil
}

func (u User) UpdateEmail(email Email) User {
	updated := NewUser(u.ID, email, u.Password, u.Name)
	updated.Timezone = u.Timezone
	return updated
}

func (u User) UpdatePassword(password Password) User {
	updated := NewUser(u.ID, u.Email, password, u.Name)
	updated.Timezone = u.Timezone
	return updated
}

func (u User) UpdateTimezone(timezone UserTimezone) User {
	u.Timezone = timezone
	return u
}

func (u User) Login(email Email, password Password) bool {
	return u.Email == email && u.Password == password
}
