package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cashi/cashi/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) UserExists(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists)
	return exists, err
}

func (s *Store) GetCampaign(ctx context.Context, id string) (domain.Campaign, error) {
	var c domain.Campaign
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, rate_bps, min_payment_idr, daily_cap_idr,
		       budget_total_idr, budget_spent_idr, status, timezone
		FROM campaigns WHERE id = $1
	`, id).Scan(
		&c.ID, &c.Name, &c.RateBPS, &c.MinPaymentIDR, &c.DailyCapIDR,
		&c.BudgetTotalIDR, &c.BudgetSpentIDR, &c.Status, &c.Timezone,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Campaign{}, domain.ErrCampaignMissing
	}
	if err != nil {
		return domain.Campaign{}, err
	}
	c.BudgetLeftIDR = c.BudgetTotalIDR - c.BudgetSpentIDR
	return c, nil
}

func (s *Store) GetCashbackSummary(ctx context.Context, userID string, day string) (domain.CashbackSummary, error) {
	var available, redeemed int64
	err := s.pool.QueryRow(ctx, `
		SELECT available_idr, redeemed_idr FROM wallets WHERE user_id = $1
	`, userID).Scan(&available, &redeemed)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CashbackSummary{}, domain.ErrUnknownUser
	}
	if err != nil {
		return domain.CashbackSummary{}, err
	}

	var earnedToday int64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(earned_idr, 0) FROM user_daily_earnings
		WHERE user_id = $1 AND day = $2
	`, userID, day).Scan(&earnedToday)
	if errors.Is(err, pgx.ErrNoRows) {
		earnedToday = 0
		err = nil
	}
	if err != nil {
		return domain.CashbackSummary{}, err
	}

	dailyLeft := domain.DailyCapIDR - earnedToday
	if dailyLeft < 0 {
		dailyLeft = 0
	}
	return domain.CashbackSummary{
		UserID:         userID,
		AvailableIDR:   available,
		RedeemedIDR:    redeemed,
		EarnedTodayIDR: earnedToday,
		DailyCapIDR:    domain.DailyCapIDR,
		DailyLeftIDR:   dailyLeft,
	}, nil
}

