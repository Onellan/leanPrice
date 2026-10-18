// Package alerts connects deterministic rule evaluation to the durable SQLite
// episode and in-app notification outbox.
package alerts

import (
	"context"
	"fmt"
	"time"

	"github.com/Onellan/leanPrice/internal/pricing"
	"github.com/Onellan/leanPrice/internal/storage"
	"github.com/Onellan/leanPrice/internal/watches"
	"github.com/google/uuid"
)

type Repository interface {
	ListWatchEvaluations(context.Context, time.Time, time.Duration, int) ([]storage.WatchEvaluation, error)
	CreateAlertEpisode(context.Context, storage.AlertDelivery) (string, bool, error)
	CloseRecoveredAlertEpisode(context.Context, storage.AlertRecovery) (bool, error)
}

type Engine struct {
	Repository     Repository
	Now            func() time.Time
	Freshness      time.Duration
	BaselineAge    time.Duration
	MaxRows        int
	DestinationKey string
}

type Summary struct {
	Evaluated int
	Created   int
	Recovered int
}

func (e Engine) Evaluate(ctx context.Context) (Summary, error) {
	if e.Repository == nil {
		return Summary{}, fmt.Errorf("alerts: repository is required")
	}
	now := time.Now().UTC()
	if e.Now != nil {
		now = e.Now().UTC()
	}
	freshness := e.Freshness
	if freshness <= 0 || freshness > 365*24*time.Hour {
		freshness = watches.DefaultFreshness
	}
	baselineAge := e.BaselineAge
	if baselineAge <= 0 || baselineAge > 365*24*time.Hour {
		baselineAge = watches.DefaultBaselineAge
	}
	rows, err := e.Repository.ListWatchEvaluations(ctx, now, baselineAge, e.MaxRows)
	if err != nil {
		return Summary{}, fmt.Errorf("alerts: load current watch evidence: %w", err)
	}
	summary := Summary{}
	for _, row := range rows {
		summary.Evaluated++
		rule := watches.Rule{Version: row.Rule.Version, Operator: row.Rule.ConditionOperator,
			AllowMember: row.Rule.AllowMember, AllowMultibuy: row.Rule.AllowMultibuy,
			Freshness: freshness, BaselineAge: baselineAge}
		if row.Rule.TargetMinor != nil {
			rule.Target = &pricing.Money{Minor: *row.Rule.TargetMinor, Currency: "ZAR", Scale: 2}
		}
		rule.DropBasisPoints = row.Rule.DropBasisPoints
		var baseline *watches.Baseline
		if row.Baseline != nil {
			baseline = &watches.Baseline{ListingID: row.ListingID, Offer: *row.Baseline, SourceApproved: true, ExactMatch: true, Valid: true}
		}
		result := watches.Evaluate(rule, row.Current, baseline, now)
		if result.Triggered {
			id := uuid.NewString()
			destination := row.Rule.NotificationChannel
			if destination == "" {
				destination = e.DestinationKey
			}
			if destination == "" {
				destination = "in_app"
			}
			_, created, err := e.Repository.CreateAlertEpisode(ctx, storage.AlertDelivery{
				EpisodeID: id, WatchID: row.WatchID, RuleVersion: row.Rule.Version, OfferID: row.OfferID,
				EpisodeKey: fmt.Sprintf("watch:%s:%s:v%d", row.WatchID, row.ListingID, row.Rule.Version),
				ReasonCode: result.ReasonCode, DestinationKey: destination, IdempotencyKey: "episode:" + id,
				CreatedAt: now.UnixMilli(), NextAttemptAt: now.UnixMilli(), MaxAge: freshness,
			})
			if err != nil {
				return summary, fmt.Errorf("alerts: create durable episode: %w", err)
			}
			if created {
				summary.Created++
			}
			continue
		}
		if result.Eligible && result.RecoveryConfirmed {
			recovered, err := e.Repository.CloseRecoveredAlertEpisode(ctx, storage.AlertRecovery{
				WatchID: row.WatchID, RuleVersion: row.Rule.Version, OfferID: row.OfferID,
				RecoveredAt: now.UnixMilli(), MaxAge: freshness,
			})
			if err != nil {
				return summary, fmt.Errorf("alerts: close recovered episode: %w", err)
			}
			if recovered {
				summary.Recovered++
			}
		}
	}
	return summary, nil
}
