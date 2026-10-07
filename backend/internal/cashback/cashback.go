package cashback

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cashi/cashi/backend/internal/domain"
	"github.com/cashi/cashi/backend/internal/postgres"
	"github.com/cashi/cashi/backend/internal/redisstore"
)

type Service struct {
	db    *postgres.Store
	redis *redisstore.Store
	loc   *time.Location
}

func New(db *postgres.Store, redis *redisstore.Store) (*Service, error) {
	loc, err := time.LoadLocation(domain.CampaignTZ)
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}
	return &Service{db: db, redis: redis, loc: loc}, nil
}

func (s *Service) CampaignDay(now time.Time) string {
	return now.In(s.loc).Format("2006-01-02")
}

func (s *Service) GetCampaign(ctx context.Context) (domain.Campaign, error) {
	return s.db.GetCampaign(ctx, domain.CampaignIDFlash)
}

func (s *Service) GetSummary(ctx context.Context, userID string) (domain.CashbackSummary, error) {
	if err := s.requireUser(ctx, userID); err != nil {
		return domain.CashbackSummary{}, err
	}
	return s.db.GetCashbackSummary(ctx, userID, s.CampaignDay(time.Now()))
}

func (s *Service) ListLedger(ctx context.Context, userID string) ([]domain.LedgerEntry, error) {
	if err := s.requireUser(ctx, userID); err != nil {
		return nil, err
	}
	return s.db.ListLedger(ctx, userID, 50)
}

func (s *Service) Pay(ctx context.Context, userID string, amountIDR int64, idempotencyKey string) (domain.PaymentResult, error) {
	if userID == "" {
		return domain.PaymentResult{}, domain.ErrMissingUser
	}
	if idempotencyKey == "" {
		return domain.PaymentResult{}, domain.ErrMissingIdempotency
	}
	if amountIDR <= 0 {
		return domain.PaymentResult{}, domain.ErrInvalidAmount
	}

	if result, ok, err := s.db.GetIdempotentPayment(ctx, userID, idempotencyKey); err != nil {
		return domain.PaymentResult{}, err
	} else if ok {
		return result, nil
	}

	locked, err := s.redis.TryLock(ctx, "payment", userID, idempotencyKey, 30*time.Second)
	if err != nil {
		return domain.PaymentResult{}, fmt.Errorf("redis lock: %w", err)
	}
	if !locked {
		// Another in-flight request; wait briefly then return stored result or retry once via DB unique.
		time.Sleep(50 * time.Millisecond)
		if result, ok, err := s.db.GetIdempotentPayment(ctx, userID, idempotencyKey); err != nil {
			return domain.PaymentResult{}, err
		} else if ok {
			return result, nil
		}
		return domain.PaymentResult{}, fmt.Errorf("payment in progress for this idempotency key; retry")
	}
	defer func() { _ = s.redis.Unlock(ctx, "payment", userID, idempotencyKey) }()

	// Re-check after lock.
	if result, ok, err := s.db.GetIdempotentPayment(ctx, userID, idempotencyKey); err != nil {
		return domain.PaymentResult{}, err
	} else if ok {
		return result, nil
	}

	return s.db.Earn(ctx, postgres.EarnParams{
		UserID:         userID,
		AmountIDR:      amountIDR,
		IdempotencyKey: idempotencyKey,
		Day:            s.CampaignDay(time.Now()),
	})
}

func (s *Service) Redeem(ctx context.Context, userID string, amountIDR int64, idempotencyKey string) (domain.RedeemResult, error) {
	if userID == "" {
		return domain.RedeemResult{}, domain.ErrMissingUser
	}
	if idempotencyKey == "" {
		return domain.RedeemResult{}, domain.ErrMissingIdempotency
	}
	if amountIDR <= 0 {
		return domain.RedeemResult{}, domain.ErrInvalidAmount
	}

	if result, ok, err := s.db.GetIdempotentRedeem(ctx, userID, idempotencyKey); err != nil {
		return domain.RedeemResult{}, err
	} else if ok {
		return result, nil
	}

	locked, err := s.redis.TryLock(ctx, "redeem", userID, idempotencyKey, 30*time.Second)
	if err != nil {
		return domain.RedeemResult{}, fmt.Errorf("redis lock: %w", err)
	}
	if !locked {
		time.Sleep(50 * time.Millisecond)
		if result, ok, err := s.db.GetIdempotentRedeem(ctx, userID, idempotencyKey); err != nil {
			return domain.RedeemResult{}, err
		} else if ok {
			return result, nil
		}
		return domain.RedeemResult{}, fmt.Errorf("redeem in progress for this idempotency key; retry")
	}
	defer func() { _ = s.redis.Unlock(ctx, "redeem", userID, idempotencyKey) }()

	if result, ok, err := s.db.GetIdempotentRedeem(ctx, userID, idempotencyKey); err != nil {
		return domain.RedeemResult{}, err
	} else if ok {
		return result, nil
	}

	return s.db.Redeem(ctx, postgres.RedeemParams{
		UserID:         userID,
		AmountIDR:      amountIDR,
		IdempotencyKey: idempotencyKey,
	})
}

func (s *Service) requireUser(ctx context.Context, userID string) error {
	if userID == "" {
		return domain.ErrMissingUser
	}
	ok, err := s.db.UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrUnknownUser
	}
	return nil
}

func IsClientError(err error) bool {
	return errors.Is(err, domain.ErrInvalidAmount) ||
		errors.Is(err, domain.ErrMissingUser) ||
		errors.Is(err, domain.ErrUnknownUser) ||
		errors.Is(err, domain.ErrMissingIdempotency) ||
		errors.Is(err, domain.ErrInsufficientFunds) ||
		errors.Is(err, domain.ErrCampaignMissing)
}
