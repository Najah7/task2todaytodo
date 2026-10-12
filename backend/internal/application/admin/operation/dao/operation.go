package dao

import "time"

type Operation struct {
	ID                   string
	Name                 string
	JobName              string
	State                string
	RequestedBy          string
	Cutoff               time.Time
	CreatedAt            time.Time
	StartedAt            time.Time
	FinishedAt           *time.Time
	ProcessedRules       int64
	GeneratedSchedules   int64
	GeneratedOccurrences int64
	Response             map[string]int64
	ErrorCode            string
	ErrorMessage         string
}

func (o Operation) Done() bool { return o.State == "succeeded" || o.State == "failed" }
