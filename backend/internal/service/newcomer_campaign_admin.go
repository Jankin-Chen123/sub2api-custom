package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// NewcomerCampaignAdminConfig is the operator-facing campaign configuration.
// Membership grants copy factor and duration at issuance time, so changing
// this config never rewrites historical grants.
type NewcomerCampaignAdminConfig struct {
	CampaignKey string                 `json:"campaign_key"`
	Name        string                 `json:"name"`
	Phase       string                 `json:"phase"`
	StartsAt    time.Time              `json:"starts_at"`
	EndsAt      time.Time              `json:"ends_at"`
	Tiers       []NewcomerCampaignTier `json:"tiers"`
}

// NewcomerCampaignAdminUserMembership is the minimal operator view needed to
// assign or clear a user's manual membership without exposing unrelated user
// data.
type NewcomerCampaignAdminUserMembership struct {
	UserID            int64                     `json:"user_id"`
	Email             string                    `json:"email"`
	Username          string                    `json:"username"`
	ValidInviteCount  int                       `json:"valid_invite_count"`
	ManualMembership  *NewcomerMembershipStatus `json:"manual_membership,omitempty"`
	CurrentMembership *NewcomerMembershipStatus `json:"current_membership,omitempty"`
}

// NewcomerCampaignAdminMembershipInput describes an explicit administrator
// grant. Factor and duration are optional: when omitted, the selected tier's
// current config is copied into the immutable grant record.
type NewcomerCampaignAdminMembershipInput struct {
	TierKey      string
	Factor       *float64
	StartsAt     *time.Time
	ExpiresAt    *time.Time
	DurationDays *int
	Reason       string
}

// configuredCampaignTiers reads the current config and falls back to the
// frozen defaults when the new optional migration has not reached a database
// yet. The fallback keeps older installations readable; the admin endpoints
// themselves are strict and surface migration errors to operators.
func (s *NewcomerCampaignService) configuredCampaignTiers(ctx context.Context) []NewcomerCampaignTier {
	if s == nil || s.db == nil {
		return NewcomerCampaignTiers()
	}
	tiers, err := loadNewcomerCampaignTiers(ctx, s.db)
	if err != nil {
		// A migration race should not take down billing. Once the migration is
		// applied, all newly issued grants use the database config.
		return NewcomerCampaignTiers()
	}
	return tiers
}

