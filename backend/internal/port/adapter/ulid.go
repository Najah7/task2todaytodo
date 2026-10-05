package adapter

import (
	"github.com/oklog/ulid/v2"
)

type ULID struct{}

func NewULID() ULID {
	return ULID{}
}

func (g ULID) Generate() string {
	return ulid.Make().String()
}
