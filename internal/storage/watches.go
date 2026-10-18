package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrConflict = errors.New("storage: concurrent state change")

// WatchRule is an immutable version of one user's price criteria.
type WatchRule struct {
	Version             int
	TargetMinor         *int64
	DropBasisPoints     *int
	ConditionOperator   string
	AllowMember         bool
	AllowMultibuy       bool
	NotificationChannel string
	RearmPolicyJSON     string
	CreatedAt           int64
}

// CreateWatch inserts a watch and its initial rule version atomically.
func (s *Store) CreateWatch(ctx context.Context, id, productID, listingID string, active bool, rule WatchRule, now time.Time) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(productID) == "" {
		return errors.New("storage: watch and product IDs are required")
	}
	if err := validateWatchRule(rule); err != nil {
		return err
	}
	if rule.Version != 0 && rule.Version != 1 {
		return errors.New("storage: initial watch rule version must be one")
	}
	createdAt := now.UTC().UnixMilli()
	if rule.CreatedAt != 0 {
		createdAt = rule.CreatedAt
	}
	if createdAt <= 0 {
		return errors.New("storage: watch creation time must be positive")
	}
	activeValue := 0
	if active {
		activeValue = 1
	}
	rule.Version = 1
	rule.CreatedAt = createdAt
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin watch creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO watches(id, canonical_product_id, listing_id, active, created_at, current_rule_version)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, 1)`, id, productID, listingID, activeValue, createdAt); err != nil {
		return fmt.Errorf("storage: insert watch: %w", err)
	}
	if err := insertRuleVersion(ctx, tx, id, rule); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit watch: %w", err)
	}
	return nil
}

// UpdateWatchRule appends a new immutable rule version if expectedVersion is
// still current. Competing edits are rejected instead of silently overwriting.
func (s *Store) UpdateWatchRule(ctx context.Context, watchID string, expectedVersion int, rule WatchRule, now time.Time) (int, error) {
	if strings.TrimSpace(watchID) == "" || expectedVersion < 1 {
		return 0, errors.New("storage: watch ID and current version are required")
	}
	if err := validateWatchRule(rule); err != nil {
		return 0, err
	}
	newVersion := expectedVersion + 1
	createdAt := now.UTC().UnixMilli()
	if createdAt <= 0 {
		return 0, errors.New("storage: rule update time must be positive")
	}
	rule.Version = newVersion
	rule.CreatedAt = createdAt
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("storage: begin watch rule update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var current int
	if err := tx.QueryRowContext(ctx, "SELECT current_rule_version FROM watches WHERE id = ?", watchID).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("storage: read watch rule version: %w", err)
	}
	if current != expectedVersion {
		return current, ErrConflict
	}
	if err := insertRuleVersion(ctx, tx, watchID, rule); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, "UPDATE watches SET current_rule_version = ? WHERE id = ? AND current_rule_version = ?", newVersion, watchID, expectedVersion)
	if err != nil {
		return 0, fmt.Errorf("storage: advance watch rule version: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return current, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("storage: commit watch rule update: %w", err)
	}
	return newVersion, nil
}

func validateWatchRule(rule WatchRule) error {
	if rule.NotificationChannel != "" && rule.NotificationChannel != "in_app" && rule.NotificationChannel != "webhook" {
		return errors.New("storage: watch notification channel must be in_app or webhook")
	}
	if rule.TargetMinor == nil && rule.DropBasisPoints == nil {
		return errors.New("storage: a watch rule needs a target price or relative drop")
	}
	if rule.TargetMinor != nil && *rule.TargetMinor <= 0 {
		return errors.New("storage: target price must be positive minor units")
	}
	if rule.DropBasisPoints != nil && (*rule.DropBasisPoints < 1 || *rule.DropBasisPoints > 9900) {
		return errors.New("storage: relative drop must be between 1 and 9900 basis points")
	}
	if rule.ConditionOperator == "" {
		rule.ConditionOperator = "any"
	}
	if rule.ConditionOperator != "any" && rule.ConditionOperator != "all" {
		return errors.New("storage: watch condition must be any or all")
	}
	if rule.ConditionOperator == "all" && (rule.TargetMinor == nil || rule.DropBasisPoints == nil) {
		return errors.New("storage: all condition requires both price and relative drop")
	}
	if len(rule.RearmPolicyJSON) > 4096 {
		return errors.New("storage: re-arm policy exceeds 4096 bytes")
	}
	if rule.RearmPolicyJSON != "" && !json.Valid([]byte(rule.RearmPolicyJSON)) {
		return errors.New("storage: re-arm policy must be valid JSON")
	}
	return nil
}

func insertRuleVersion(ctx context.Context, tx *sql.Tx, watchID string, rule WatchRule) error {
	operator := rule.ConditionOperator
	if operator == "" {
		operator = "any"
	}
	policy := rule.RearmPolicyJSON
	if policy == "" {
		policy = "{}"
	}
	channel := rule.NotificationChannel
	if channel == "" {
		channel = "in_app"
	}
	allowMember, allowMultibuy := 0, 0
	if rule.AllowMember {
		allowMember = 1
	}
	if rule.AllowMultibuy {
		allowMultibuy = 1
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO watch_rule_versions
		(watch_id, version, target_minor, drop_basis_points, condition_operator, allow_member, allow_multibuy, notification_channel, rearm_policy_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, watchID, rule.Version, rule.TargetMinor, rule.DropBasisPoints,
		operator, allowMember, allowMultibuy, channel, policy, rule.CreatedAt); err != nil {
		return fmt.Errorf("storage: insert watch rule version %d: %w", rule.Version, err)
	}
	return nil
}
