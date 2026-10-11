package adapter

import (
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

type IDIssuer struct{}

func NewIDIssuer() IDIssuer { return IDIssuer{} }

func (i IDIssuer) NewUUID() string { return uuid.NewString() }

func (i IDIssuer) NewULID() string { return ulid.Make().String() }
