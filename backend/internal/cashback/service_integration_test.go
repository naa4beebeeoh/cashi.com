package cashback_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cashi/cashi/backend/internal/cashback"
	"github.com/cashi/cashi/backend/internal/domain"
	"github.com/cashi/cashi/backend/internal/postgres"
	"github.com/cashi/cashi/backend/internal/redisstore"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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

func dbURL() string {
	return env("DATABASE_URL", "postgres://cashi:cashi@localhost:5432/cashi?sslmode=disable")
}

// freshUser inserts an isolated user+wallet so tests do not inherit demo daily-cap state.
func freshUser(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL())
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)

	id := "itest-" + uuid.NewString()
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, display_name) VALUES ($1, $2)
	`, id, "Integration "+id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1)`, id)
	if err != nil {
		t.Fatalf("insert wallet: %v", err)
	}
	return id
}

func payWithRetry(t *testing.T, svc *cashback.Service, ctx context.Context, user string, amount int64, key string) (domain.PaymentResult, error) {
	t.Helper()
	var last error
	for attempt := 0; attempt < 40; attempt++ {
		res, err := svc.Pay(ctx, user, amount, key)
		if err == nil {
			return res, nil
		}
		last = err
		if strings.Contains(err.Error(), "in progress") {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		return domain.PaymentResult{}, err
	}
	return domain.PaymentResult{}, fmt.Errorf("exhausted retries: %w", last)
}

func TestPayRulesAndIdempotency(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	user := freshUser(t)

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

func TestConcurrentIdempotentPayDoesNotDoubleEarn(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	requireBudgetLeft(t, svc, 5_000)
	user := freshUser(t)
	key := "concurrent-idem-" + uuid.NewString()

	const workers = 16
	var wg sync.WaitGroup
	results := make([]domain.PaymentResult, workers)
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			res, err := payWithRetry(t, svc, ctx, user, 100_000, key)
			results[i] = res
			errs[i] = err
		}(i)
	}
	wg.Wait()

	var paymentID string
	var cashback int64
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
		if i == 0 {
			paymentID = results[i].PaymentID
			cashback = results[i].CashbackIDR
			continue
		}
		if results[i].PaymentID != paymentID || results[i].CashbackIDR != cashback {
			t.Fatalf("worker %d diverged: %+v want payment=%s cashback=%d", i, results[i], paymentID, cashback)
		}
	}
	if cashback != 5_000 {
		t.Fatalf("want single 5000 award, got %d", cashback)
	}

	summary, err := svc.GetSummary(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if summary.AvailableIDR != 5_000 || summary.EarnedTodayIDR != 5_000 {
		t.Fatalf("wallet/daily must reflect one earn: %+v", summary)
	}
}

func requireBudgetLeft(t *testing.T, svc *cashback.Service, need int64) {
	t.Helper()
	camp, err := svc.GetCampaign(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if camp.BudgetLeftIDR < need {
		t.Skipf("campaign budget left %d < %d; reset demo DB or skip", camp.BudgetLeftIDR, need)
	}
}

func TestConcurrentDailyCapDoesNotExceed(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	requireBudgetLeft(t, svc, domain.DailyCapIDR)
	user := freshUser(t)

	// Each 200_000 pay awards 10_000 uncapped; 20 concurrent → must clamp at 50_000.
	const workers = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	var awarded int64
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("daily-race-%d-%s", i, uuid.NewString())
			res, err := svc.Pay(ctx, user, 200_000, key)
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

	if awarded > domain.DailyCapIDR {
		t.Fatalf("concurrent awards %d over daily cap %d", awarded, domain.DailyCapIDR)
	}
	summary, err := svc.GetSummary(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EarnedTodayIDR > domain.DailyCapIDR {
		t.Fatalf("earned today %d over cap", summary.EarnedTodayIDR)
	}
	if summary.EarnedTodayIDR != awarded {
		t.Fatalf("summary earned %d != sum of awards %d", summary.EarnedTodayIDR, awarded)
	}
	if awarded != domain.DailyCapIDR {
		t.Fatalf("expected full daily cap fill under race, got awarded=%d", awarded)
	}
}

func TestConcurrentBudgetDoesNotOverspend(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	requireBudgetLeft(t, svc, 100_000)

	userA := freshUser(t)
	userB := freshUser(t)

	const workers = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	var awarded int64
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			user := userA
			if i%2 == 1 {
				user = userB
			}
			res, err := svc.Pay(ctx, user, 100_000, fmt.Sprintf("budget-race-%d-%s", i, uuid.NewString()))
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

	camp, err := svc.GetCampaign(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if camp.BudgetSpentIDR > camp.BudgetTotalIDR {
		t.Fatalf("overspent: spent=%d total=%d awarded_batch=%d", camp.BudgetSpentIDR, camp.BudgetTotalIDR, awarded)
	}
}
