package shared

import "errors"

var ErrIDEmpty = errors.New("ID cannot be empty")

type IDGenerator interface {
	Generate() string
}

type ID string

func NewID(id string) (ID, error) {
	if id == "" {
		return "", ErrIDEmpty
	}

	return ID(id), nil
}