func loadNewcomerCampaignTiers(ctx context.Context, q campaignQueryer) ([]NewcomerCampaignTier, error) {
	rows, err := q.QueryContext(ctx, `
SELECT tier_key, tier_name, threshold, factor, duration_days
FROM newcomer_campaign_tier_configs
WHERE campaign_key = $1
ORDER BY threshold ASC`, NewcomerCampaignKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tiers []NewcomerCampaignTier
	for rows.Next() {
		var tier NewcomerCampaignTier
		if err := rows.Scan(&tier.Key, &tier.Name, &tier.Threshold, &tier.Factor, &tier.DurationDays); err != nil {
			return nil, err
		}
		tiers = append(tiers, tier)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := validateNewcomerCampaignTiers(tiers); err != nil {
		return nil, err
	}
	return tiers, nil
}

func validateNewcomerCampaignTiers(tiers []NewcomerCampaignTier) error {
	if len(tiers) != len(newcomerCampaignTiers) {
		return fmt.Errorf("newcomer campaign requires exactly %d tiers", len(newcomerCampaignTiers))
	}
	expected := make(map[string]NewcomerCampaignTier, len(newcomerCampaignTiers))
	for _, tier := range newcomerCampaignTiers {
		expected[tier.Key] = tier
	}
	seen := make(map[string]bool, len(tiers))
	lastThreshold := 0
	var previousFactor float64
	var previousDuration int
	for _, tier := range tiers {
		_, ok := expected[tier.Key]
		if !ok || seen[tier.Key] {
			return fmt.Errorf("invalid or duplicate newcomer campaign tier %q", tier.Key)
		}
		if tier.Threshold <= 0 || tier.Threshold > 1000000000 {
			return fmt.Errorf("tier %q has invalid threshold", tier.Key)
		}
		if tier.Name == "" || tier.Factor <= 0 || tier.Factor > 1 || math.IsNaN(tier.Factor) || math.IsInf(tier.Factor, 0) {
			return fmt.Errorf("tier %q has invalid factor", tier.Key)
		}
		if tier.DurationDays <= 0 || tier.DurationDays > 3650 {
			return fmt.Errorf("tier %q has invalid duration_days", tier.Key)
		}
		if tier.Threshold <= lastThreshold {
			return fmt.Errorf("newcomer campaign tier thresholds must be ascending")
		}
		if len(seen) > 0 && tier.Factor > previousFactor {
			return fmt.Errorf("tier %q factor cannot increase with threshold", tier.Key)
		}
		if len(seen) > 0 && tier.DurationDays < previousDuration {
			return fmt.Errorf("tier %q duration_days cannot decrease with threshold", tier.Key)
		}
		seen[tier.Key] = true
		lastThreshold = tier.Threshold
		previousFactor = tier.Factor
		previousDuration = tier.DurationDays
	}
	return nil
}

func (s *NewcomerCampaignService) AdminGetConfig(ctx context.Context) (*NewcomerCampaignAdminConfig, error) {
	if !s.enabled() {
		return nil, errors.New("newcomer campaign service is unavailable")
	}
	tiers, err := loadNewcomerCampaignTiers(ctx, s.db)
	if err != nil {
		return nil, fmt.Errorf("load newcomer campaign tier config: %w", err)
	}
	start, end := NewcomerCampaignWindow()
	name := NewcomerCampaignName
	if s.persistentConfig {
		var err error
		start, end, err = s.loadConfiguredWindowStrict(ctx, s.db)
		if err != nil {
			return nil, fmt.Errorf("load newcomer campaign window: %w", err)
		}
		if err := s.queryOne(ctx, s.db, `
SELECT name FROM newcomer_campaign_config
WHERE campaign_key = $1`, []any{NewcomerCampaignKey}, &name); err != nil {
			return nil, fmt.Errorf("load newcomer campaign name: %w", err)
		}
	}
	now := s.currentTime()
	phase := "upcoming"
	if !now.Before(start) && now.Before(end) {
		phase = "active"
	} else if !now.Before(end) {
		phase = "ended"
	}
	return &NewcomerCampaignAdminConfig{
		CampaignKey: NewcomerCampaignKey,
		Name:        name,
		Phase:       phase,
		StartsAt:    start,
		EndsAt:      end,
		Tiers:       tiers,
	}, nil
}

func (s *NewcomerCampaignService) AdminUpdateConfig(ctx context.Context, actorID int64, tiers []NewcomerCampaignTier) (*NewcomerCampaignAdminConfig, error) {
	return s.adminUpdateConfig(ctx, actorID, tiers, nil, nil)
}

// AdminUpdateConfigWithWindow updates the long-lived fixed campaign key. A
// date edit only affects future registration/payment/invite eligibility; it
// never clears historical invitations or rewrites issued grants.
func (s *NewcomerCampaignService) AdminUpdateConfigWithWindow(ctx context.Context, actorID int64, tiers []NewcomerCampaignTier, startsAt, endsAt *time.Time) (*NewcomerCampaignAdminConfig, error) {
	return s.adminUpdateConfig(ctx, actorID, tiers, startsAt, endsAt)
}

func (s *NewcomerCampaignService) adminUpdateConfig(ctx context.Context, actorID int64, tiers []NewcomerCampaignTier, startsAt, endsAt *time.Time) (*NewcomerCampaignAdminConfig, error) {
	if !s.enabled() {
		return nil, errors.New("newcomer campaign service is unavailable")
	}
	if err := validateNewcomerCampaignTiers(tiers); err != nil {
		return nil, err
	}
	if (startsAt == nil) != (endsAt == nil) {
		return nil, errors.New("starts_at and ends_at must be provided together")
	}
	var normalizedStart, normalizedEnd time.Time
	if startsAt != nil {
		var err error
		normalizedStart, err = normalizeCampaignBoundary(*startsAt)
		if err != nil {
			return nil, err
		}
		normalizedEnd, err = normalizeCampaignBoundary(*endsAt)
		if err != nil {
			return nil, err
		}
		if !normalizedEnd.After(normalizedStart) {
			return nil, errors.New("campaign end date must be after start date")
		}
	}
	var updatedBy any
	if actorID > 0 {
		updatedBy = actorID
	}
	err := s.withTx(ctx, func(txCtx context.Context, tx *dbent.Client) error {
		if s.persistentConfig {
			if startsAt == nil {
				var err error
				normalizedStart, normalizedEnd, err = s.loadConfiguredWindowStrict(txCtx, tx)
				if err != nil {
					return err
				}
			}
			if startsAt != nil {
				if _, err := tx.ExecContext(txCtx, `
UPDATE newcomer_campaign_config
SET starts_at = $2, ends_at = $3, updated_by = $4, updated_at = NOW()
WHERE campaign_key = $1`, NewcomerCampaignKey, normalizedStart, normalizedEnd, updatedBy); err != nil {
					return err
				}
			}
			// Move all current thresholds out of the unique-key space before
			// applying the new ordered values, so arbitrary monotone edits do not
			// collide with an old threshold during a row-by-row update.
			if _, err := tx.ExecContext(txCtx, `
UPDATE newcomer_campaign_tier_configs
SET threshold = threshold + 1000000000
WHERE campaign_key = $1`, NewcomerCampaignKey); err != nil {
				return err
			}
		}
		for _, tier := range tiers {
			updateQuery := `
UPDATE newcomer_campaign_tier_configs
SET factor = $3, duration_days = $4, updated_by = $5, updated_at = NOW()
WHERE campaign_key = $1 AND tier_key = $2`
			args := []any{NewcomerCampaignKey, tier.Key, tier.Factor, tier.DurationDays, updatedBy}
			if s.persistentConfig {
				updateQuery = `
UPDATE newcomer_campaign_tier_configs
		SET threshold = $3, factor = $4, duration_days = $5, updated_by = $6, updated_at = NOW()
WHERE campaign_key = $1 AND tier_key = $2`
				args = []any{NewcomerCampaignKey, tier.Key, tier.Threshold, tier.Factor, tier.DurationDays, updatedBy}
			}
			result, err := tx.ExecContext(txCtx, updateQuery, args...)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected != 1 {
				return fmt.Errorf("newcomer campaign tier %q is not configured", tier.Key)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update newcomer campaign tier config: %w", err)
	}
	// Configuration is copied into newly-issued grants, but invalidate any
	// cached effective factors so an operator change cannot leave stale billing
	// multipliers in this process.
	s.invalidateAllMembershipFactors()
	return s.AdminGetConfig(ctx)
}

func normalizeCampaignBoundary(value time.Time) (time.Time, error) {
	value = value.UTC()
	shanghai := value.In(newcomerCampaignLocation)
	if shanghai.Hour() != 0 || shanghai.Minute() != 0 || shanghai.Second() != 0 || shanghai.Nanosecond() != 0 {
		return time.Time{}, errors.New("campaign dates must be Shanghai midnight boundaries")
	}
	return time.Date(shanghai.Year(), shanghai.Month(), shanghai.Day(), 0, 0, 0, 0, newcomerCampaignLocation).UTC(), nil
}

func (s *NewcomerCampaignService) AdminGetUserMembership(ctx context.Context, userID int64) (*NewcomerCampaignAdminUserMembership, error) {
	if !s.enabled() {
		return nil, errors.New("newcomer campaign service is unavailable")
	}
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	var result NewcomerCampaignAdminUserMembership
	result.UserID = userID
	if err := s.queryOne(ctx, s.db, `
SELECT email, COALESCE(username, '')
FROM users
WHERE id = $1 AND deleted_at IS NULL`, []any{userID}, &result.Email, &result.Username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("load newcomer campaign target user: %w", err)
	}
	if err := s.queryOne(ctx, s.db, `
SELECT COUNT(*)
FROM newcomer_campaign_invites
WHERE campaign_key = $1 AND inviter_id = $2 AND status = 'qualified'
  AND qualifying_amount >= $3`, []any{NewcomerCampaignKey, userID, newcomerInviteThreshold}, &result.ValidInviteCount); err != nil {
		return nil, fmt.Errorf("load newcomer campaign invite count: %w", err)
	}
	now := s.currentTime()
	expireResult, err := s.db.ExecContext(ctx, `
UPDATE newcomer_campaign_admin_memberships
SET status = 'expired', updated_at = $3
	WHERE campaign_key = $1 AND user_id = $2 AND status = 'active' AND expires_at <= $3`, NewcomerCampaignKey, userID, now)
	if err != nil {
		return nil, fmt.Errorf("expire newcomer campaign manual membership: %w", err)
	}
	if affected, err := expireResult.RowsAffected(); err == nil && affected > 0 {
		s.invalidateMembershipFactor(userID)
	}
	manual, err := s.loadManualMembership(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	result.ManualMembership = manual
	current, err := s.loadEffectiveMembership(ctx, userID, now, result.ValidInviteCount)
	if err != nil {
		return nil, err
	}
	result.CurrentMembership = current
	return &result, nil
}

func (s *NewcomerCampaignService) AdminSetUserMembership(ctx context.Context, actorID, userID int64, input NewcomerCampaignAdminMembershipInput) (*NewcomerCampaignAdminUserMembership, error) {
	if !s.enabled() {
		return nil, errors.New("newcomer campaign service is unavailable")
	}
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	input.TierKey = strings.ToLower(strings.TrimSpace(input.TierKey))
	if input.TierKey == "" {
		return nil, errors.New("tier_key is required")
	}
	if input.ExpiresAt != nil && input.DurationDays != nil {
		return nil, errors.New("expires_at and duration_days cannot both be provided")
	}
	if input.Factor != nil && (math.IsNaN(*input.Factor) || math.IsInf(*input.Factor, 0) || *input.Factor <= 0 || *input.Factor > 1) {
		return nil, errors.New("factor must be greater than 0 and no greater than 1")
	}
	if input.DurationDays != nil && (*input.DurationDays <= 0 || *input.DurationDays > 3650) {
		return nil, errors.New("duration_days must be between 1 and 3650")
	}
	if len([]rune(input.Reason)) > 255 {
		return nil, errors.New("reason is too long")
	}

	now := s.currentTime()
	var configuredTier NewcomerCampaignTier
	var startsAt, expiresAt time.Time
	var factor float64
	var grantedBy any
	if actorID > 0 {
		grantedBy = actorID
	}
	err := s.withTx(ctx, func(txCtx context.Context, tx *dbent.Client) error {
		tiers, err := loadNewcomerCampaignTiers(txCtx, tx)
		if err != nil {
			return fmt.Errorf("load tier config: %w", err)
		}
		for _, tier := range tiers {
			if tier.Key == input.TierKey {
				configuredTier = tier
				break
			}
		}
		if configuredTier.Key == "" {
			return fmt.Errorf("unknown newcomer campaign tier %q", input.TierKey)
		}
		var targetRole string
		if err := s.queryOne(txCtx, tx, `
SELECT role FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, []any{userID}, &targetRole); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}
		if targetRole == RoleAdmin {
			return errors.New("newcomer campaign membership can only be assigned to a user")
		}

		factor = configuredTier.Factor
		if input.Factor != nil {
			factor = *input.Factor
		}
		startsAt = now
		if input.StartsAt != nil {
			startsAt = input.StartsAt.UTC()
		}
		if input.ExpiresAt != nil {
			expiresAt = input.ExpiresAt.UTC()
		} else {
			durationDays := configuredTier.DurationDays
			if input.DurationDays != nil {
				durationDays = *input.DurationDays
			}
			expiresAt = startsAt.Add(time.Duration(durationDays) * 24 * time.Hour)
		}
		if !expiresAt.After(startsAt) {
			return errors.New("expires_at must be after starts_at")
		}

		if _, err := tx.ExecContext(txCtx, `
UPDATE newcomer_campaign_admin_memberships
SET status = 'revoked', revoked_at = $3, revoked_by = $4,
    revoke_reason = 'replaced by administrator', updated_at = $3
WHERE campaign_key = $1 AND user_id = $2 AND status = 'active'`, NewcomerCampaignKey, userID, now, grantedBy); err != nil {
			return err
		}
		_, err = tx.ExecContext(txCtx, `
INSERT INTO newcomer_campaign_admin_memberships
    (campaign_key, user_id, tier_key, factor, starts_at, expires_at, status, granted_by, reason)
VALUES ($1, $2, $3, $4, $5, $6, 'active', $7, $8)`,
			NewcomerCampaignKey, userID, configuredTier.Key, factor, startsAt, expiresAt, grantedBy, strings.TrimSpace(input.Reason))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("set newcomer campaign manual membership: %w", err)
	}
	s.invalidateMembershipCache(ctx, userID)
	return s.AdminGetUserMembership(ctx, userID)
}

func (s *NewcomerCampaignService) AdminClearUserMembership(ctx context.Context, actorID, userID int64) (*NewcomerCampaignAdminUserMembership, error) {
	if !s.enabled() {
		return nil, errors.New("newcomer campaign service is unavailable")
	}
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	var revokedBy any
	if actorID > 0 {
		revokedBy = actorID
	}
	now := s.currentTime()
	result, err := s.db.ExecContext(ctx, `
UPDATE newcomer_campaign_admin_memberships
SET status = 'revoked', revoked_at = $3, revoked_by = $4,
    revoke_reason = 'cleared by administrator', updated_at = $3
WHERE campaign_key = $1 AND user_id = $2 AND status = 'active'`, NewcomerCampaignKey, userID, now, revokedBy)
	if err != nil {
		return nil, fmt.Errorf("clear newcomer campaign manual membership: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected > 0 {
		s.invalidateMembershipCache(ctx, userID)
	}
	return s.AdminGetUserMembership(ctx, userID)
}

func (s *NewcomerCampaignService) loadManualMembership(ctx context.Context, userID int64, now time.Time) (*NewcomerMembershipStatus, error) {
	var membership NewcomerMembershipStatus
	err := s.queryOne(ctx, s.db, `
SELECT tier_key, factor, starts_at, expires_at
FROM newcomer_campaign_admin_memberships
WHERE campaign_key = $1 AND user_id = $2 AND status = 'active'
ORDER BY id DESC LIMIT 1`, []any{NewcomerCampaignKey, userID}, &membership.TierKey, &membership.Factor, &membership.StartsAt, &membership.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load newcomer campaign manual membership: %w", err)
	}
	membership.TierName = tierName(membership.TierKey)
	if !membership.ExpiresAt.After(now) {
		return nil, nil
	}
	return &membership, nil
}

func (s *NewcomerCampaignService) loadEffectiveMembership(ctx context.Context, userID int64, now time.Time, validInviteCount int) (*NewcomerMembershipStatus, error) {
	var membership NewcomerMembershipStatus
	err := s.queryOne(ctx, s.db, `
SELECT tier_key, factor, starts_at, expires_at
FROM (
    SELECT m.tier_key, m.factor, m.starts_at, m.expires_at,
           1 AS priority, 2147483647 AS sort_threshold
    FROM newcomer_campaign_admin_memberships m
    WHERE m.campaign_key = $1 AND m.user_id = $2 AND m.status = 'active'
      AND m.starts_at <= $3 AND m.expires_at > $3
    UNION ALL
    SELECT g.tier_key, g.factor, g.starts_at, g.expires_at,
           0 AS priority, g.threshold AS sort_threshold
    FROM newcomer_campaign_membership_grants g
    WHERE g.campaign_key = $1 AND g.user_id = $2 AND g.status = 'active'
      AND g.starts_at <= $3 AND g.expires_at > $3 AND g.threshold <= $4
) effective
ORDER BY priority DESC, sort_threshold DESC, expires_at DESC
LIMIT 1`, []any{NewcomerCampaignKey, userID, now, validInviteCount}, &membership.TierKey, &membership.Factor, &membership.StartsAt, &membership.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load effective newcomer campaign membership: %w", err)
	}
	membership.TierName = tierName(membership.TierKey)
	return &membership, nil
}

func (s *NewcomerCampaignService) invalidateMembershipCache(ctx context.Context, userID int64) {
	s.invalidateMembershipFactor(userID)
	if s != nil && s.authCacheInvalidator != nil && userID > 0 {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
}
