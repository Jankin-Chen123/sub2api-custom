package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// InsertAccountProbeResults stores per-account observations separately from
// channel_monitor_histories, keeping the existing one-row-per-model trend
// contract intact.
func (r *channelMonitorRepository) InsertAccountProbeResults(ctx context.Context, rows []*service.ChannelMonitorAccountProbeResult) error {
	if len(rows) == 0 {
		return nil
	}
	if r.db == nil {
		return fmt.Errorf("channel monitor database is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin account probe result transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const insertSQL = `
		INSERT INTO channel_monitor_account_probe_results
			(monitor_id, group_id, account_id, model, provider, status, latency_ms,
			 checked_at, message, skipped, skip_reason, round_duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("prepare account probe result insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, row := range rows {
		if row == nil {
			continue
		}
		var latency any
		if row.LatencyMs != nil {
			latency = *row.LatencyMs
		}
		if _, err := stmt.ExecContext(ctx,
			row.MonitorID,
			row.GroupID,
			row.AccountID,
			row.Model,
			row.Provider,
			row.Status,
			latency,
			row.CheckedAt,
			row.Message,
			row.Skipped,
			row.SkipReason,
			row.RoundDurationMs,
		); err != nil {
			return fmt.Errorf("insert account probe result: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit account probe results: %w", err)
	}
	return nil
}

// ApplyAccountProbeResults persists observations and advances the latest
// account health snapshots in one transaction. Snapshot computation remains a
// pure service function, while SELECT ... FOR UPDATE serializes concurrent
// monitor cycles for the same account/model tuple.
func (r *channelMonitorRepository) ApplyAccountProbeResults(ctx context.Context, rows []*service.ChannelMonitorAccountProbeResult) ([]*service.ChannelMonitorAccountHealthSnapshot, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	if r.db == nil {
		return nil, fmt.Errorf("channel monitor database is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin account probe health transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const insertSQL = `
		INSERT INTO channel_monitor_account_probe_results
			(monitor_id, group_id, account_id, model, provider, status, latency_ms,
			 checked_at, message, skipped, skip_reason, round_duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return nil, fmt.Errorf("prepare account probe health insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	const loadSQL = `
		SELECT group_id, account_id, provider, model, score, health_state,
		       ewma_success_rate, ewma_latency_ms, sample_count,
		       consecutive_successes, consecutive_failures, last_status,
		       last_probe_at, updated_at, expires_at
		FROM channel_monitor_account_health_snapshots
		WHERE group_id = $1 AND account_id = $2 AND provider = $3 AND model = $4
		FOR UPDATE`
	const upsertSQL = `
		INSERT INTO channel_monitor_account_health_snapshots
			(group_id, account_id, provider, model, score, health_state,
			 ewma_success_rate, ewma_latency_ms, sample_count,
			 consecutive_successes, consecutive_failures, last_status,
			 last_probe_at, updated_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (group_id, account_id, provider, model) DO UPDATE SET
			score = EXCLUDED.score,
			health_state = EXCLUDED.health_state,
			ewma_success_rate = EXCLUDED.ewma_success_rate,
			ewma_latency_ms = EXCLUDED.ewma_latency_ms,
			sample_count = EXCLUDED.sample_count,
			consecutive_successes = EXCLUDED.consecutive_successes,
			consecutive_failures = EXCLUDED.consecutive_failures,
			last_status = EXCLUDED.last_status,
			last_probe_at = EXCLUDED.last_probe_at,
			updated_at = EXCLUDED.updated_at,
			expires_at = EXCLUDED.expires_at`

	type snapshotKey struct {
		groupID, accountID int64
		provider, model    string
	}
	snapshots := make(map[snapshotKey]*service.ChannelMonitorAccountHealthSnapshot)
	for _, row := range rows {
		if row == nil {
			continue
		}
		var latency any
		if row.LatencyMs != nil {
			latency = *row.LatencyMs
		}
		if _, err := stmt.ExecContext(ctx,
			row.MonitorID,
			row.GroupID,
			row.AccountID,
			row.Model,
			row.Provider,
			row.Status,
			latency,
			row.CheckedAt,
			row.Message,
			row.Skipped,
			row.SkipReason,
			row.RoundDurationMs,
		); err != nil {
			return nil, fmt.Errorf("insert account probe health result: %w", err)
		}
		if row.Skipped || row.Status == "skipped" {
			continue
		}

		var previous service.ChannelMonitorAccountHealthSnapshot
		var ewmaLatency sql.NullInt64
		var lastProbeAt, updatedAt, expiresAt time.Time
		err := tx.QueryRowContext(ctx, loadSQL, row.GroupID, row.AccountID, strings.ToLower(strings.TrimSpace(row.Provider)), strings.TrimSpace(row.Model)).Scan(
			&previous.GroupID,
			&previous.AccountID,
			&previous.Provider,
			&previous.Model,
			&previous.Score,
			&previous.HealthState,
			&previous.EWMASuccessRate,
			&ewmaLatency,
			&previous.SampleCount,
			&previous.ConsecutiveSuccesses,
			&previous.ConsecutiveFailures,
			&previous.LastStatus,
			&lastProbeAt,
			&updatedAt,
			&expiresAt,
		)
		var previousPtr *service.ChannelMonitorAccountHealthSnapshot
		if err == nil {
			previous.EWMALatencyMs = nullIntPointer(ewmaLatency)
			previous.LastProbeAt = lastProbeAt
			previous.UpdatedAt = updatedAt
			previous.ExpiresAt = expiresAt
			previousPtr = &previous
		} else if err != sql.ErrNoRows {
			return nil, fmt.Errorf("load account health snapshot: %w", err)
		}

		next := service.AdvanceChannelMonitorAccountHealthSnapshot(previousPtr, row, time.Now())
		var nextLatency any
		if next.EWMALatencyMs != nil {
			nextLatency = *next.EWMALatencyMs
		}
		if _, err := tx.ExecContext(ctx, upsertSQL,
			next.GroupID,
			next.AccountID,
			next.Provider,
			next.Model,
			next.Score,
			next.HealthState,
			next.EWMASuccessRate,
			nextLatency,
			next.SampleCount,
			next.ConsecutiveSuccesses,
			next.ConsecutiveFailures,
			next.LastStatus,
			next.LastProbeAt,
			next.UpdatedAt,
			next.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("upsert account health snapshot: %w", err)
		}
		snapshots[snapshotKey{groupID: next.GroupID, accountID: next.AccountID, provider: next.Provider, model: next.Model}] = next
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit account probe health transaction: %w", err)
	}
	out := make([]*service.ChannelMonitorAccountHealthSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		out = append(out, snapshot)
	}
	return out, nil
}

// ListAccountHealthSnapshots returns only latest rows; it never scans the
// account observation history table and is intended for internal/admin views.
func (r *channelMonitorRepository) ListAccountHealthSnapshots(ctx context.Context, groupID *int64, provider, model string, limit int) ([]*service.ChannelMonitorAccountHealthSnapshot, error) {
	if r.db == nil {
		return nil, fmt.Errorf("channel monitor database is not configured")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	const q = `
		SELECT group_id, account_id, provider, model, score, health_state,
		       ewma_success_rate, ewma_latency_ms, sample_count,
		       consecutive_successes, consecutive_failures, last_status,
		       last_probe_at, updated_at, expires_at
		FROM channel_monitor_account_health_snapshots
		WHERE ($1::bigint IS NULL OR group_id = $1)
		  AND ($2 = '' OR provider = $2)
		  AND ($3 = '' OR model = $3)
		ORDER BY updated_at DESC
		LIMIT $4`
	rows, err := r.db.QueryContext(ctx, q, groupID, strings.ToLower(strings.TrimSpace(provider)), strings.TrimSpace(model), limit)
	if err != nil {
		return nil, fmt.Errorf("list account health snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.ChannelMonitorAccountHealthSnapshot, 0)
	for rows.Next() {
		snapshot := &service.ChannelMonitorAccountHealthSnapshot{}
		var ewmaLatency sql.NullInt64
		if err := rows.Scan(
			&snapshot.GroupID,
			&snapshot.AccountID,
			&snapshot.Provider,
			&snapshot.Model,
			&snapshot.Score,
			&snapshot.HealthState,
			&snapshot.EWMASuccessRate,
			&ewmaLatency,
			&snapshot.SampleCount,
			&snapshot.ConsecutiveSuccesses,
			&snapshot.ConsecutiveFailures,
			&snapshot.LastStatus,
			&snapshot.LastProbeAt,
			&snapshot.UpdatedAt,
			&snapshot.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan account health snapshot: %w", err)
		}
		snapshot.EWMALatencyMs = nullIntPointer(ewmaLatency)
		out = append(out, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate account health snapshots: %w", err)
	}
	return out, nil
}

// ListAccountHealthSnapshotsForMonitor scopes the admin view to tuples that
// this monitor has actually probed. Snapshots are shared by group/account/
// provider/model, so the EXISTS clause prevents another monitor in the same
// group from leaking into this view while still retaining deleted accounts.
func (r *channelMonitorRepository) ListAccountHealthSnapshotsForMonitor(ctx context.Context, monitorID int64, provider string, models []string, model string, limit int) ([]*service.ChannelMonitorAccountHealthSnapshot, error) {
	if r.db == nil {
		return nil, fmt.Errorf("channel monitor database is not configured")
	}
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.TrimSpace(model)
	if len(models) == 0 {
		return []*service.ChannelMonitorAccountHealthSnapshot{}, nil
	}
	const q = `
		SELECT s.group_id, s.account_id, COALESCE(a.name, ''), s.provider, s.model,
		       s.score, s.health_state, s.ewma_success_rate, s.ewma_latency_ms,
		       s.sample_count, s.consecutive_successes, s.consecutive_failures,
		       s.last_status, s.last_probe_at, s.updated_at, s.expires_at
		FROM channel_monitor_account_health_snapshots s
		LEFT JOIN accounts a ON a.id = s.account_id
		WHERE s.provider = $2
		  AND s.model = ANY($3)
		  AND ($4 = '' OR s.model = $4)
		  AND EXISTS (
				SELECT 1
				FROM channel_monitor_account_probe_results p
				WHERE p.monitor_id = $1
				  AND p.group_id = s.group_id
				  AND p.account_id = s.account_id
				  AND p.provider = s.provider
				  AND p.model = s.model
			)
		ORDER BY s.model ASC, s.score DESC, s.account_id ASC
		LIMIT $5`
	rows, err := r.db.QueryContext(ctx, q, monitorID, provider, pq.Array(models), model, limit)
	if err != nil {
		return nil, fmt.Errorf("list monitor account health snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.ChannelMonitorAccountHealthSnapshot, 0)
	for rows.Next() {
		snapshot := &service.ChannelMonitorAccountHealthSnapshot{}
		var ewmaLatency sql.NullInt64
		if err := rows.Scan(
			&snapshot.GroupID,
			&snapshot.AccountID,
			&snapshot.AccountName,
			&snapshot.Provider,
			&snapshot.Model,
			&snapshot.Score,
			&snapshot.HealthState,
			&snapshot.EWMASuccessRate,
			&ewmaLatency,
			&snapshot.SampleCount,
			&snapshot.ConsecutiveSuccesses,
			&snapshot.ConsecutiveFailures,
			&snapshot.LastStatus,
			&snapshot.LastProbeAt,
			&snapshot.UpdatedAt,
			&snapshot.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan monitor account health snapshot: %w", err)
		}
		snapshot.EWMALatencyMs = nullIntPointer(ewmaLatency)
		out = append(out, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monitor account health snapshots: %w", err)
	}
	return out, nil
}

func nullIntPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}
