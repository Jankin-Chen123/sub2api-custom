package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type checkinRepository struct {
	client *dbent.Client
}

func NewCheckinRepository(client *dbent.Client) *checkinRepository {
	return &checkinRepository{client: client}
}

func (r *checkinRepository) ListCheckinPrizes(ctx context.Context) ([]service.CheckinPrize, error) {
	return listCheckinPrizes(ctx, r.client, false)
}

func listCheckinPrizes(ctx context.Context, client *dbent.Client, lock bool) ([]service.CheckinPrize, error) {
	query := `
		SELECT id, name, amount::double precision, probability::double precision, color, sort_order
		FROM daily_checkin_prizes
		ORDER BY sort_order ASC, id ASC`
	if lock {
		query += " FOR SHARE"
	}
	rows, err := client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	prizes := make([]service.CheckinPrize, 0)
	for rows.Next() {
		var prize service.CheckinPrize
		if err := rows.Scan(&prize.ID, &prize.Name, &prize.Amount, &prize.Probability, &prize.Color, &prize.SortOrder); err != nil {
			return nil, err
		}
		prizes = append(prizes, prize)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return prizes, nil
}

func (r *checkinRepository) GetCheckinByDate(ctx context.Context, userID int64, date string) (*service.CheckinRecord, error) {
	rows, err := r.client.QueryContext(ctx, `
		SELECT id, prize_id, prize_name, amount::double precision,
		       bonus_amount::double precision, streak_days,
		       probability::double precision, created_at
		FROM daily_checkins
		WHERE user_id = $1 AND checkin_date = $2
		LIMIT 1`, userID, date)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	return scanCheckinRecord(rows)
}

// GetConsecutiveCheckinDays returns the active streak ending today or yesterday.
// A gap of even one day intentionally resets the streak to zero.
func (r *checkinRepository) GetConsecutiveCheckinDays(ctx context.Context, userID int64, date string) (int, error) {
	return getConsecutiveCheckinDays(ctx, r.client, userID, date)
}

func getConsecutiveCheckinDays(ctx context.Context, client *dbent.Client, userID int64, date string) (int, error) {
	var streak int
	rows, err := client.QueryContext(ctx, `
		WITH RECURSIVE latest AS (
			SELECT checkin_date
			FROM daily_checkins
			WHERE user_id = $1
			  AND checkin_date BETWEEN ($2::date - INTERVAL '1 day')::date AND $2::date
			ORDER BY checkin_date DESC
			LIMIT 1
		), consecutive AS (
			SELECT checkin_date FROM latest
			UNION ALL
			SELECT d.checkin_date
			FROM daily_checkins d
			JOIN consecutive c
			  ON d.user_id = $1
				 AND d.checkin_date = (c.checkin_date - INTERVAL '1 day')::date
		)
		SELECT COUNT(*) FROM consecutive`, userID, date)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	if err := rows.Scan(&streak); err != nil {
		return 0, err
	}
	return streak, rows.Err()
}

func (r *checkinRepository) ListCheckinsByUser(ctx context.Context, userID int64, limit int) ([]service.CheckinRecord, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := r.client.QueryContext(ctx, `
		SELECT id, prize_id, prize_name, amount::double precision,
		       bonus_amount::double precision, streak_days,
		       probability::double precision, created_at
		FROM daily_checkins
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	checkins := make([]service.CheckinRecord, 0)
	for rows.Next() {
		record, err := scanCheckinRecord(rows)
		if err != nil {
			return nil, err
		}
		checkins = append(checkins, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return checkins, nil
}

func (r *checkinRepository) DrawCheckin(ctx context.Context, userID int64, date string, randomUnit float64, streakBonusAmount float64) (*service.CheckinRecord, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	prizes, err := listCheckinPrizes(ctx, txClient, true)
	if err != nil {
		return nil, err
	}
	prize, err := service.SelectCheckinPrize(prizes, randomUnit)
	if err != nil {
		return nil, err
	}
	priorStreak, err := getConsecutiveCheckinDays(ctx, txClient, userID, date)
	if err != nil {
		return nil, err
	}
	streakDays := priorStreak + 1
	bonusAmount := 0.0
	if streakDays%7 == 0 && streakBonusAmount > 0 {
		bonusAmount = streakBonusAmount
	}

	createdAt := time.Now().UTC()
	rows, err := txClient.QueryContext(ctx, `
		INSERT INTO daily_checkins
			(user_id, checkin_date, prize_id, prize_name, amount, bonus_amount, streak_days, probability, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`,
		userID, date, prize.ID, prize.Name, prize.Amount, bonusAmount, streakDays, prize.Probability, createdAt)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return nil, service.ErrCheckinAlreadyClaimed
		}
		return nil, err
	}
	var recordID int64
	if !rows.Next() {
		_ = rows.Close()
		if rowsErr := rows.Err(); rowsErr != nil {
			if isUniqueConstraintViolation(rowsErr) {
				return nil, service.ErrCheckinAlreadyClaimed
			}
			return nil, rowsErr
		}
		return nil, errorsNewNoCheckinInsert()
	}
	if err := rows.Scan(&recordID, &createdAt); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	balanceRows, err := txClient.QueryContext(ctx, `
		UPDATE users
		SET balance = balance + $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance::double precision`, prize.Amount+bonusAmount, userID)
	if err != nil {
		return nil, err
	}
	if !balanceRows.Next() {
		_ = balanceRows.Close()
		if rowsErr := balanceRows.Err(); rowsErr != nil {
			return nil, rowsErr
		}
		return nil, service.ErrUserNotFound
	}
	var newBalance float64
	if err := balanceRows.Scan(&newBalance); err != nil {
		_ = balanceRows.Close()
		return nil, err
	}
	if err := balanceRows.Close(); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	prizeID := prize.ID
	return &service.CheckinRecord{
		ID:          recordID,
		PrizeID:     &prizeID,
		PrizeName:   prize.Name,
		Amount:      prize.Amount,
		BonusAmount: bonusAmount,
		TotalAmount: prize.Amount + bonusAmount,
		Probability: prize.Probability,
		NewBalance:  newBalance,
		StreakDays:  streakDays,
		CheckedAt:   createdAt,
	}, nil
}

func (r *checkinRepository) ReplaceCheckinPrizes(ctx context.Context, prizes []service.CheckinPrize) ([]service.CheckinPrize, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()
	if _, err := txClient.ExecContext(ctx, `DELETE FROM daily_checkin_prizes`); err != nil {
		return nil, err
	}

	created := make([]service.CheckinPrize, 0, len(prizes))
	for i := range prizes {
		prize := prizes[i]
		rows, err := txClient.QueryContext(ctx, `
			INSERT INTO daily_checkin_prizes
				(name, amount, probability, color, sort_order, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			RETURNING id`, prize.Name, prize.Amount, prize.Probability, prize.Color, i)
		if err != nil {
			return nil, err
		}
		if !rows.Next() {
			_ = rows.Close()
			if rowsErr := rows.Err(); rowsErr != nil {
				return nil, rowsErr
			}
			return nil, errorsNewNoCheckinInsert()
		}
		if err := rows.Scan(&prize.ID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		prize.SortOrder = i
		created = append(created, prize)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return created, nil
}

type checkinScanner interface {
	Scan(dest ...any) error
}

func scanCheckinRecord(scanner checkinScanner) (*service.CheckinRecord, error) {
	var (
		record  service.CheckinRecord
		prizeID sql.NullInt64
	)
	if err := scanner.Scan(
		&record.ID,
		&prizeID,
		&record.PrizeName,
		&record.Amount,
		&record.BonusAmount,
		&record.StreakDays,
		&record.Probability,
		&record.CheckedAt,
	); err != nil {
		return nil, err
	}
	if prizeID.Valid {
		record.PrizeID = &prizeID.Int64
	}
	record.TotalAmount = record.Amount + record.BonusAmount
	return &record, nil
}

func errorsNewNoCheckinInsert() error {
	return fmt.Errorf("check-in insert returned no row")
}
