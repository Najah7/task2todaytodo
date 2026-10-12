package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewRuleDefinitionValidatesScheduleSpecificConstraints(t *testing.T) {
	start := time.Date(2026, time.January, 12, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	valid := RuleDefinition{
		Title:      "Planning",
		LocalStart: start,
		LocalEnd:   end,
		Timezone:   "Asia/Tokyo",
		Frequency:  FrequencyWeekly,
		Interval:   1,
		Weekdays:   []time.Weekday{time.Monday},
	}

	tests := []struct {
		name string
		edit func(*RuleDefinition)
		want error
	}{
		{name: "valid weekly rule", edit: func(*RuleDefinition) {}, want: nil},
		{name: "blank title", edit: func(rule *RuleDefinition) { rule.Title = "  " }, want: ErrRuleTitleEmpty},
		{name: "missing timezone", edit: func(rule *RuleDefinition) { rule.Timezone = "" }, want: ErrRuleTimezoneEmpty},
		{name: "end not after start", edit: func(rule *RuleDefinition) { rule.LocalEnd = rule.LocalStart }, want: ErrRuleTimeRange},
		{name: "all day requires midnight bounds", edit: func(rule *RuleDefinition) { rule.AllDay = true }, want: ErrRuleAllDayTime},
		{name: "once cannot repeat", edit: func(rule *RuleDefinition) { rule.Frequency = FrequencyOnce }, want: ErrRuleInterval},
		{name: "weekly requires weekdays", edit: func(rule *RuleDefinition) { rule.Weekdays = nil }, want: ErrRuleWeekdays},
		{name: "duplicate weekdays rejected", edit: func(rule *RuleDefinition) { rule.Weekdays = []time.Weekday{time.Monday, time.Monday} }, want: ErrRuleWeekdays},
		{name: "weekday outside weekly frequency rejected", edit: func(rule *RuleDefinition) { rule.Frequency = FrequencyDaily }, want: ErrRuleWeekdays},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition := valid
			definition.Weekdays = append([]time.Weekday(nil), valid.Weekdays...)
			test.edit(&definition)
			_, err := NewRuleDefinition(definition)
			if !errors.Is(err, test.want) {
				t.Fatalf("NewRuleDefinition() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNewRuleDefinitionCopiesAndNormalizesInputs(t *testing.T) {
	start := time.Date(2026, time.January, 12, 9, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.January, 30, 16, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	definition := RuleDefinition{
		Title: "Planning", LocalStart: start, LocalEnd: start.Add(time.Hour),
		Timezone: "Asia/Tokyo", Frequency: FrequencyWeekly, Interval: 1,
		Weekdays: []time.Weekday{time.Monday}, EndDate: &endDate,
	}
	created, err := NewRuleDefinition(definition)
	if err != nil {
		t.Fatalf("NewRuleDefinition() error = %v", err)
	}
	definition.Weekdays[0] = time.Friday
	if created.Weekdays[0] != time.Monday {
		t.Fatalf("weekdays alias caller input: %v", created.Weekdays)
	}
	if created.EndDate.Location() != time.UTC || created.EndDate.Hour() != 0 {
		t.Fatalf("end date was not normalized to UTC date: %v", created.EndDate)
	}
}
