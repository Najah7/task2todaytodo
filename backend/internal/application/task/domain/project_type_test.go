package domain

import (
	"errors"
	"testing"
)

func TestNewProjectType(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    ProjectType
		wantErr error
	}{
		{name: "work", value: "work", want: ProjectType{Value: "work", Label: "Work", LabelJp: "仕事"}},
		{name: "side work", value: "side_work", want: ProjectType{Value: "side_work", Label: "Side work", LabelJp: "副業"}},
		{name: "study", value: "study", want: ProjectType{Value: "study", Label: "Study", LabelJp: "勉強"}},
		{name: "book", value: "book", want: ProjectType{Value: "book", Label: "Book", LabelJp: "読書"}},
		{name: "personal project", value: "personal_project", want: ProjectType{Value: "personal_project", Label: "Personal Project", LabelJp: "個人プロジェクト"}},
		{name: "hobby", value: "hobby", want: ProjectType{Value: "hobby", Label: "Hobby", LabelJp: "趣味"}},
		{name: "other", value: "other", want: ProjectType{Value: "other", Label: "Other", LabelJp: "その他"}},
		{name: "empty", wantErr: ErrProjectTypeEmpty},
		{name: "invalid", value: "fitness", wantErr: ErrProjectTypeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProjectType(tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewProjectType() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("project type = %+v, want %+v", got, tt.want)
			}
			if err == nil && got.String() != tt.want.Value {
				t.Errorf("project type string = %q, want %q", got.String(), tt.want.Value)
			}
		})
	}
}
