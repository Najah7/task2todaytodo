package domain

import "strings"

type UserName struct {
	FirstName string
	LastName  string
}

func NewUserName(firstName, lastName string) UserName {
	return UserName{
		FirstName: firstName,
		LastName:  lastName,
	}
}

func (n UserName) FullName() string {
	return strings.TrimSpace(n.FirstName + " " + n.LastName)
}
