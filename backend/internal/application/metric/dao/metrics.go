package dao

type CompletionDay struct {
	Date            string
	PlannedItems    int32
	CompletedItems  int32
	CompletionRatio *float64
}

type TodoCompletion struct {
	Timezone          string
	StartDate         string
	EndDate           string
	TotalPlannedItems int32
	CompletedItems    int32
	CompletionRatio   *float64
	Days              []CompletionDay
}

type OverdueProject struct {
	ProjectID    *string
	ProjectTitle *string
	Count        int32
}

type OverdueWork struct {
	Timezone   string
	AsOfDate   string
	TotalCount int32
	Projects   []OverdueProject
	OtherCount int32
}
