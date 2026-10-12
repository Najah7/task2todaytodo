package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

// ValidateProjectResume checks whether the current user's rules for a project
// can resume at resumedAt. It is intended to run with a transaction-bound
// Repository after the Project and user rows are locked. A conflict is returned
// as a stable reason so the caller can leave the rule paused while allowing the
// Project transition to succeed.
func ValidateProjectResume(ctx context.Context, repo Repository, userID, ruleID, projectID string, resumedAt time.Time) (string, error) {
	rules, err := repo.ListRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return "", err
	}
	profile, err := repo.GetProfile(ctx, userID)
	if err != nil {
		return "", err
	}
	location, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return "", ErrInvalidRequest
	}
	from := calendar.DateOnly(resumedAt.In(location))
	through := from.AddDate(0, 0, 400)
	latestRevision := int32(0)
	latestProject := ""
	latestPausedReason := ""
	for _, rule := range rules {
		if rule.ID != ruleID {
			continue
		}
		if rule.Revision > latestRevision {
			latestRevision = rule.Revision
			latestProject = rule.ProjectID
			latestPausedReason = rule.PausedReason
		}
	}
	if latestRevision == 0 || latestProject != projectID {
		return "", ErrNotFound
	}
	if latestPausedReason == "" {
		return "", nil
	}
	dateFrom := from.AddDate(0, 0, -1)
	dateThrough := through.AddDate(0, 0, 1)
	allOriginsFrom := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	allOriginsThrough := time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)
	exceptionOrigins, err := repo.ListExceptionOrigins(ctx, userID, allOriginsFrom, allOriginsThrough)
	if err != nil {
		return "", err
	}
	candidates := make([]domain.Occurrence, 0)
	for _, rule := range rules {
		if rule.ID != ruleID {
			continue
		}
		occurrences, err := projectedRuleOccurrencesForResume(ctx, repo, userID, rule, dateFrom, dateThrough)
		if err != nil {
			return "", err
		}
		for _, occurrence := range occurrences {
			if occurrence.StartAt.Before(resumedAt) {
				continue
			}
			candidates = append(candidates, domain.Occurrence{
				RuleID: rule.ID, ProjectID: occurrence.ProjectID, OccurrenceDate: occurrence.OccurrenceDate,
				StartAt: occurrence.StartAt, EndAt: occurrence.EndAt, Timezone: occurrence.Timezone,
			})
		}
	}
	originPrefix := ruleID + "|"
	for key := range exceptionOrigins {
		if !strings.HasPrefix(key, originPrefix) {
			continue
		}
		date, err := time.Parse("2006-01-02", strings.TrimPrefix(key, originPrefix))
		if err != nil {
			return "", ErrInvalidRequest
		}
		snapshot, err := repo.ReadOccurrence(ctx, ruleID, userID, date)
		if err == ErrNotFound {
			// A tombstoned exception suppresses the ordinary origin without
			// creating a candidate to validate on resume.
			continue
		}
		if err != nil {
			return "", err
		}
		if snapshot.StartAt.Before(resumedAt) {
			continue
		}
		candidates = append(candidates, domain.Occurrence{
			RuleID: snapshot.RuleID, ProjectID: snapshot.ProjectID,
			OccurrenceDate: snapshot.OccurrenceDate, StartAt: snapshot.StartAt,
			EndAt: snapshot.EndAt, Timezone: snapshot.Timezone,
		})
	}
	if len(candidates) == 0 {
		return "", nil
	}
	if err := validateAvailability(ctx, repo, userID, candidates, ruleID, resumedAt); err != nil {
		if conflictErr, ok := err.(ConflictError); ok {
			return conflictErr.Kind, nil
		}
		return "", err
	}
	return "", nil
}
