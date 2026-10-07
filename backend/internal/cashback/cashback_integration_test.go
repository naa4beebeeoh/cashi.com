package cashback_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cashi/cashi/backend/internal/cashback"
	"github.com/cashi/cashi/backend/internal/domain"
	"github.com/cashi/cashi/backend/internal/postgres"
	"github.com/cashi/cashi/backend/internal/redisstore"
	"github.com/google/uuid"
)

func testService(t *testing.T) *cashback.Service {
	t.Helper()
	if os.Getenv("RUN_INTEGRATION") == "" {
		t.Skip("set RUN_INTEGRATION=1 with docker compose up")
	}
	ctx := context.Background()
	dbURL := env("DATABASE_URL", "postgres://cashi:cashi@localhost:5432/cashi?sslmode=disable")
	redisURL := env("REDIS_URL", "redis://localhost:6379/0")

	db, err := postgres.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	t.Cleanup(db.Close)

	rdb, err := redisstore.Connect(ctx, redisURL)
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	svc, err := cashback.New(db, rdb)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func TestPayRulesAndIdempotency(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	user := "user_a"

	below, err := svc.Pay(ctx, user, 19_999, "test-below-"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if below.CashbackIDR != 0 || below.AwardReason != domain.ReasonBelowMinimum {
		t.Fatalf("below min: %+v", below)
	}

	key := "test-pay-" + uuid.NewString()
	first, err := svc.Pay(ctx, user, 100_000, key)
	if err != nil {
		t.Fatal(err)
	}
	if first.CashbackIDR != 5_000 {
		t.Fatalf("want 5000 cashback, got %d", first.CashbackIDR)
	}
	second, err := svc.Pay(ctx, user, 100_000, key)
	if err != nil {
		t.Fatal(err)
	}
	if !second.IdempotentReplay || second.PaymentID != first.PaymentID || second.CashbackIDR != first.CashbackIDR {
		t.Fatalf("idempotent replay mismatch: first=%+v second=%+v", first, second)
	}
}

func TestDailyCap(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	user := "user_b"

	// Large uncapped earn would be 50_000+; force toward daily cap with multiple pays.
	var total int64
	for i := 0; i < 20; i++ {
		res, err := svc.Pay(ctx, user, 200_000, fmt.Sprintf("daily-%s-%d", uuid.NewString(), i))
		if err != nil {
			t.Fatal(err)
		}
		total += res.CashbackIDR
	}
	if total > domain.DailyCapIDR {
		t.Fatalf("earned %d over daily cap %d", total, domain.DailyCapIDR)
	}
	summary, err := svc.GetSummary(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EarnedTodayIDR > domain.DailyCapIDR {
		t.Fatalf("summary earned today %d", summary.EarnedTodayIDR)
	}
}

func TestConcurrentBudgetDoesNotOverspend(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()

	// This test is informational against shared demo DB; skip if budget already low.
	camp, err := svc.GetCampaign(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if camp.BudgetLeftIDR < 100_000 {
		t.Skip("campaign budget too low for concurrency check on shared DB")
	}

	const workers = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	var awarded int64
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			res, err := svc.Pay(ctx, "user_a", 100_000, fmt.Sprintf("race-%d-%s", i, uuid.NewString()))
			if err != nil {
				t.Errorf("pay: %v", err)
				return
			}
			mu.Lock()
			awarded += res.CashbackIDR
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	camp, err = svc.GetCampaign(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if camp.BudgetSpentIDR > camp.BudgetTotalIDR {
		t.Fatalf("overspent: spent=%d total=%d awarded_batch=%d", camp.BudgetSpentIDR, camp.BudgetTotalIDR, awarded)
	}
	_ = time.Now()
}
