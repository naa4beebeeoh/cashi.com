package cashback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cashi/cashi/backend/internal/domain"
)

func TestCampaignDayUsesJakartaCalendar(t *testing.T) {
	loc, err := time.LoadLocation(domain.CampaignTZ)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{loc: loc}

	// 2026-03-20 17:30 UTC == 2026-03-21 00:30 Asia/Jakarta
	utcEvening := time.Date(2026, 3, 20, 17, 30, 0, 0, time.UTC)
	if got := svc.CampaignDay(utcEvening); got != "2026-03-21" {
		t.Fatalf("got %s want 2026-03-21", got)
	}

	// Still previous Jakarta day just before midnight WIB
	utcBefore := time.Date(2026, 3, 20, 16, 59, 0, 0, time.UTC)
	if got := svc.CampaignDay(utcBefore); got != "2026-03-20" {
		t.Fatalf("got %s want 2026-03-20", got)
	}
}

func TestPayInputValidation(t *testing.T) {
	svc := &Service{loc: time.UTC}
	ctx := context.Background()

	tests := []struct {
		name string
		user string
		amt  int64
		key  string
		want error
	}{
		{"missing user", "", 100_000, "k", domain.ErrMissingUser},
		{"missing key", "user_a", 100_000, "", domain.ErrMissingIdempotency},
		{"zero amount", "user_a", 0, "k", domain.ErrInvalidAmount},
		{"negative amount", "user_a", -5, "k", domain.ErrInvalidAmount},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Pay(ctx, tt.user, tt.amt, tt.key)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err=%v want %v", err, tt.want)
			}
		})
	}
}

func TestRedeemInputValidation(t *testing.T) {
	svc := &Service{loc: time.UTC}
	ctx := context.Background()

	tests := []struct {
		name string
		user string
		amt  int64
		key  string
		want error
	}{
		{"missing user", "", 1_000, "k", domain.ErrMissingUser},
		{"missing key", "user_a", 1_000, "", domain.ErrMissingIdempotency},
		{"zero amount", "user_a", 0, "k", domain.ErrInvalidAmount},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Redeem(ctx, tt.user, tt.amt, tt.key)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err=%v want %v", err, tt.want)
			}
		})
	}
}
