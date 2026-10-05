package domain

import "errors"

var (
	ErrProjectTypeEmpty   = errors.New("project type cannot be empty")
	ErrProjectTypeInvalid = errors.New("invalid project type")
)

type ProjectType struct {
	Value   string
	Label   string
	LabelJp string
}

var projectTypes = map[string]ProjectType{
	"work":             {Value: "work", Label: "Work", LabelJp: "仕事"},
	"side_work":        {Value: "side_work", Label: "Side work", LabelJp: "副業"},
	"study":            {Value: "study", Label: "Study", LabelJp: "勉強"},
	"book":             {Value: "book", Label: "Book", LabelJp: "読書"},
	"personal_project": {Value: "personal_project", Label: "Personal Project", LabelJp: "個人プロジェクト"},
	"hobby":            {Value: "hobby", Label: "Hobby", LabelJp: "趣味"},
	"other":            {Value: "other", Label: "Other", LabelJp: "その他"},
}

func NewProjectType(projectType string) (ProjectType, error) {
	t, ok := projectTypes[projectType]
	if !ok {
		t = ProjectType{Value: projectType}
	}
	if err := t.validate(); err != nil {
		return ProjectType{}, err
	}
	return t, nil
}

func (t ProjectType) validate() error {
	if t.String() == "" {
		return ErrProjectTypeEmpty
	}

	if _, ok := projectTypes[t.Value]; ok {
		return nil
	}
	return ErrProjectTypeInvalid
}

func (t ProjectType) String() string {
	return t.Value
}
