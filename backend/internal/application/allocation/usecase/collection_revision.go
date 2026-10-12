package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
)

func rulesRevision(userID, timezone string, items []dao.Rule) string {
	items = append([]dao.Rule(nil), items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	type ruleState struct {
		ID, ProjectID, ProjectTitle, SourceRevisionID      string
		Revision                                           int32
		Weekday                                            int
		StartTime, EndTime                                 string
		EndDayOffset                                       int
		Timezone, PausedReason                             string
		EffectiveFrom, EffectiveAt, StoppedAfter, PausedAt *time.Time
	}
	canonical := struct {
		Owner, Resource, Timezone string
		Items                     []ruleState
	}{Owner: userID, Resource: "allocation-rules", Timezone: timezone, Items: make([]ruleState, 0, len(items))}
	for _, item := range items {
		canonical.Items = append(canonical.Items, ruleState{
			ID: item.ID, ProjectID: item.ProjectID, ProjectTitle: item.ProjectTitle,
			SourceRevisionID: item.SourceRevisionID, Revision: item.Revision,
			Weekday: item.Weekday, StartTime: item.StartTime, EndTime: item.EndTime,
			EndDayOffset: item.EndDayOffset, Timezone: item.Timezone,
			PausedReason: item.PausedReason, EffectiveFrom: optionalTime(item.EffectiveFrom),
			EffectiveAt: optionalTime(item.EffectiveAt), StoppedAfter: normalizedTime(item.StoppedAfter), PausedAt: normalizedTime(item.PausedAt),
		})
	}
	return contentRevision(canonical)
}

func plansRevision(userID string, items []dao.PlanItem) string {
	items = append([]dao.PlanItem(nil), items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ProjectID < items[j].ProjectID })
	canonical := struct {
		Owner, Resource string
		Items           []dao.PlanItem
	}{Owner: userID, Resource: "allocation-plans", Items: items}
	return contentRevision(canonical)
}

func contentRevision(value any) string {
	data, _ := json.Marshal(value)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}

func normalizedTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}
