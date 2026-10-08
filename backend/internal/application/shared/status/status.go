package status

import "errors"

var (
	ErrEmpty   = errors.New("status cannot be empty")
	ErrInvalid = errors.New("invalid status")
)

const (
	Open            = "open"
	Pending         = "pending"
	WaitingOnOthers = "waiting_on_others"
	InProgress      = "in_progress"
	Done            = "done"
)

// Status is the shared, finite status catalog used by Tasks and Projects.
// Lifecycle transitions remain owned by each business context.
type Status struct {
	Value   string
	Label   string
	LabelJp string
}

var catalog = map[string]Status{
	Open:            {Value: Open, Label: "Open", LabelJp: "オープン"},
	Pending:         {Value: Pending, Label: "Pending", LabelJp: "保留"},
	WaitingOnOthers: {Value: WaitingOnOthers, Label: "Waiting on others", LabelJp: "他者待ち"},
	InProgress:      {Value: InProgress, Label: "In progress", LabelJp: "進行中"},
	Done:            {Value: Done, Label: "Done", LabelJp: "完了"},
}

var ordered = []string{InProgress, Pending, Done, Open, WaitingOnOthers}

func New(value string) (Status, error) {
	if value == "" {
		return Status{}, ErrEmpty
	}
	s, ok := catalog[value]
	if !ok {
		return Status{}, ErrInvalid
	}
	return s, nil
}

func All() []Status {
	statuses := make([]Status, 0, len(ordered))
	for _, value := range ordered {
		statuses = append(statuses, catalog[value])
	}
	return statuses
}

func (s Status) String() string          { return s.Value }
func (s Status) IsOpen() bool            { return s.Value == Open }
func (s Status) IsPending() bool         { return s.Value == Pending }
func (s Status) IsWaitingOnOthers() bool { return s.Value == WaitingOnOthers }
func (s Status) IsInProgress() bool      { return s.Value == InProgress }
func (s Status) IsDone() bool            { return s.Value == Done }
