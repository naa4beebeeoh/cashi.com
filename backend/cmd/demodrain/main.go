// Command demodrain sets how much campaign budget is left (demo/frontend checks).
//
// Daily caps make burning millions via Pay slow, so the default path sets
// budget_spent in Postgres (demo fixture). Use -via-pay to move via real Pay.
//
//	go run ./cmd/demodrain -leave 5000          # nearly empty, still active
//	go run ./cmd/demodrain -leave 0             # exhausted
//	go run ./cmd/demodrain -leave 10000000      # full budget reset
//	go run ./cmd/demodrain -leave 5000 -via-pay # drain with real pays until left <= 5000
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cashi/cashi/backend/internal/cashback"
	"github.com/cashi/cashi/backend/internal/config"
	"github.com/cashi/cashi/backend/internal/domain"
	"github.com/cashi/cashi/backend/internal/postgres"
	"github.com/cashi/cashi/backend/internal/redisstore"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	leave := flag.Int64("leave", -1, "IDR campaign budget to leave (required; 0 = exhausted)")
	viaPay := flag.Bool("via-pay", false, "reach -leave via real Pay calls instead of SQL fixture")
	flag.Parse()
	if flag.NArg() != 0 {
		fatalf("usage: demodrain -leave N [-via-pay]")
	}
	if *leave < 0 {
		fatalf("-leave is required (IDR to leave; 0 = exhausted, e.g. 5000 = nearly empty)")
	}

	cfg, err := config.Load()
	if err != nil {
		fatalf("config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatalf("db: %v", err)
	}
	defer pool.Close()

	before, err := readCampaign(ctx, pool)
	if err != nil {
		fatalf("%v", err)
	}
	if *leave > before.Total {
		fatalf("-leave %d exceeds budget_total %d", *leave, before.Total)
	}
	printSnap("before", before)

	if *viaPay {
		if err := drainViaPay(ctx, cfg, *leave); err != nil {
			fatalf("via-pay: %v", err)
		}
	} else {
		status := string(domain.CampaignStatusActive)
		if *leave == 0 {
			status = string(domain.CampaignStatusExhausted)
		}
		if err := setBudgetLeft(ctx, pool, *leave, status); err != nil {
			fatalf("%v", err)
		}
	}

	after, err := readCampaign(ctx, pool)
	if err != nil {
		fatalf("%v", err)
	}
	printSnap("after", after)
	fmt.Println()
	fmt.Println("Refresh the Expo campaign strip (pull-to-refresh / re-open).")
	if after.Left > 0 && after.Left <= domain.DailyCapIDR {
		fmt.Printf("Tip: Pay Rp100.000 in the UI to consume up to the last %d IDR.\n", after.Left)
	}
}

type snap struct {
	Status string
	Total  int64
	Spent  int64
	Left   int64
}

func readCampaign(ctx context.Context, pool *pgxpool.Pool) (snap, error) {
	var s snap
	err := pool.QueryRow(ctx, `
		SELECT status, budget_total_idr, budget_spent_idr,
		       budget_total_idr - budget_spent_idr
		FROM campaigns WHERE id = $1
	`, domain.CampaignIDFlash).Scan(&s.Status, &s.Total, &s.Spent, &s.Left)
	if err != nil {
		return snap{}, fmt.Errorf("read campaign: %w", err)
	}
	return s, nil
}

func printSnap(label string, s snap) {
	fmt.Printf("[%s] status=%s spent=%d total=%d left=%d\n", label, s.Status, s.Spent, s.Total, s.Left)
}

func setBudgetLeft(ctx context.Context, pool *pgxpool.Pool, left int64, status string) error {
	tag, err := pool.Exec(ctx, `
		UPDATE campaigns
		SET budget_spent_idr = GREATEST(0, budget_total_idr - $2),
		    status = $3
		WHERE id = $1
	`, domain.CampaignIDFlash, left, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("campaign %s not found", domain.CampaignIDFlash)
	}
	fmt.Printf("demo fixture: set budget left=%d status=%s (SQL; not a real earn path)\n", left, status)
	return nil
}

func drainViaPay(ctx context.Context, cfg config.Config, targetLeft int64) error {
	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	rdb, err := redisstore.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	svc, err := cashback.New(db, rdb)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	camp, err := svc.GetCampaign(ctx)
	if err != nil {
		return err
	}
	if camp.BudgetLeftIDR < targetLeft {
		return fmt.Errorf("current left %d is already below target %d; refuse to increase budget via Pay", camp.BudgetLeftIDR, targetLeft)
	}

	var awarded int64
	pays := 0
	for {
		camp, err = svc.GetCampaign(ctx)
		if err != nil {
			return err
		}
		if camp.BudgetLeftIDR <= targetLeft {
			break
		}

		user, err := insertUser(ctx, pool)
		if err != nil {
			return err
		}
		for i := 0; i < 20; i++ {
			camp, err = svc.GetCampaign(ctx)
			if err != nil {
				return err
			}
			if camp.BudgetLeftIDR <= targetLeft {
				break
			}
			res, err := svc.Pay(ctx, user, 100_000, fmt.Sprintf("drain-%s-%d", uuid.NewString(), i))
			if err != nil {
				return err
			}
			awarded += res.CashbackIDR
			pays++
			if res.CashbackIDR == 0 {
				break
			}
		}
		if pays%50 == 0 && pays > 0 {
			fmt.Printf("... pays=%d awarded=%d left=%d\n", pays, awarded, camp.BudgetLeftIDR)
		}
	}

	if targetLeft == 0 {
		if err := setBudgetLeft(ctx, pool, 0, string(domain.CampaignStatusExhausted)); err != nil {
			return err
		}
	}
	fmt.Printf("via-pay: pays=%d awarded_cashback=%d\n", pays, awarded)
	return nil
}

func insertUser(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	id := "drain-" + uuid.NewString()
	_, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $2)`, id, "Drain "+id)
	if err != nil {
		return "", err
	}
	_, err = pool.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1)`, id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "demodrain: "+format+"\n", args...)
	os.Exit(2)
}