func (s *Store) ListLedger(ctx context.Context, userID string, limit int) ([]domain.LedgerEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, entry_type, amount_idr,
		       payment_id::text, redemption_id::text, created_at
		FROM ledger_entries
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.LedgerEntry
	for rows.Next() {
		var e domain.LedgerEntry
		var paymentID, redemptionID *string
		if err := rows.Scan(&e.ID, &e.EntryType, &e.AmountIDR, &paymentID, &redemptionID, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.PaymentID = paymentID
		e.RedemptionID = redemptionID
		out = append(out, e)
	}
	if out == nil {
		out = []domain.LedgerEntry{}
	}
	return out, rows.Err()
}

type EarnParams struct {
	UserID         string
	AmountIDR      int64
	IdempotencyKey string
	// Day is YYYY-MM-DD in the campaign timezone (Asia/Jakarta).
	Day string
}

func (s *Store) Earn(ctx context.Context, p EarnParams) (domain.PaymentResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PaymentResult{}, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, p.UserID).Scan(&exists); err != nil {
		return domain.PaymentResult{}, err
	}
	if !exists {
		return domain.PaymentResult{}, domain.ErrUnknownUser
	}

	// Serialize concurrent earns against the same campaign budget.
	var campID string
	var rateBPS int
	var minPayment, dailyCap, budgetTotal, budgetSpent int64
	var status domain.CampaignStatus
	err = tx.QueryRow(ctx, `
		SELECT id, rate_bps, min_payment_idr, daily_cap_idr,
		       budget_total_idr, budget_spent_idr, status
		FROM campaigns WHERE id = $1 FOR UPDATE
	`, domain.CampaignIDFlash).Scan(
		&campID, &rateBPS, &minPayment, &dailyCap, &budgetTotal, &budgetSpent, &status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PaymentResult{}, domain.ErrCampaignMissing
	}
	if err != nil {
		return domain.PaymentResult{}, err
	}

	// Ensure daily row exists, then lock it.
	_, err = tx.Exec(ctx, `
		INSERT INTO user_daily_earnings (user_id, day, earned_idr)
		VALUES ($1, $2, 0)
		ON CONFLICT (user_id, day) DO NOTHING
	`, p.UserID, p.Day)
	if err != nil {
		return domain.PaymentResult{}, err
	}
	var earnedToday int64
	if err := tx.QueryRow(ctx, `
		SELECT earned_idr FROM user_daily_earnings
		WHERE user_id = $1 AND day = $2 FOR UPDATE
	`, p.UserID, p.Day).Scan(&earnedToday); err != nil {
		return domain.PaymentResult{}, err
	}

	decision := domain.ComputeAward(domain.AwardInput{
		AmountIDR:      p.AmountIDR,
		MinPaymentIDR:  minPayment,
		RateBPS:        rateBPS,
		DailyCapIDR:    dailyCap,
		EarnedTodayIDR: earnedToday,
		BudgetTotalIDR: budgetTotal,
		BudgetSpentIDR: budgetSpent,
		CampaignStatus: status,
	})
	award := decision.CashbackIDR
	reason := decision.Reason

	paymentID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO payments (id, user_id, amount_idr, cashback_idr, award_reason, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, paymentID, p.UserID, p.AmountIDR, award, string(reason), p.IdempotencyKey)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			_ = tx.Rollback(ctx)
			result, ok, getErr := s.GetIdempotentPayment(ctx, p.UserID, p.IdempotencyKey)
			if getErr != nil {
				return domain.PaymentResult{}, getErr
			}
			if ok {
				return result, nil
			}
			return domain.PaymentResult{}, fmt.Errorf("duplicate payment without stored idempotency response")
		}
		return domain.PaymentResult{}, err
	}

	if award > 0 {
		ledgerID := uuid.New()
		_, err = tx.Exec(ctx, `
			INSERT INTO ledger_entries (id, user_id, entry_type, amount_idr, payment_id)
			VALUES ($1, $2, 'cashback_credit', $3, $4)
		`, ledgerID, p.UserID, award, paymentID)
		if err != nil {
			return domain.PaymentResult{}, err
		}
		_, err = tx.Exec(ctx, `
			UPDATE wallets
			SET available_idr = available_idr + $2, updated_at = now()
			WHERE user_id = $1
		`, p.UserID, award)
		if err != nil {
			return domain.PaymentResult{}, err
		}
		_, err = tx.Exec(ctx, `
			UPDATE user_daily_earnings
			SET earned_idr = earned_idr + $3
			WHERE user_id = $1 AND day = $2
		`, p.UserID, p.Day, award)
		if err != nil {
			return domain.PaymentResult{}, err
		}
		_, err = tx.Exec(ctx, `
			UPDATE campaigns
			SET budget_spent_idr = budget_spent_idr + $2,
			    status = CASE
			      WHEN budget_spent_idr + $2 >= budget_total_idr THEN $3
			      ELSE status
			    END
			WHERE id = $1
		`, campID, award, domain.CampaignStatusExhausted)
		if err != nil {
			return domain.PaymentResult{}, err
		}
		budgetSpent += award
		earnedToday += award
	}

	var available int64
	if err := tx.QueryRow(ctx, `SELECT available_idr FROM wallets WHERE user_id = $1`, p.UserID).Scan(&available); err != nil {
		return domain.PaymentResult{}, err
	}

	result := domain.PaymentResult{
		PaymentID:      paymentID.String(),
		UserID:         p.UserID,
		AmountIDR:      p.AmountIDR,
		CashbackIDR:    award,
		AwardReason:    reason,
		AvailableIDR:   available,
		EarnedTodayIDR: earnedToday,
		BudgetLeftIDR:  budgetTotal - budgetSpent,
	}

	if err := persistIdempotency(ctx, tx, "payment", p.UserID, p.IdempotencyKey, result); err != nil {
		return domain.PaymentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PaymentResult{}, err
	}
	return result, nil
}

