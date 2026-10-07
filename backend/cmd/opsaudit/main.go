// Command opsaudit produces a Flash Cashback daily audit snapshot for ops.
//
// Typical cron (production):
//
//	DATABASE_URL=... go run ./cmd/opsaudit -out /var/log/cashi/audit
//
// Exit 0 when invariants hold; exit 1 when budget or daily-cap integrity fails.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cashi/cashi/backend/internal/config"
	"github.com/cashi/cashi/backend/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CampaignSnap struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	Timezone       string  `json:"timezone"`
	DailyCapIDR    int64   `json:"dailyCapIdr"`
	BudgetTotalIDR int64   `json:"budgetTotalIdr"`
	BudgetSpentIDR int64   `json:"budgetSpentIdr"`
	BudgetLeftIDR  int64   `json:"budgetLeftIdr"`
	UtilizationPct float64 `json:"utilizationPct"`
}

type DailyUser struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	EarnedIDR   int64  `json:"earnedIdr"`
	AtCap       bool   `json:"atCap"`
	OverCap     bool   `json:"overCap"`
}

type DayTotals struct {
	Day                 string `json:"day"`
	UsersWithEarnings   int    `json:"usersWithEarnings"`
	TotalEarnedIDR      int64  `json:"totalEarnedIdr"`
	UsersAtDailyCap     int    `json:"usersAtDailyCap"`
	UsersOverDailyCap   int    `json:"usersOverDailyCap"`
	PaymentsCount       int64  `json:"paymentsCount"`
	PaymentsCashbackIDR int64  `json:"paymentsCashbackIdr"`
	RedeemsCount        int64  `json:"redeemsCount"`
	RedeemedIDR         int64  `json:"redeemedIdr"`
}

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type Report struct {
	GeneratedAt string       `json:"generatedAt"`
	AppEnv      string       `json:"appEnv"`
	Campaign    CampaignSnap `json:"campaign"`
	Day         DayTotals    `json:"day"`
	TopEarners  []DailyUser  `json:"topEarners"`
	Checks      []Check      `json:"checks"`
	OK          bool         `json:"ok"`
}

func main() {
	dayFlag := flag.String("day", "", "campaign day YYYY-MM-DD (default: today in Asia/Jakarta)")
	outDir := flag.String("out", "", "if set, write <day>.json and <day>.txt under this directory")
	topN := flag.Int("top", 20, "max users to list in top earners")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fatalf("config: %v", err)
	}

	loc, err := time.LoadLocation(domain.CampaignTZ)
	if err != nil {
		fatalf("timezone: %v", err)
	}
	now := time.Now().In(loc)
	day := strings.TrimSpace(*dayFlag)
	if day == "" {
		day = now.Format("2006-01-02")
	}
	if _, err := time.ParseInLocation("2006-01-02", day, loc); err != nil {
		fatalf("invalid -day %q (want YYYY-MM-DD)", day)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatalf("db connect: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatalf("db ping: %v", err)
	}

	report, err := buildReport(ctx, pool, cfg.Env, day, now, *topN)
	if err != nil {
		fatalf("audit: %v", err)
	}

	text := formatText(report)
	fmt.Print(text)

	if *outDir != "" {
		if err := os.MkdirAll(*outDir, 0o755); err != nil {
			fatalf("mkdir out: %v", err)
		}
		base := filepath.Join(*outDir, day)
		if err := os.WriteFile(base+".txt", []byte(text), 0o644); err != nil {
			fatalf("write txt: %v", err)
		}
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fatalf("json: %v", err)
		}
		if err := os.WriteFile(base+".json", append(raw, '\n'), 0o644); err != nil {
			fatalf("write json: %v", err)
		}
		fmt.Fprintf(os.Stderr, "wrote %s.txt and %s.json\n", base, base)
	}

	if !report.OK {
		os.Exit(1)
	}
}

