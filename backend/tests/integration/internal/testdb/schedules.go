//go:build integration

package testdb

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/oklog/ulid/v2"
)

type scheduleExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// ScheduleFixture inserts one blueprint definition and one persisted occurrence
// using the current schedule schema. Use Frequency/Interval/Weekdays to make a
// recurrence root; the saved row remains the requested concrete origin.
type ScheduleFixture struct {
	BlueprintID    string
	RevisionID     string
	ScheduleID     string
	UserID         string
	ProjectID      string
	AssigneeID     string
	ChangedBy      string
	Title          string
	Description    string
	Location       string
	Timezone       string
	Frequency      string
	Interval       int
	Weekdays       []string
	OccurrenceDate string
	AnchorDate     string
	EndDate        *string
	CutoffAfter    *string
	StartAt        time.Time
	EndAt          time.Time
	Completed      bool
	Deleted        bool
	Skipped        bool
}

func InsertScheduleFixture(ctx context.Context, db scheduleExecer, fixture ScheduleFixture) error {
	if fixture.BlueprintID == "" {
		return fmt.Errorf("blueprint ID is required")
	}
	if fixture.RevisionID == "" {
		fixture.RevisionID = ulid.Make().String()
	}
	if fixture.ScheduleID == "" {
		fixture.ScheduleID = ulid.Make().String()
	}
	if fixture.AssigneeID == "" {
		fixture.AssigneeID = fixture.UserID
	}
	if fixture.ChangedBy == "" {
		fixture.ChangedBy = fixture.UserID
	}
	if fixture.Timezone == "" {
		fixture.Timezone = "UTC"
	}
	if fixture.Frequency == "" {
		fixture.Frequency = "once"
	}
	if fixture.Frequency == "once" {
		fixture.Interval = 0
	} else if fixture.Interval < 1 {
		fixture.Interval = 1
	}
	fixture.Weekdays = append([]string{}, fixture.Weekdays...)
	location, err := time.LoadLocation(fixture.Timezone)
	if err != nil {
		return fmt.Errorf("load fixture timezone %s: %w", fixture.Timezone, err)
	}
	localStart := fixture.StartAt.In(location)
	localEnd := fixture.EndAt.In(location)
	if fixture.AnchorDate == "" {
		fixture.AnchorDate = localStart.Format("2006-01-02")
	}
	if fixture.OccurrenceDate == "" {
		fixture.OccurrenceDate = fixture.AnchorDate
	}
	if _, err := db.Exec(ctx, `INSERT INTO schedule_blueprints(id,user_id) VALUES($1,$2)`, fixture.BlueprintID, fixture.UserID); err != nil {
		return fmt.Errorf("insert schedule blueprint: %w", err)
	}
	_, err = db.Exec(ctx, `
		INSERT INTO schedule_blueprint_revisions(
			id,blueprint_id,user_id,revision,effective_from,recurrence_anchor_date,local_start,local_end,
			timezone,frequency,interval_count,weekdays,end_date,cutoff_after,title,description,location,
			project_id,assignee_id,changed_by
		) VALUES($1,$2,$3,1,$4::date,$5::date,$6::timestamp,$7::timestamp,$8,$9,$10,$11,$12::date,$13::date,
			$14,NULLIF($15,''),NULLIF($16,''),NULLIF($17,''),$18,$19)
	`, fixture.RevisionID, fixture.BlueprintID, fixture.UserID, fixture.AnchorDate, fixture.AnchorDate,
		localStart.Format("2006-01-02 15:04:05"), localEnd.Format("2006-01-02 15:04:05"), fixture.Timezone,
		fixture.Frequency, fixture.Interval, fixture.Weekdays, fixture.EndDate, fixture.CutoffAfter,
		fixture.Title, fixture.Description, fixture.Location, fixture.ProjectID, fixture.AssigneeID, fixture.ChangedBy)
	if err != nil {
		return fmt.Errorf("insert schedule blueprint revision: %w", err)
	}
	var deletedAt, skippedAt *time.Time
	if fixture.Deleted {
		value := time.Now().UTC()
		deletedAt = &value
	}
	if fixture.Skipped {
		value := time.Now().UTC()
		skippedAt = &value
	}
	_, err = db.Exec(ctx, `
		INSERT INTO schedules(id,blueprint_id,source_revision_id,user_id,occurrence_date,start_at,end_at,
			title,description,location,project_id,assignee_id,timezone,completed,skipped_at,deleted_at,is_override,changed_by)
		VALUES($1,$2,$3,$4,$5::date,$6,$7,$8,NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),$12,$13,$14,$15,$16,false,$17)
	`, fixture.ScheduleID, fixture.BlueprintID, fixture.RevisionID, fixture.UserID, fixture.OccurrenceDate,
		fixture.StartAt, fixture.EndAt, fixture.Title, fixture.Description, fixture.Location, fixture.ProjectID,
		fixture.AssigneeID, fixture.Timezone, fixture.Completed, skippedAt, deletedAt, fixture.ChangedBy)
	if err != nil {
		return fmt.Errorf("insert persisted schedule occurrence: %w", err)
	}
	return nil
}

func DeleteScheduleFixture(ctx context.Context, db scheduleExecer, blueprintID string) error {
	if blueprintID == "" {
		return nil
	}
	for _, statement := range []string{
		`DELETE FROM schedule_revisions WHERE schedule_id IN (SELECT id FROM schedules WHERE blueprint_id=$1)`,
		`DELETE FROM schedules WHERE blueprint_id=$1`,
		`DELETE FROM schedule_backfill_checkpoints WHERE revision_id IN (SELECT id FROM schedule_blueprint_revisions WHERE blueprint_id=$1)`,
		`DELETE FROM schedule_project_pause_intervals WHERE blueprint_id=$1`,
		`DELETE FROM schedule_blueprint_revisions WHERE blueprint_id=$1`,
		`DELETE FROM schedule_blueprints WHERE id=$1`,
	} {
		if _, err := db.Exec(ctx, statement, blueprintID); err != nil {
			return fmt.Errorf("delete schedule fixture %s: %w", blueprintID, err)
		}
	}
	return nil
}