type RedeemParams struct {
	UserID         string
	AmountIDR      int64
	IdempotencyKey string
}

func (s *Store) Redeem(ctx context.Context, p RedeemParams) (domain.RedeemResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.RedeemResult{}, err
	}
	defer tx.Rollback(ctx)

	var available, redeemed int64
	err = tx.QueryRow(ctx, `
		SELECT available_idr, redeemed_idr FROM wallets WHERE user_id = $1 FOR UPDATE
	`, p.UserID).Scan(&available, &redeemed)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RedeemResult{}, domain.ErrUnknownUser
	}
	if err != nil {
		return domain.RedeemResult{}, err
	}
	if p.AmountIDR > available {
		return domain.RedeemResult{}, domain.ErrInsufficientFunds
	}

	redemptionID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO redemptions (id, user_id, amount_idr, idempotency_key)
		VALUES ($1, $2, $3, $4)
	`, redemptionID, p.UserID, p.AmountIDR, p.IdempotencyKey)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			_ = tx.Rollback(ctx)
			result, ok, getErr := s.GetIdempotentRedeem(ctx, p.UserID, p.IdempotencyKey)
			if getErr != nil {
				return domain.RedeemResult{}, getErr
			}
			if ok {
				return result, nil
			}
			return domain.RedeemResult{}, fmt.Errorf("duplicate redeem without stored idempotency response")
		}
		return domain.RedeemResult{}, err
	}

	ledgerID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO ledger_entries (id, user_id, entry_type, amount_idr, redemption_id)
		VALUES ($1, $2, 'redeem_debit', $3, $4)
	`, ledgerID, p.UserID, p.AmountIDR, redemptionID)
	if err != nil {
		return domain.RedeemResult{}, err
	}

	available -= p.AmountIDR
	redeemed += p.AmountIDR
	_, err = tx.Exec(ctx, `
		UPDATE wallets
		SET available_idr = $2, redeemed_idr = $3, updated_at = now()
		WHERE user_id = $1
	`, p.UserID, available, redeemed)
	if err != nil {
		return domain.RedeemResult{}, err
	}

	result := domain.RedeemResult{
		RedemptionID: redemptionID.String(),
		UserID:       p.UserID,
		AmountIDR:    p.AmountIDR,
		AvailableIDR: available,
		RedeemedIDR:  redeemed,
	}
	if err := persistIdempotency(ctx, tx, "redeem", p.UserID, p.IdempotencyKey, result); err != nil {
		return domain.RedeemResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RedeemResult{}, err
	}
	return result, nil
}

func persistIdempotency(ctx context.Context, tx pgx.Tx, scope, userID, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO idempotency_records (scope, user_id, idempotency_key, response_json)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING
	`, scope, userID, key, raw)
	return err
}

func (s *Store) GetIdempotentPayment(ctx context.Context, userID, key string) (domain.PaymentResult, bool, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT response_json FROM idempotency_records
		WHERE scope = 'payment' AND user_id = $1 AND idempotency_key = $2
	`, userID, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PaymentResult{}, false, nil
	}
	if err != nil {
		return domain.PaymentResult{}, false, err
	}
	var result domain.PaymentResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return domain.PaymentResult{}, false, err
	}
	result.IdempotentReplay = true
	return result, true, nil
}

func (s *Store) GetIdempotentRedeem(ctx context.Context, userID, key string) (domain.RedeemResult, bool, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT response_json FROM idempotency_records
		WHERE scope = 'redeem' AND user_id = $1 AND idempotency_key = $2
	`, userID, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RedeemResult{}, false, nil
	}
	if err != nil {
		return domain.RedeemResult{}, false, err
	}
	var result domain.RedeemResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return domain.RedeemResult{}, false, err
	}
	result.IdempotentReplay = true
	return result, true, nil
}
