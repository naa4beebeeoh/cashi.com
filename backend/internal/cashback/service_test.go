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

	// Asia/Jakarta is UTC+7 with no DST. Midnight WIB = 17:00:00 UTC previous calendar day.
	tests := []struct {
		name string
		utc  time.Time
		want string
	}{
		{
			name: "last second before Jakarta midnight stays previous day",
			utc:  time.Date(2026, 3, 20, 16, 59, 59, 0, time.UTC), // == 2026-03-20 23:59:59 WIB
			want: "2026-03-20",
		},
		{
			name: "exact Jakarta midnight rolls to next day",
			utc:  time.Date(2026, 3, 20, 17, 0, 0, 0, time.UTC), // == 2026-03-21 00:00:00 WIB
			want: "2026-03-21",
		},
		{
			name: "one second after Jakarta midnight is next day",
			utc:  time.Date(2026, 3, 20, 17, 0, 1, 0, time.UTC), // == 2026-03-21 00:00:01 WIB
			want: "2026-03-21",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.CampaignDay(tt.utc); got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
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