func buildReport(ctx context.Context, pool *pgxpool.Pool, appEnv, day string, now time.Time, topN int) (Report, error) {
	var c CampaignSnap
	err := pool.QueryRow(ctx, `
		SELECT id, name, status, timezone, daily_cap_idr,
		       budget_total_idr, budget_spent_idr
		FROM campaigns WHERE id = $1
	`, domain.CampaignIDFlash).Scan(
		&c.ID, &c.Name, &c.Status, &c.Timezone, &c.DailyCapIDR,
		&c.BudgetTotalIDR, &c.BudgetSpentIDR,
	)
	if err != nil {
		return Report{}, fmt.Errorf("campaign: %w", err)
	}
	c.BudgetLeftIDR = c.BudgetTotalIDR - c.BudgetSpentIDR
	if c.BudgetTotalIDR > 0 {
		c.UtilizationPct = float64(c.BudgetSpentIDR) * 100 / float64(c.BudgetTotalIDR)
	}

	dayTot := DayTotals{Day: day}
	err = pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COALESCE(SUM(earned_idr), 0),
			COUNT(*) FILTER (WHERE earned_idr >= $2)::int,
			COUNT(*) FILTER (WHERE earned_idr > $2)::int
		FROM user_daily_earnings
		WHERE day = $1::date
	`, day, c.DailyCapIDR).Scan(
		&dayTot.UsersWithEarnings,
		&dayTot.TotalEarnedIDR,
		&dayTot.UsersAtDailyCap,
		&dayTot.UsersOverDailyCap,
	)
	if err != nil {
		return Report{}, fmt.Errorf("daily earnings: %w", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(cashback_idr), 0)
		FROM payments
		WHERE (created_at AT TIME ZONE $2)::date = $1::date
	`, day, domain.CampaignTZ).Scan(&dayTot.PaymentsCount, &dayTot.PaymentsCashbackIDR)
	if err != nil {
		return Report{}, fmt.Errorf("payments: %w", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount_idr), 0)
		FROM redemptions
		WHERE (created_at AT TIME ZONE $2)::date = $1::date
	`, day, domain.CampaignTZ).Scan(&dayTot.RedeemsCount, &dayTot.RedeemedIDR)
	if err != nil {
		return Report{}, fmt.Errorf("redemptions: %w", err)
	}

	rows, err := pool.Query(ctx, `
		SELECT ude.user_id, u.display_name, ude.earned_idr
		FROM user_daily_earnings ude
		JOIN users u ON u.id = ude.user_id
		WHERE ude.day = $1::date
		ORDER BY ude.earned_idr DESC, ude.user_id
		LIMIT $2
	`, day, topN)
	if err != nil {
		return Report{}, fmt.Errorf("top earners: %w", err)
	}
	defer rows.Close()

	var top []DailyUser
	for rows.Next() {
		var u DailyUser
		if err := rows.Scan(&u.UserID, &u.DisplayName, &u.EarnedIDR); err != nil {
			return Report{}, err
		}
		u.AtCap = u.EarnedIDR >= c.DailyCapIDR
		u.OverCap = u.EarnedIDR > c.DailyCapIDR
		top = append(top, u)
	}
	if err := rows.Err(); err != nil {
		return Report{}, err
	}
	if top == nil {
		top = []DailyUser{}
	}

	checks := []Check{
		{
			Name:   "budget_not_overspent",
			OK:     c.BudgetSpentIDR <= c.BudgetTotalIDR,
			Detail: fmt.Sprintf("spent=%d total=%d", c.BudgetSpentIDR, c.BudgetTotalIDR),
		},
		{
			Name:   "budget_left_non_negative",
			OK:     c.BudgetLeftIDR >= 0,
			Detail: fmt.Sprintf("left=%d", c.BudgetLeftIDR),
		},
		{
			Name:   "no_user_over_daily_cap",
			OK:     dayTot.UsersOverDailyCap == 0,
			Detail: fmt.Sprintf("over_cap_users=%d daily_cap=%d", dayTot.UsersOverDailyCap, c.DailyCapIDR),
		},
		{
			Name:   "exhausted_implies_no_budget_left",
			OK:     c.Status != string(domain.CampaignStatusExhausted) || c.BudgetLeftIDR == 0,
			Detail: fmt.Sprintf("status=%s left=%d", c.Status, c.BudgetLeftIDR),
		},
	}

	ok := true
	for _, ch := range checks {
		if !ch.OK {
			ok = false
			break
		}
	}

	return Report{
		GeneratedAt: now.Format(time.RFC3339),
		AppEnv:      appEnv,
		Campaign:    c,
		Day:         dayTot,
		TopEarners:  top,
		Checks:      checks,
		OK:          ok,
	}, nil
}

func formatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Flash Cashback daily audit ===\n")
	fmt.Fprintf(&b, "generated_at: %s\n", r.GeneratedAt)
	fmt.Fprintf(&b, "app_env:      %s\n", r.AppEnv)
	fmt.Fprintf(&b, "day:          %s (%s)\n", r.Day.Day, domain.CampaignTZ)
	fmt.Fprintf(&b, "\n")

	c := r.Campaign
	fmt.Fprintf(&b, "-- Campaign %s (%s)\n", c.ID, c.Name)
	fmt.Fprintf(&b, "status:           %s\n", c.Status)
	fmt.Fprintf(&b, "budget_total_idr: %d\n", c.BudgetTotalIDR)
	fmt.Fprintf(&b, "budget_spent_idr: %d\n", c.BudgetSpentIDR)
	fmt.Fprintf(&b, "budget_left_idr:  %d\n", c.BudgetLeftIDR)
	fmt.Fprintf(&b, "utilization_pct:  %.2f\n", c.UtilizationPct)
	fmt.Fprintf(&b, "daily_cap_idr:    %d\n", c.DailyCapIDR)
	fmt.Fprintf(&b, "\n")

	d := r.Day
	fmt.Fprintf(&b, "-- Daily activity\n")
	fmt.Fprintf(&b, "users_with_earnings:   %d\n", d.UsersWithEarnings)
	fmt.Fprintf(&b, "total_earned_idr:      %d\n", d.TotalEarnedIDR)
	fmt.Fprintf(&b, "users_at_daily_cap:    %d\n", d.UsersAtDailyCap)
	fmt.Fprintf(&b, "users_over_daily_cap:  %d\n", d.UsersOverDailyCap)
	fmt.Fprintf(&b, "payments_count:        %d\n", d.PaymentsCount)
	fmt.Fprintf(&b, "payments_cashback_idr: %d\n", d.PaymentsCashbackIDR)
	fmt.Fprintf(&b, "redeems_count:         %d\n", d.RedeemsCount)
	fmt.Fprintf(&b, "redeemed_idr:          %d\n", d.RedeemedIDR)
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "-- Top earners (max %d)\n", len(r.TopEarners))
	if len(r.TopEarners) == 0 {
		fmt.Fprintf(&b, "(none)\n")
	} else {
		for _, u := range r.TopEarners {
			flag := ""
			if u.OverCap {
				flag = " OVER_CAP"
			} else if u.AtCap {
				flag = " AT_CAP"
			}
			fmt.Fprintf(&b, "%-40s %12d%s\n", u.UserID+" ("+u.DisplayName+")", u.EarnedIDR, flag)
		}
	}
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "-- Invariant checks\n")
	for _, ch := range r.Checks {
		status := "PASS"
		if !ch.OK {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "[%s] %s — %s\n", status, ch.Name, ch.Detail)
	}
	fmt.Fprintf(&b, "\n")
	if r.OK {
		fmt.Fprintf(&b, "RESULT: OK\n")
	} else {
		fmt.Fprintf(&b, "RESULT: FAIL — investigate before close of day\n")
	}
	return b.String()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "opsaudit: "+format+"\n", args...)
	os.Exit(2)
}
